# Drives the upstream ESy implementation (v1.2) to produce golden-master
# fixtures for habitatus. The upstream tree is never modified: this script
# only sources its code and reads the objects it leaves behind.
#
# Usage:
#   Rscript spike/resy/generate-fixtures.R <upstream-dir> <esy-file> <out-dir>
#
# Optional environment variables:
#   HABITATUS_SYNTHETIC  path to a JSONL file of synthetic plots (see
#                        spike/resy/synthesize.go). Its plots are appended to
#                        the Tuexen-Archiv plots and run through the same
#                        upstream pipeline.
#   HABITATUS_INTERMEDIATE_PLOTS
#                        comma-separated RELEVE_NRs whose per-condition values
#                        and per-expression truth values are written to
#                        intermediates.jsonl. Default: the first 200 plots.
#
# The calling sequence follows the maintainers' own Readme_Example-code.R:
# prep.R, then step1and2, then the caller must allocate plot.cond itself
# (upstream's step3and5 writes into it but never creates it), then step4, then
# step3and5. plot.cond's rows are unique(obs$RELEVE_NR), and step 5.10 indexes
# header by row position, so header must be in exactly that order.

args <- commandArgs(trailingOnly = TRUE)
if (length(args) != 3) stop("need <upstream-dir> <esy-file> <out-dir>")
upstream <- normalizePath(args[1])
esyfile <- normalizePath(args[2])
dir.create(args[3], recursive = TRUE, showWarnings = FALSE)
outdir <- normalizePath(args[3])

synth <- Sys.getenv("HABITATUS_SYNTHETIC")
if (nzchar(synth)) synth <- normalizePath(synth, mustWork = FALSE)

owd <- setwd(upstream)
on.exit(setwd(owd))

source("code/prep.R")
suppressPackageStartupMessages(library(jsonlite))

# ---------------------------------------------------------------- input data
obs <- fread(file.path("data", "obs_100716Hoppe2005.csv"), encoding = "UTF-8")
header <- read.csv(file.path("data", "header_100716Hoppe2005.csv"))

# The one place this generator does not feed the bundled data verbatim.
#
# Upstream binds a header column to a rule's "$$C"/"$$N" field by name, and
# the test is case-sensitive (step3and5...R:407,395):
#
#	categorical.header <- intersect(names(header), substr(w, 5, nchar(w)))
#
# The bundled CSV spells its dataset column "dataset" while the rule file asks
# for "$$C Dataset", so the intersect misses, both condition columns keep
# their zero default, and "<$$C Dataset EQ ...>" is 0 == 0 -- TRUE for every
# plot. That is not a semantic of the expert system to reproduce; it is a
# column name that does not match the field the rules reference, and any
# caller supplying a properly named header gets FALSE out of R too.
#
# Renamed explicitly rather than by a blanket case-fold, so the next
# maintainer can see exactly what was touched and extend the list if another
# column ever drifts. Verified safe for this data: "dataset" holds one
# distinct value, "Germany Vegetweb 2", which is not itself a condition
# string, so filling the column cannot collide with another field's levels.
#
# read.csv also mangles "Altitude (m)" into "Altitude..m.", and that one is
# renamed for a different reason. It causes no DISAGREEMENT -- an unmatched
# numeric field is 0 in R and 0 in habitatus -- but that is agreement on a
# degenerate case: the 39 "$$N Altitude" conditions across 24 rules are never
# exercised at all, because neither side can see the column. Restoring the
# name turns the altitude gates of those rules from untested into tested, on
# plots that carry real altitudes. It does move the baseline, deliberately.
header.renames <- c(dataset = "Dataset", "Altitude..m." = "Altitude (m)")
for (from in names(header.renames)) {
  i <- match(from, names(header))
  if (!is.na(i)) {
    names(header)[i] <- header.renames[[from]]
    message("renamed header column '", from, "' to '", header.renames[[from]],
            "' to match the rule file's field name")
  }
}

