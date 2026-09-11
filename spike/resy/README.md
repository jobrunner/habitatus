# R spike: golden master against upstream ESy v1.2

This directory drives the upstream R implementation
(`spike/ESy-upstream`, commit 416bab9, version 1.2) to produce the fixtures
`internal/esy/golden_test.go` compares habitatus against. **Upstream is never
modified** — the scripts only source its code and read the objects it leaves
behind.

    make fixtures   # regenerate testdata/golden/
    make golden     # run the golden master against them

## Files

| file | what it does |
| --- | --- |
| `generate-fixtures.R` | drives upstream and writes `testdata/golden/` |
| `synthesize.go` | builds rule-driven plots so rules the German archive cannot reach are still compared (`//go:build ignore`) |
| `diagnose.go` | compares the two sides and reports where they differ, down to the single expression (`//go:build ignore`) |

## The calling sequence

`Readme_Example-code.R` is the maintainers' own runnable example and is the
authority on how the pieces fit together:

1. `source('code/prep.R')` — packages and helper functions.
2. read `data/obs_100716Hoppe2005.csv` and
   `data/header_100716Hoppe2005.csv`.
3. set `expertfile`, then `source('code/step1and2_...R')`.
4. **allocate `plot.cond` yourself** — `array(0, c(n_plots, n_conditions),
   dimnames = list(as.character(unique(obs$RELEVE_NR)), conditions))`.
   `step3and5` writes into this matrix but never creates it, so the run dies
   without it. This is the single most important thing the example teaches.
5. `source('code/step4_...R')` — taxon aggregation. It rewrites `obs` in
   place, so anything that needs the raw observations must copy them first.
6. `source('code/step3and5_...R')` — conditions, expressions, formulas,
   classification. It leaves `plot.cond`, `logi1`, `logi2`, `types` and
   `result.classification` behind.

Two invariants follow from step 4 and are asserted by the script:

* `plot.cond`'s rows are `unique(obs$RELEVE_NR)`, and step 5.10 indexes
  `header` by row position, so `header` must be in exactly that order.
* every plot in `header` must have at least one observation, or the two go out
  of step. Synthetic plots therefore always carry species.

`mc` must exist before `step3and5` is sourced; the example sets it with
`getOption("mc.cores", 3)`.

`prep.R` opens with `library(profvis)` among others, so profvis has to be
installed even though nothing in the pipeline profiles anything.

## Input

`data/obs_100716Hoppe2005.csv` (155,039 observations) and
`data/header_100716Hoppe2005.csv` (10,717 plots) are the Tüxen-Archiv extract
bundled with upstream. 422 plots sit at coordinates (0, 0) — no coordinates,
which would place them in the Atlantic off West Africa — and are dropped,
leaving 10,295.

The header columns come from `read.csv`, which turns `Altitude (m)` into
`Altitude..m.`. The rule file asks for `$$N Altitude (m)` and `$$C Dataset`,
and neither matches a column, so upstream leaves both at zero for every plot.
The fixtures carry the header keys verbatim, so habitatus sees exactly what R
saw and reproduces the same behaviour. Renaming them would exercise those
conditions but would also change the archive baseline, so the schema is left
as upstream's own data has it.

## Output

Written to `testdata/golden/`:

| file | contents |
| --- | --- |
| `cases.jsonl` | one plot per line: id, raw records, header |
| `expected.jsonl` | id, winner, all types that fired, plus upstream's raw label |
| `conditions.json` | the condition strings, in `plot.cond` column order |
| `expressions.json` | the membership expressions, in `logi1` order |
| `rule-exprs.json` | per rule, the expression indices its formula uses, in order |
| `intermediates.jsonl` | per-condition values and per-expression truth values |
| `synthetic.jsonl` | the generated plots, before they go through R |
| `meta.json`, `rulepack.sha256`, `upstream.commit` | provenance |

`intermediates.jsonl` is what makes a divergence diagnosable: comparing only
the final EUNIS code says *that* something differs, comparing conditions says
*which one*. `TestGoldenExpressions` uses it to compare every membership
expression of every rule against upstream's `logi1`, which is where two
errors cancelling inside one formula would show up. It defaults to an even
stride of 300 plots across the archive and the synthetic plots; set
`HABITATUS_INTERMEDIATE_PLOTS=<id,id,...>` to dump specific ones, then

    go run spike/resy/diagnose.go plot <id> <rule>

prints every expression of that rule with habitatus's truth value and both
operand values next to R's, marking the lines that differ. `diagnose.go tally`
reports which rules the two sides disagree about over all plots.

## Labels

Upstream's short type label is `trim(substr(name, 1, 6))` over the text from
column 12 of the rule header. Columns 12–16 are the five-character code field,
so character 6 is the first letter of the habitat name, glued on whenever the
code is shorter than five characters: `T1H  B`, `N15!!A`, `MAa  A`. Since
`classify()` uses that same string on both sides of its `match()`, it is a
label artefact and never changes which rule matched. The script normalises it
back to the code field with `trim(substr(label, 1, 5))`, which keeps the
variant marker (`N15!!`) that cutting at the first `!` would lose.

One short label is not unique: twelve rules are named `T3M  Coniferous
plantation of non site-native trees 1..12` and all collapse to `T3M  C`.
Upstream's own `anyDuplicated` check reports it and then carries on.
