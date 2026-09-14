#!/usr/bin/env python3
"""Guard the ratchet configuration against being weakened in the PR that needs it.

A ratchet is only a ratchet if its own settings cannot move the wrong way. Both
`.coverage-floors` and `.codecharta-ratchet.json` are read exclusively from the
working tree, so until this existed a change could lower a floor, delete a
package line, raise a complexity cap or grandfather a file — and every gate
would still report green, because each gate faithfully enforces whatever the
file now says.

This compares both files against the merge base and fails on a weakening.
Loosening is still allowed, but only deliberately: it has to be a reviewed
change to a tracked file with the justification next to the number, and this
script names exactly what moved so the reviewer cannot miss it.

Usage: scripts/ratchet-baseline.py <base-ref>      (e.g. origin/main)
Exit codes: 0 within the ratchet, 1 weakened, 2 usage/unreadable input.
"""
import json
import subprocess
import sys


def _reject_nonfinite(value):
    """json accepts the non-standard literals NaN, Infinity and -Infinity. A cap
    or threshold set to NaN makes every comparison in both ratchets false —
    x > NaN, x < NaN and NaN > old are all false — so a config could pass every
    gate without being judged. Reject them at the parser."""
    raise ValueError(f"non-finite number {value!r} is not allowed in ratchet input")


def loads(text):
    return json.loads(text, parse_constant=_reject_nonfinite)


def at(ref, path):
    """File content at a git ref, or None when it did not exist there."""
    p = subprocess.run(["git", "show", f"{ref}:{path}"],
                       capture_output=True, text=True, check=False)
    return p.stdout if p.returncode == 0 else None


def _number(s):
    """float() accepts 'nan' and 'inf'. A threshold or floor set to NaN makes
    every comparison here false — h[k] < v included — so a lowering would be
    reported as unchanged. Non-finite values are not numbers for this purpose."""
    v = float(s)
    if v != v or v in (float("inf"), float("-inf")):
        raise ValueError(f"non-finite value {s!r}")
    return v


def floors(text):
    out = {}
    for line in (text or "").splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        parts = line.split()
        if len(parts) >= 2:
            try:
                out[parts[0]] = _number(parts[1])
            except ValueError:
                continue
    return out


def check_floors(base, head, bad):
    b, h = floors(base), floors(head)
    for pkg, floor in sorted(b.items()):
        if pkg not in h:
            bad.append(f".coverage-floors: {pkg} (floor {floor:g}) is gone or no longer "
                       f"a finite number — a package with no usable floor has no gate")
        elif h[pkg] < floor:
            bad.append(f".coverage-floors: {pkg} lowered {floor:g} -> {h[pkg]:g}")


def caps(cfg, section):
    s = (cfg or {}).get(section) or {}
    base = {k: v for k, v in (s.get("baseline") or {}).items() if not k.startswith("_")}
    return s.get("default_cap"), base


def thresholds(text):
    out = {}
    for line in (text or "").splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        parts = line.split()
        if len(parts) >= 2:
            try:
                out[parts[0]] = _number(parts[1])
            except ValueError:
                continue
    return out


def check_thresholds(base, head, bad):
    """Mutation thresholds may only rise. They live in a file rather than in the
    workflow precisely so that this can see them."""
    b, h = thresholds(base), thresholds(head)
    for k, v in sorted(b.items()):
        if k not in h:
            bad.append(f".mutation-thresholds: {k} ({v:g}) is gone or no longer a "
                       f"finite number")
        elif h[k] < v:
            bad.append(f".mutation-thresholds: {k} lowered {v:g} -> {h[k]:g}")


def check_caps(base, head, section, bad):
    bdef, bbase = caps(base, section)
    hdef, hbase = caps(head, section)
    if bdef is not None and hdef is not None and hdef > bdef:
        bad.append(f".codecharta-ratchet.json: {section}.default_cap raised {bdef} -> {hdef}")
    for f, cap in sorted(bbase.items()):
        if f in hbase and hbase[f] > cap:
            bad.append(f".codecharta-ratchet.json: {section}.baseline[{f}] raised {cap} -> {hbase[f]}")
        elif f not in hbase and hdef is not None and hdef > cap:
            # Dropping a baseline entry is a loosening only when the default is
            # higher than the value that entry pinned.
            bad.append(f".codecharta-ratchet.json: {section}.baseline[{f}] ({cap}) dropped, "
                       f"so it falls back to default_cap {hdef}")
    # A NEW baseline entry above the base default lifts that file out of the
    # default cap — the same loosening as raising the default, one file at a
    # time, and invisible if only the base's own entries are compared.
    for f, cap in sorted(hbase.items()):
        if f not in bbase and bdef is not None and cap > bdef:
            bad.append(f".codecharta-ratchet.json: {section}.baseline[{f}] added at {cap}, "
                       f"above the base default_cap {bdef}")


