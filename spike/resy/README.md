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

### Header column names are normalised

`generate-fixtures.R` feeds upstream the bundled data verbatim except for two
header column names, and anyone re-running this after an upstream bump needs
to know about it. Both are in one explicit mapping:

```r
header.renames <- c(dataset = "Dataset", "Altitude..m." = "Altitude (m)")
```

An explicit list, not a blanket case-fold, so the next maintainer can see
exactly what was touched and extend it if another column drifts.
`synthesize.go`'s `headerFields` carries the same names, because `rbind`
demands identical columns.

Upstream binds a header column to a rule's `$$C`/`$$N` field **by name**, and
the test is case-sensitive (`step3and5…R:407` and `:395`):

```r
categorical.header <- intersect(names(header), substr(w, 5, nchar(w)))
```

#### `dataset` → `Dataset`

The bundled CSV spells its dataset column `dataset`; the rule file asks for
`$$C Dataset`. The `intersect` misses, both condition columns keep their zero
default, and `<$$C Dataset EQ Swedish National Forest Inventory>` is `0 == 0`
— TRUE for every plot. That is not a semantic of the expert system to
reproduce: it is a column name that does not match the field the rules
reference. A caller supplying a properly named header — as habitatus
requires — gets FALSE out of R too. So the generator renames it.

Verified safe for this data: `dataset` holds one distinct value,
`Germany Vegetweb 2`, which is not itself a condition string, so filling the
column cannot collide with another field's levels.

Blast radius, measured rather than assumed: exactly one expression in the rule
file names `$$C Dataset`, and the only rule using it is `U21`, one of the 100
that can never fire. Re-running the generator flipped that one expression from
TRUE to FALSE on every plot and moved nothing else — `expected.jsonl` came
out byte-identical. `TestGoldenExpressions` is back to requiring **zero**
disagreements, with no exception mechanism.

#### `Altitude..m.` → `Altitude (m)`

`read.csv` also mangles `Altitude (m)` into `Altitude..m.`, and that one is
renamed for a different reason than `dataset`.

It never caused a *disagreement*: an unmatched **numeric** field is 0 in R and
0 in habitatus, so both sides always answered the same. But that is agreement
on a degenerate case. With the column invisible to both, the 39
`$$N Altitude` conditions across 24 rules were never exercised at all — the
golden master was confirming that two implementations do nothing with a field
neither can see. Restoring the name turns those altitude gates from untested
into tested, on plots carrying real altitudes (the archive runs 0.1 m to
1900 m).

Measured effect of adding it to `header.renames`:

* **Still zero mismatches** — winner, match set and expressions all agree, so
  the altitude path was already correct on both sides. No bugs surfaced.
* Rule coverage rose from **195 to 204** rules firing. Nine rules moved out of
  "reachable but never triggered": `T1C`, `T1C!`, `T1C!!`, `T1D!`, `T1D!!`,
  `T31`, `T34`, `T34!`, `T37`, `S26`.
* The condition column went from all-zero to a real range, and five of the ten
  altitude expressions now take **both** truth values across the sampled
  plots instead of being pinned to one.
* 24 of 11,337 plots (0.212%) changed winner, every one of them from a broad
  parent type to a specific subtype — `T` → `T3M`/`T1C`/`T1D!`/`T31`/`T34`/
  `T37`, `Sa` → `S26` — which is what an altitude gate opening looks like.
* The unambiguous-assignment rate on the archive did **not** move: 89.42%
  before and after. Those plots were already assigned, just less
  specifically. So altitude is not the source of the gap to the published
  94% (see `task-11-report.md`, concern 1).

Residual: the thresholds above 900 m are thinly covered. Only 10 archive plots
exceed 900 m and 1 exceeds 1500 m, and the 300-plot stride behind
`TestGoldenExpressions` happens to miss them, so four altitude expressions show
a single truth value in that sample. `TestGoldenMaster` runs over all 11,337
plots and does exercise them.

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
