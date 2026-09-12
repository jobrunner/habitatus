#!/usr/bin/env bash
# Builds the "repaired" oracle: a COPY of the upstream ESy tree in which the
# two lines of step 8 are swapped, so that membership expressions which stay
# numeric are coerced to truth values (0 = FALSE, anything else = TRUE) as R
# itself did up to ESy v1.1, instead of being replaced by FALSE wholesale as
# v1.2 does.
#
#   Usage: spike/resy/patch-upstream.sh <upstream-dir> <dest-dir>
#
# The clone under spike/ESy-upstream is READ-ONLY and is never touched: this
# script copies it (without .git, which is ~486 MB) and patches the copy. The
# patch is the upstream authors' own commented-out line, not an edit of ours.
#
# Every step is checked. A silently unpatched copy would produce a "repaired"
# fixture set that is really the faithful one, and the golden master would
# then pass for the wrong reason -- which is the one failure mode this whole
# exercise cannot afford.
set -euo pipefail

if [ $# -ne 2 ]; then
	echo "usage: $0 <upstream-dir> <dest-dir>" >&2
	exit 2
fi

src=$1
dest=$2
step=code/step3and5_extract-and-solve-membership-conditions.R

coerce='# logi1[which(unlist(lapply(logi1, is.numeric)))] <- lapply(logi1[which(unlist(lapply(logi1, is.numeric)))], function(x) ifelse(x == 0, FALSE, TRUE))'
blanket='logi1[which(unlist(lapply(logi1, is.numeric)))] <- FALSE'

if [ ! -f "$src/$step" ]; then
	echo "$0: $src/$step not found -- is $src the upstream tree?" >&2
	exit 1
fi

# Both lines must be present exactly once in the unmodified source. If
# upstream ever rewrites this block, the swap below would silently do
# something else, so stop here instead.
n_coerce=$(grep -cxF "$coerce" "$src/$step" || true)
n_blanket=$(grep -cxF "$blanket" "$src/$step" || true)
if [ "$n_coerce" != "1" ] || [ "$n_blanket" != "1" ]; then
	echo "$0: expected exactly one commented coercion line and one blanket FALSE line in $step," >&2
	echo "  found $n_coerce and $n_blanket. Upstream's step 8 has changed; re-derive the patch by hand." >&2
	exit 1
fi

rm -rf "$dest"
mkdir -p "$dest"
# The .git directory is excluded deliberately: it is ~486 MB and nothing in
# the pipeline reads it. The commit the copy corresponds to is passed to the
# generator as HABITATUS_UPSTREAM_COMMIT instead.
(cd "$src" && tar --exclude=.git -cf - .) | (cd "$dest" && tar -xf -)

python3 - "$dest/$step" "$coerce" "$blanket" <<'PY'
import sys

path, coerce, blanket = sys.argv[1], sys.argv[2], sys.argv[3]
with open(path, encoding="utf-8") as f:
    lines = f.read().split("\n")

out = []
for line in lines:
    if line == coerce:
        # Uncomment the coercion: R's own "0 is FALSE, anything else TRUE".
        out.append(coerce[2:])
    elif line == blanket:
        # Comment out the blanket FALSE introduced in upstream 376cffc.
        out.append("# " + blanket)
    else:
        out.append(line)

with open(path, "w", encoding="utf-8") as f:
    f.write("\n".join(out))
PY

# Verify the copy really is patched, and in exactly the intended way.
if ! grep -qxF "${coerce:2}" "$dest/$step"; then
	echo "$0: the coercion line is not active in the copy" >&2
	exit 1
fi
if grep -qxF "$blanket" "$dest/$step"; then
	echo "$0: the blanket FALSE line is still active in the copy" >&2
	exit 1
fi
if [ "$(diff -r --brief "$src" "$dest" 2>/dev/null | grep -vc '^Only in' || true)" != "1" ]; then
	echo "$0: the copy differs from upstream in more than the one file:" >&2
	diff -r --brief "$src" "$dest" | grep -v '^Only in' >&2
	exit 1
fi

echo "patched copy of $src at $dest (step 8 coerces instead of forcing FALSE)"
