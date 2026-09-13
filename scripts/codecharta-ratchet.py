#!/usr/bin/env python3
"""CodeCharta ratchet gate.

CodeCharta is a visualizer; it has no built-in "fail when worse". This turns its
merged map into three ratchets over metrics that aren't already gated elsewhere
(coverage has its own floors in the Test job; mutation is a separate gate):

  1. Complexity cap — no file may exceed its cap on the per-file SUM of function
     complexity. Files listed in the baseline are capped at their recorded value (so
     they can't grow); everything else is capped at default_cap. This stops a file
     becoming a monolith, but is satisfiable by splitting a file (the sum moves with
     the code). Ratchet: lower the numbers as code is simplified.
  1b. Function-complexity cap — same shape, on max_complexity_per_function (the single
     most complex function in a file). This is the overall-complexity control that
     canNOT be gamed by moving code between files: a function keeps its complexity
     wherever it lives, so passing requires actually simplifying the function.
  2. Hotspot gate — a file that is both complex (complexity >= min_complexity) and
     under-tested (line_coverage < min_coverage) fails, unless it is grandfathered
     in `allow`. This blocks NEW complex-and-untested files; shrink `allow` as the
     existing ones get tests.

Inside a baseline, keys starting with `_` are prose, not paths — write the
justification for a number next to that number.

Usage: codecharta-ratchet.py <map.cc.json[.gz]> [config.json]
Exit codes: 0 = within ratchet, 1 = a metric regressed (per-file report),
2 = usage / unreadable input / malformed map.
"""
import gzip
import json
import sys


def _reject_nonfinite(value):
    """json.load accepts the non-standard literals NaN, Infinity and -Infinity.
    A threshold or cap set to NaN makes every comparison false, so the gate
    would pass without judging anything — the exact failure this script exists
    to prevent, reachable by editing a tracked config file."""
    raise ValueError(f"non-finite number {value!r} is not allowed in ratchet input")


def load_json(path):
    opener = gzip.open if path.endswith(".gz") else open
    with opener(path, "rt", encoding="utf-8") as fh:
        return json.load(fh, parse_constant=_reject_nonfinite)


# The paths ccsh's unifiedparser was asked to measure: Go sources outside the
# excluded trees. Everything else in the map — tests, Markdown, YAML, the R
# spike — reaches it only through gitlogparser and legitimately carries no
# complexity metric. Keep this in step with the -fe / -e flags in
# .github/workflows/codecharta.yml and the Makefile's codecharta target.
PARSED_EXCLUDES = ("vendor/", "spike/", "build/")


def is_parsed_source(rel):
    """True for a file the complexity parser was supposed to measure."""
    if not rel.endswith(".go") or rel.endswith("_test.go"):
        return False
    return not rel.startswith(PARSED_EXCLUDES)


def cap_check(files, metric, default_cap, baseline, label, cfg_path):
    """Per-file ceiling on `metric`: files in `baseline` are frozen at their recorded
    value (can't grow), everything else must stay <= default_cap. Returns
    (violations, hints); a baseline file now BELOW its cap yields a ratchet-down hint.
    Files missing the metric are skipped."""
    # `_`-prefixed keys are prose, not paths: a justification belongs next to the
    # number it explains, and without this every such note is reported as a stale
    # baseline entry on every run — noise that trains readers to ignore the hints.
    baseline = {k: v for k, v in baseline.items() if not k.startswith("_")}
    violations, hints = [], []
    for rel, attrs in sorted(files.items()):
        val = attrs.get(metric)
        if val is None:
            # A missing value means this file is subject to no cap at all. For
            # anything the parser was meant to measure that is a hole, not a
            # pass: a new source file, or one ccsh failed to parse, would be
            # free to grow while the gate stayed green. The global "absent
            # everywhere" guard cannot see a single file going missing.
            if rel in baseline:
                violations.append(
                    f"{label}: {rel} is baselined at {baseline[rel]} but the map "
                    f"carries no '{metric}' for it — its cap would vanish")
            elif is_parsed_source(rel):
                violations.append(
                    f"{label}: {rel} is a parsed source but the map carries no "
                    f"'{metric}' for it — it would be subject to no cap")
            continue
        cap = baseline.get(rel, default_cap)
        if val > cap:
            where = "baseline" if rel in baseline else f"default_cap {default_cap}"
            violations.append(f"{label}: {rel} = {val:.0f} > {cap} ({where})")
        elif rel in baseline and val < cap:
            hints.append(f"ratchet down: {rel} {label} {cap} -> {val:.0f} in {cfg_path}")
    # Flag stale baseline entries (file renamed/deleted) so the config stays clean —
    # a dead entry otherwise silently stops applying, mirroring the hotspot.allow hint.
    for rel in sorted(baseline):
        if rel not in files:
            hints.append(f"{label} baseline: {rel} is not in the map (renamed/deleted?) — remove it from {cfg_path}")
    return violations, hints