# 422 plots carry no coordinates; (0,0) would put them in the Atlantic off
# West Africa, where the coastal and biogeographic rules behave differently.
keep <- !(header$DEG_LON == 0 & header$DEG_LAT == 0)
message("dropping ", sum(!keep), " plots without coordinates")
header <- header[keep, ]
obs <- obs[obs$RELEVE_NR %in% header$RELEVE_NR, ]

# Synthetic, rule-driven plots. They share the header schema and use RELEVE_NRs
# far above the archive's, so the two sets never collide.
if (nzchar(synth) && file.exists(synth)) {
  sp <- jsonlite::stream_in(file(synth), verbose = FALSE)
  message("adding ", nrow(sp), " synthetic plots from ", synth)
  srecs <- rbindlist(lapply(seq_len(nrow(sp)), function(i) {
    r <- sp$records[[i]]
    data.table(RELEVE_NR = as.integer(sp$id[i]),
               TaxonName = as.character(r$name),
               Cover_Perc = as.numeric(r$cover))
  }))
  obs <- rbind(obs, srecs)
  sh <- sp$header
  shdr <- data.frame(RELEVE_NR = as.integer(sp$id), stringsAsFactors = FALSE)
  for (nm in names(header)) {
    if (nm == "RELEVE_NR") next
    v <- if (nm %in% names(sh)) sh[[nm]] else rep(NA, nrow(sp))
    shdr[[nm]] <- if (is.numeric(header[[nm]])) as.numeric(v) else as.character(v)
  }
  header <- rbind(header, shdr)
}

stopifnot(identical(as.integer(unique(obs$RELEVE_NR)), as.integer(header$RELEVE_NR)))

obs.raw <- copy(obs) # step4 aggregates obs in place; the cases keep the input

# ------------------------------------------------------- drive the pipeline
mc <- getOption("mc.cores", 3)
expertfile <- esyfile
source("code/step1and2_load-and-parse-the-expert-file.R")

message("formulas: ", length(vegtype.formulas),
        "  conditions: ", length(conditions),
        "  expressions: ", length(membership.expressions),
        "  groups: ", length(groups))
dup <- anyDuplicated(vegtype.formula.names.short)
if (dup > 0) message("WARNING duplicated short type label at ", dup, ": ",
                     vegtype.formula.names.short[dup])

plot.cond <- array(0, c(length(unique(obs$RELEVE_NR)), length(conditions)),
                   dimnames = list(as.character(unique(obs$RELEVE_NR)), conditions))
source("code/step4_aggregate-taxon-levels.R")
source("code/step3and5_extract-and-solve-membership-conditions.R")

ids <- as.integer(unique(obs$RELEVE_NR))
stopifnot(length(result.classification) == length(ids))

# ------------------------------------------------------------------- labels
# Upstream's short type label is trim(substr(name, 1, 6)) over the text from
# column 12 on. Columns 12-16 are the five-character code field, so character 6
# is the first letter of the habitat name, glued on whenever the code is
# shorter than five characters ("T1H  B", "N15!!A", "MAa  A"). Characters 1-5
# are the code field alone, so trimming those recovers code plus variant
# marker exactly, which is what habitatus parses.
bare <- function(x) ifelse(x %in% c("?", "+"), x, trimws(substr(x, 1, 5)))

# ------------------------------------------------------------------ outputs
w <- function(name) file(file.path(outdir, name), "w")

writeLines(toJSON(list(
  n_plots = length(ids),
  n_rules = length(vegtype.formulas),
  n_conditions = length(conditions),
  n_expressions = length(membership.expressions),
  upstream_commit = system("git rev-parse HEAD", intern = TRUE),
  esy_file = basename(esyfile),
  r_version = R.version.string,
  generated = format(Sys.time(), "%Y-%m-%dT%H:%M:%S%z")
), auto_unbox = TRUE), file.path(outdir, "meta.json"))

