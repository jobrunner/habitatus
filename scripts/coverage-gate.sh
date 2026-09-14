#!/usr/bin/env bash
#
# coverage-gate.sh — per-package coverage ratchet.
#
# Computes per-package statement coverage from a Go coverprofile and fails if
# any package listed in .coverage-floors is below its floor. Packages not
# listed are exempt (composition root, cmd, thin SDK wrappers — see
# the coverage policy in README.md). The floors are a RATCHET: they may only ever be raised.
#
# Usage: scripts/coverage-gate.sh [coverprofile]   (default: coverage.out)
set -euo pipefail

# Force a C numeric locale: awk's printf "%.1f" and numeric comparisons must use a
# dot decimal separator. Under a comma-decimal locale (e.g. de_DE.UTF-8) "%.1f" yields
# "100,0", which awk then compares as a STRING ("100,0" < "99" is true) → spurious
# BELOW-FLOOR failures. CI runs under C; make local runs match it.
export LC_ALL=C

PROFILE="${1:-coverage.out}"
FLOORS="$(dirname "$0")/../.coverage-floors"
MODULE="github.com/jobrunner/habitatus"

[ -f "$PROFILE" ] || { echo "coverage-gate: profile not found: $PROFILE" >&2; exit 2; }
[ -f "$FLOORS" ]  || { echo "coverage-gate: floors file not found: $FLOORS" >&2; exit 2; }

BYPKG="$(mktemp)"
trap 'rm -f "$BYPKG"' EXIT

# Per-package + global statement coverage from the profile.
#   line: <module>/<pkg>/<file>.go:s.c,e.c <numStmts> <count>
awk -v module="$MODULE/" '
  NR == 1 && $1 == "mode:" { next }
  {
    path = $1; sub(/:.*/, "", path)        # strip :start.col,end.col
    sub(module, "", path)                  # strip module prefix
    pkg = path; sub(/\/[^\/]*$/, "", pkg)  # dir = package
    stmts = $2; cnt = $3
    tot[pkg] += stmts; gtot += stmts
    if (cnt > 0) { cov[pkg] += stmts; gcov += stmts }
  }
  END {
    for (p in tot) printf "%s %d %d\n", p, cov[p], tot[p]
    printf "TOTAL %d %d\n", gcov, gtot
  }
' "$PROFILE" > "$BYPKG"

# Every package in the profile must carry a floor or be exempt by name. Without
# this the gate only ever judged what the file happened to list: a new package
# with no tests and no floor line passed, as long as TOTAL stayed up — the gate
# would faithfully check the packages it knew about and never mention the one
# that was added. Exemptions are lines starting with "!", so leaving a package
# out is a deliberate, reviewable act rather than an omission.
EXEMPT="$(sed -n 's/^!//p' "$FLOORS" | awk '{print $1}')"
missing=0
while read -r pkg _; do
  [ "$pkg" = "TOTAL" ] && continue
  grep -qE "^!?${pkg}([[:space:]]|$)" "$FLOORS" && continue
  echo "coverage-gate: $pkg has no floor and is not exempt — add a floor line, or" >&2
  echo "               '!$pkg' with the reason, so the omission is visible." >&2
  missing=1
done <<EOF
$(awk '{print $1}' "$BYPKG")
EOF
[ "$missing" -eq 0 ] || exit 1
[ -z "$EXEMPT" ] || printf "exempt (no floor, by name): %s\n\n" "$(echo "$EXEMPT" | tr '\n' ' ')"

fail=0
printf "%-42s %8s %7s\n" "package" "cov" "floor"
printf -- "------------------------------------------------------------\n"
while read -r pkg floor; do
  [ -z "$pkg" ] && continue
  case "$pkg" in \#*) continue ;; !*) continue ;; esac
  # Validate the floor before using it. awk compares an unparseable value as 0
  # and a negative one as itself, so `internal/esy -1` or a stray character
  # would let ANY coverage pass — a typo would switch the ratchet off with no
  # sign that it had. A bad floor is a configuration error, not a pass.
  if ! awk -v f="$floor" 'BEGIN{exit !(f ~ /^[0-9]+(\.[0-9]+)?$/ && f+0 <= 100)}'; then
    echo "coverage-gate: invalid floor for $pkg: '$floor' (expected a number between 0 and 100)" >&2
    exit 2
  fi
  read -r c t <<<"$(awk -v p="$pkg" '$1==p {print $2, $3}' "$BYPKG")"
  if [ -z "${t:-}" ] || [ "${t:-0}" -eq 0 ]; then
    printf "%-42s %8s %7s  NO DATA\n" "$pkg" "-" "$floor"; fail=1; continue
  fi
  # pct is for DISPLAY only. The comparison uses the unrounded ratio: at one
  # decimal place, 85.96 prints as 86.0 and would pass a floor of 86.
  pct=$(awk -v c="$c" -v t="$t" 'BEGIN{printf "%.1f", 100*c/t}')
  if awk -v c="$c" -v t="$t" -v f="$floor" 'BEGIN{exit !(100*c/t < f)}'; then
    printf "%-42s %7s%% %6s%%  ▼ BELOW FLOOR\n" "$pkg" "$pct" "$floor"; fail=1
  else
    printf "%-42s %7s%% %6s%%\n" "$pkg" "$pct" "$floor"
  fi
done < "$FLOORS"

if [ "$fail" -ne 0 ]; then
  echo
  echo "coverage-gate: FAIL — a package dropped below its floor." >&2
  echo "The fix is to ADD TESTS. Floors are a raise-only ratchet; lowering one" >&2
  echo "is a deliberate, justified exception that must be called out in the PR." >&2
  exit 1
fi
echo
echo "coverage-gate: OK — all floored packages at or above their floor."