def check_metric(base, head, section, bad):
    """The caps are meaningless if the metric under them changes. Swapping
    complexity.metric from `complexity` (the per-file sum) to
    `max_complexity_per_function` (a much smaller number) leaves every cap in
    place and disables the gate."""
    b = ((base or {}).get(section) or {}).get("metric")
    h = ((head or {}).get(section) or {}).get("metric")
    if b is not None and h != b:
        bad.append(f".codecharta-ratchet.json: {section}.metric changed {b!r} -> {h!r} — "
                   f"the existing caps then judge a different quantity")


def check_hotspot(base, head, bad):
    b = (base or {}).get("hotspot") or {}
    h = (head or {}).get("hotspot") or {}
    if b.get("min_coverage") is not None and h.get("min_coverage") is not None \
            and h["min_coverage"] < b["min_coverage"]:
        bad.append(f".codecharta-ratchet.json: hotspot.min_coverage lowered "
                   f"{b['min_coverage']} -> {h['min_coverage']}")
    if b.get("min_complexity") is not None and h.get("min_complexity") is not None \
            and h["min_complexity"] > b["min_complexity"]:
        bad.append(f".codecharta-ratchet.json: hotspot.min_complexity raised "
                   f"{b['min_complexity']} -> {h['min_complexity']} (fewer files judged)")
    added = set(h.get("allow") or []) - set(b.get("allow") or [])
    for f in sorted(added):
        bad.append(f".codecharta-ratchet.json: hotspot.allow gained {f} — "
                   f"a file was grandfathered out of the gate")


def main():
    if len(sys.argv) != 2:
        print(__doc__.strip().splitlines()[-3], file=sys.stderr)
        return 2
    ref = sys.argv[1]
    bad = []

    try:
        with open(".coverage-floors", encoding="utf-8") as fh:
            head_floors = fh.read()
    except OSError as e:
        print(f"::error::cannot read .coverage-floors ({e})", file=sys.stderr)
        return 2
    base_floors = at(ref, ".coverage-floors")
    if base_floors is None:
        print(f"::notice::.coverage-floors does not exist at {ref} — nothing to "
              f"compare, this is its first introduction")
    check_floors(base_floors, head_floors, bad)

    base_thr = at(ref, ".mutation-thresholds")
    if base_thr is None:
        print(f"::notice::.mutation-thresholds does not exist at {ref} — nothing "
              f"to compare, this is its first introduction")
    try:
        with open(".mutation-thresholds", encoding="utf-8") as fh:
            check_thresholds(base_thr, fh.read(), bad)
    except OSError as e:
        print(f"::error::cannot read .mutation-thresholds ({e})", file=sys.stderr)
        return 2

    base_cfg_text = at(ref, ".codecharta-ratchet.json")
    try:
        base_cfg = loads(base_cfg_text) if base_cfg_text else None
        with open(".codecharta-ratchet.json", encoding="utf-8") as fh:
            head_cfg = loads(fh.read())
    except (OSError, ValueError) as e:
        print(f"::error::cannot read the ratchet config ({e})", file=sys.stderr)
        return 2

    if base_cfg is None:
        print(f"::notice::.codecharta-ratchet.json does not exist at {ref} — nothing "
              f"to compare, this is its first introduction")
    else:
        for section in ("complexity", "function_complexity"):
            check_metric(base_cfg, head_cfg, section, bad)
            check_caps(base_cfg, head_cfg, section, bad)
        check_hotspot(base_cfg, head_cfg, bad)

    if bad:
        print(f"\n❌ the ratchet configuration was weakened against {ref}:")
        for b in bad:
            print(f"  - {b}")
        print("\nRatchets may only tighten. If a change genuinely needs one of these "
              "loosened, say so in the PR description and in a comment next to the "
              "number, and a reviewer can override this check deliberately.")
        return 1
    print(f"✅ ratchet configuration is no weaker than {ref}.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