# cases.jsonl -- the input side, exactly what habitatus is handed.
setkey(obs.raw, RELEVE_NR)
hdrnames <- setdiff(names(header), "RELEVE_NR")
# as.character on a numeric vector formats each element on its own, unlike
# format(), which pads them to a common width ("1.00", "53.900").
asstr <- function(v) {
  s <- as.character(v)
  s[is.na(v)] <- ""
  s
}
hdrstr <- lapply(hdrnames, function(n) asstr(header[[n]]))
names(hdrstr) <- hdrnames

con <- w("cases.jsonl")
for (i in seq_along(ids)) {
  sp <- obs.raw[.(ids[i])]
  writeLines(toJSON(list(
    id = ids[i],
    records = list(name = I(sp$TaxonName), cover = I(sp$Cover_Perc)),
    header = lapply(hdrstr, function(col) col[i])
  ), auto_unbox = TRUE, dataframe = "columns", digits = NA), con)
}
close(con)

# expected.jsonl -- the output side: every type that fired, and the winner.
con <- w("expected.jsonl")
for (i in seq_along(ids)) {
  hits <- types[[i]]
  writeLines(toJSON(list(
    id = ids[i],
    winner = bare(unname(result.classification[i])),
    winner_upstream = unname(result.classification[i]),
    matches = I(unname(bare(hits))) # I() keeps a single match an array
  ), auto_unbox = TRUE), con)
}
close(con)

# The static side of the intermediates: which expressions and conditions exist,
# and which expression indices each rule's formula is built from, in order.
writeLines(toJSON(conditions), file.path(outdir, "conditions.json"))
writeLines(toJSON(membership.expressions), file.path(outdir, "expressions.json"))

inner <- function(s) {
  a <- gregexpr("<", s, fixed = TRUE)[[1]]
  b <- gregexpr(">", s, fixed = TRUE)[[1]]
  if (a[1] < 0) return(character(0))
  substr(rep(s, length(a)), a + 1, b - 1)
}
rule.exprs <- lapply(seq_along(vegtype.formulas), function(i) {
  list(label = unname(bare(vegtype.formula.names.short[i])),
       short = unname(vegtype.formula.names.short[i]),
       priority = as.character(vegtype.priority[i]),
       exprs = I(unname(match(inner(vegtype.formulas[i]), membership.expressions))))
})
writeLines(toJSON(rule.exprs, auto_unbox = TRUE), file.path(outdir, "rule-exprs.json"))

# intermediates.jsonl -- per-plot condition values and expression truth values.
# This is the seam that says WHICH condition diverged, not merely that one did.
sel <- Sys.getenv("HABITATUS_INTERMEDIATE_PLOTS")
if (nzchar(sel)) {
  want <- match(as.integer(strsplit(sel, ",", fixed = TRUE)[[1]]), ids)
  want <- want[!is.na(want)]
} else {
  # An even stride over the whole set, so the sample spans the archive and the
  # synthetic plots rather than the first few hundred archive plots.
  want <- unique(round(seq(1, length(ids), length.out = min(300, length(ids)))))
}
L1 <- do.call(cbind, logi1) # plots x expressions, logical
con <- w("intermediates.jsonl")
for (i in want) {
  v <- as.numeric(plot.cond[i, ])
  nz <- which(v != 0)
  tr <- which(L1[i, ])
  writeLines(toJSON(list(
    id = ids[i],
    cond_index = I(nz), cond_value = I(v[nz]),
    expr_true = I(tr)
  ), auto_unbox = TRUE, digits = NA), con)
}
close(con)

message("wrote ", length(ids), " cases and ", length(want),
        " intermediates to ", outdir)
message("unassigned '?': ", sum(result.classification == "?"),
        "  ambiguous '+': ", sum(result.classification == "+"),
        "  assigned: ", sum(!result.classification %in% c("?", "+")),
        sprintf("  (%.1f%%)", 100 * mean(!result.classification %in% c("?", "+"))))
