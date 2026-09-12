UPSTREAM ?= spike/ESy-upstream
ESY_FILE ?= $(HOME)/work/projects/eunis/EUNIS-ESy/EUNIS-ESy-2025-10-03.txt
GOLDEN   ?= testdata/golden

# The repaired oracle: a patched COPY of the upstream tree (see
# spike/resy/patch-upstream.sh) and its own fixture set. The clone under
# $(UPSTREAM) stays untouched.
UPSTREAM_REPAIRED ?= build/ESy-upstream-repaired
GOLDEN_REPAIRED   ?= testdata/golden-repaired

.PHONY: test check fixtures fixtures-faithful fixtures-repaired synthetic \
	golden golden-faithful golden-repaired clean-fixtures

test:
	go test ./...

# check runs the ordinary suite, then every real-file test gated behind
# ESY_FILE — the fast assertions run directly against the actual rule file
# (parse counts, formula shapes, reachability), as opposed to `golden`,
# which needs fixtures from `make fixtures` and takes ~280s over 11,337
# plots. A test gated behind an environment variable that nothing invokes
# by default is a test nobody runs: TestGroupsRealFile sat with a stale
# assertion for four tasks because `go test ./...` skips it silently. This
# is the one command meant to catch that class of drift; if a package gets
# its own ESY_FILE-gated real-file test, name it so it matches '-run Real'
# and add its package below, or it goes unchecked the same way.
check: test
	ESY_FILE=$(ESY_FILE) go test ./internal/rulepack/ ./internal/classify/ -run Real -v

# Regenerates both golden-master fixture sets from the upstream R
# implementation: one per evaluation semantics, each against its own oracle.
# Both must end at zero mismatches; see docs/superpowers/specs/
# 2026-09-12-zwei-semantiken.md.
fixtures: fixtures-faithful fixtures-repaired

# synthesize runs first: its rule-driven plots go through the same R runs as
# the Tuexen-Archiv plots and land in both fixture sets. One file, generated
# once, so the two sets differ ONLY in the semantics of the oracle -- feeding
# them different input would make every comparison between them worthless.
synthetic:
	ESY_FILE=$(ESY_FILE) go run spike/resy/synthesize.go $(GOLDEN)/synthetic.jsonl

# The faithful oracle: upstream v1.2 exactly as shipped, never modified.
fixtures-faithful: synthetic
	HABITATUS_SYNTHETIC=$(GOLDEN)/synthetic.jsonl \
	  Rscript spike/resy/generate-fixtures.R $(UPSTREAM) $(ESY_FILE) $(GOLDEN)
	shasum -a 256 $(ESY_FILE) | cut -d' ' -f1 > $(GOLDEN)/rulepack.sha256
	git -C $(UPSTREAM) rev-parse HEAD > $(GOLDEN)/upstream.commit

# The repaired oracle: the same tree with the two lines of step 8 swapped.
# The commit is read from the ORIGINAL clone and handed to the generator,
# because the patched copy carries no .git -- so the staleness guard compares
# both fixture sets against the same upstream revision.
fixtures-repaired: synthetic
	spike/resy/patch-upstream.sh $(UPSTREAM) $(UPSTREAM_REPAIRED)
	HABITATUS_SYNTHETIC=$(GOLDEN)/synthetic.jsonl \
	  HABITATUS_UPSTREAM_COMMIT=$$(git -C $(UPSTREAM) rev-parse HEAD) \
	  Rscript spike/resy/generate-fixtures.R $(UPSTREAM_REPAIRED) $(ESY_FILE) $(GOLDEN_REPAIRED)
	shasum -a 256 $(ESY_FILE) | cut -d' ' -f1 > $(GOLDEN_REPAIRED)/rulepack.sha256
	git -C $(UPSTREAM) rev-parse HEAD > $(GOLDEN_REPAIRED)/upstream.commit

# golden runs both golden masters (~570s). The two halves are also available
# on their own; each is a subtest named after its mode.
golden:
	ESY_FILE=$(ESY_FILE) go test ./internal/esy/ -run TestGolden -v -timeout 30m

golden-faithful:
	ESY_FILE=$(ESY_FILE) go test ./internal/esy/ -run 'TestGolden.*/faithful' -v -timeout 30m

golden-repaired:
	ESY_FILE=$(ESY_FILE) go test ./internal/esy/ -run 'TestGolden.*/repaired' -v -timeout 30m

clean-fixtures:
	rm -f $(GOLDEN)/*.jsonl $(GOLDEN)/*.json $(GOLDEN)/rulepack.sha256 $(GOLDEN)/upstream.commit
	rm -f $(GOLDEN_REPAIRED)/*.jsonl $(GOLDEN_REPAIRED)/*.json \
	  $(GOLDEN_REPAIRED)/rulepack.sha256 $(GOLDEN_REPAIRED)/upstream.commit