def leaves(node, parts):
    """Yield (relpath, attributes) for every File node, path relative to repo root
    (the map's root node name — 'root' — is stripped, so the rest matches the
    committed baseline keys like 'internal/adapters/...')."""
    p = parts + [node["name"]]
    if node.get("type") == "File":
        yield "/".join(p[1:]), (node.get("attributes") or {})
    for child in node.get("children", []):
        yield from leaves(child, p)


def main():
    if len(sys.argv) < 2:
        print("usage: codecharta-ratchet.py <map.cc.json[.gz]> [config.json]", file=sys.stderr)
        return 2
    map_path = sys.argv[1]
    cfg_path = sys.argv[2] if len(sys.argv) > 2 else ".codecharta-ratchet.json"
    try:
        doc = load_json(map_path)
        cfg = load_json(cfg_path)
    except (OSError, ValueError) as e:  # missing/unreadable file or invalid JSON
        print(f"::error::cannot read input ({e})", file=sys.stderr)
        return 2
    # Both must be objects before anything calls .get on them: syntactically
    # valid JSON like [] or null would otherwise raise AttributeError deep in
    # the run, and a crash reads as a broken script rather than as the broken
    # input it is. The promise of this function is exit 2 for a malformed map.
    for name, obj in (("map", doc), ("config", cfg)):
        if not isinstance(obj, dict):
            print(f"::error::{name} is {type(obj).__name__}, expected a JSON object",
                  file=sys.stderr)
            return 2
    nodes = doc.get("nodes")
    if not nodes:
        data = doc.get("data")
        nodes = data.get("nodes") if isinstance(data, dict) else None
    if not nodes:
        print(f"::error::{map_path}: no nodes in map — cannot run the ratchet", file=sys.stderr)
        return 2
    if not isinstance(nodes, list) or not isinstance(nodes[0], dict):
        print(f"::error::{map_path}: 'nodes' is not a list of objects — cannot run "
              f"the ratchet", file=sys.stderr)
        return 2
    try:
        files = dict(leaves(nodes[0], []))
    except (AttributeError, KeyError, TypeError) as e:
        # A malformed node tree is broken INPUT, and the contract of this
        # function is exit 2 for that. A traceback reads as a broken script and
        # sends the reader to the wrong place.
        print(f"::error::{map_path}: malformed node tree ({type(e).__name__}: {e})",
              file=sys.stderr)
        return 2
    if not files:
        print(f"::error::{map_path}: map has nodes but no File leaves — cannot run the "
              f"ratchet (ccsh format change?). Refusing to pass vacuously.", file=sys.stderr)
        return 2

    try:
        cx = cfg["complexity"]
        metric, default_cap, baseline = cx["metric"], cx["default_cap"], cx["baseline"]
        fc = cfg["function_complexity"]
        fmetric, fdefault, fbaseline = fc["metric"], fc["default_cap"], fc["baseline"]
        hs = cfg["hotspot"]
        min_cx, min_cov, allow = hs["min_complexity"], hs["min_coverage"], set(hs["allow"])
    except (KeyError, TypeError) as e:  # missing key or wrong shape (typo in config)
        print(f"::error::{cfg_path}: malformed config ({e})", file=sys.stderr)
        return 2

    # Every threshold and every cap must be a finite number. A string, a None
    # or a NaN that slipped past the parser would make the comparisons below
    # silently false, and the gate would report OK having judged nothing.
    numbers = [("complexity.default_cap", default_cap),
               ("function_complexity.default_cap", fdefault),
               ("hotspot.min_complexity", min_cx),
               ("hotspot.min_coverage", min_cov)]
    numbers += [(f"complexity.baseline[{k}]", v) for k, v in baseline.items()
                if not k.startswith("_")]
    numbers += [(f"function_complexity.baseline[{k}]", v) for k, v in fbaseline.items()
                if not k.startswith("_")]
    for name, v in numbers:
        if not isinstance(v, (int, float)) or isinstance(v, bool) or v != v \
                or v in (float("inf"), float("-inf")):
            print(f"::error::{cfg_path}: {name} = {v!r} is not a finite number",
                  file=sys.stderr)
            return 2

    # Each baseline must be an object {path: cap}; otherwise cap_check's baseline.get()
    # would raise deep in the run instead of failing here with a clear message.
    for name, b in (("complexity.baseline", baseline), ("function_complexity.baseline", fbaseline)):
        if not isinstance(b, dict):
            print(f"::error::{cfg_path}: {name} must be an object (got {type(b).__name__})", file=sys.stderr)
            return 2

    # Guard against a vacuous pass: if a cap metric is absent from EVERY file (a ccsh
    # version / parser change, or a typo'd metric name), cap_check would skip all files
    # and the gate would silently pass. Fail loudly instead — the map still has files.
    for m in (metric, fmetric):
        if not any(a.get(m) is not None for a in files.values()):
            print(f"::error::metric '{m}' is absent from every file in the map — "
                  f"ccsh version/parser mismatch? Refusing to pass vacuously.", file=sys.stderr)
            return 2

    violations, hints = [], []

    # 1. Per-file aggregate complexity (sum of function complexity). Stops a file
    # becoming a monolith — but is satisfiable by splitting a file, since the sum
    # just moves with the code.
    v, h = cap_check(files, metric, default_cap, baseline, "complexity", cfg_path)
    violations += v
    hints += h

    # 1b. Per-FUNCTION complexity — the overall-complexity control that canNOT be
    # gamed by moving code between files: a function keeps its complexity wherever it
    # lives, so relocating it never lowers this number. Passing requires actually
    # simplifying the function (or extracting cohesive sub-functions).
    v, h = cap_check(files, fmetric, fdefault, fbaseline, "function-complexity", cfg_path)
    violations += v
    hints += h

    # 2. Hotspots (complex AND under-tested). Files without coverage data are skipped
    # (can't assess — e.g. cmd tools not exercised by unit tests). But if NO file
    # has coverage — the coverage import failed, or ccsh changed the attribute
    # name — the whole gate would quietly do nothing while still reporting OK.
    # Same reasoning as the cap-metric guard above: refuse to pass vacuously.
    if not any(a.get("line_coverage") is not None for a in files.values()):
        print("::error::no file in the map carries 'line_coverage' — the coverage "
              "import did not land, so the hotspot gate would pass vacuously. "
              "Check the coverage step (a failing test suite is the usual cause).",
              file=sys.stderr)
        return 2

    for rel, attrs in sorted(files.items()):
        val = attrs.get(metric)
        cov = attrs.get("line_coverage")
        if val is None:
            continue
        if cov is None:
            # A file the parser measured but coverage never mentioned has no
            # test data at all — which for the hotspot gate is the worst case,
            # not an exemption. Skipping it let a new, complex, wholly untested
            # source pass while the gate reported green. Files that legitimately
            # have no coverage (the composition root) belong in `allow`, named,
            # where the config says why.
            if val >= min_cx and rel not in allow and is_parsed_source(rel):
                violations.append(
                    f"hotspot: {rel} complexity {val:.0f} >= {min_cx} AND the map "
                    f"carries no coverage for it at all")
            continue
        if val >= min_cx and cov < min_cov and rel not in allow:
            violations.append(f"hotspot: {rel} complexity {val:.0f} >= {min_cx} AND coverage {cov:.1f}% < {min_cov}%")
    for rel in sorted(allow):
        attrs = files.get(rel)
        if attrs is None:
            hints.append(f"allowlist: {rel} is not in the map (renamed/deleted?) — remove it from hotspot.allow")
            continue
        cov, val = attrs.get("line_coverage"), attrs.get(metric)
        if val is not None and cov is not None and not (val >= min_cx and cov < min_cov):
            hints.append(f"allowlist: {rel} is no longer a hotspot — remove it from hotspot.allow")

    for h in hints:
        print(f"::notice::CodeCharta ratchet — {h}")
    if violations:
        print(f"\n❌ CodeCharta ratchet: {len(violations)} regression(s):")
        for v in violations:
            print(f"  - {v}")
        print(f"\nAdd tests / simplify the file, or (with justification) adjust {cfg_path}.")
        return 1
    print(f"✅ CodeCharta ratchet OK — {len(files)} files within complexity caps + hotspot gate.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
