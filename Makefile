UPSTREAM ?= spike/ESy-upstream
ESY_FILE ?= $(HOME)/work/projects/eunis/EUNIS-ESy/EUNIS-ESy-2025-10-03.txt
GOLDEN   ?= testdata/golden

.PHONY: test check fixtures golden clean-fixtures

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

# Regenerates the golden-master fixtures from the upstream R implementation.
# synthesize runs first: its rule-driven plots go through the same R run as the
# Tuexen-Archiv plots and land in the same fixtures.
fixtures:
	ESY_FILE=$(ESY_FILE) go run spike/resy/synthesize.go $(GOLDEN)/synthetic.jsonl
	HABITATUS_SYNTHETIC=$(GOLDEN)/synthetic.jsonl \
	  Rscript spike/resy/generate-fixtures.R $(UPSTREAM) $(ESY_FILE) $(GOLDEN)
	shasum -a 256 $(ESY_FILE) | cut -d' ' -f1 > $(GOLDEN)/rulepack.sha256
	git -C $(UPSTREAM) rev-parse HEAD > $(GOLDEN)/upstream.commit

golden:
	ESY_FILE=$(ESY_FILE) go test ./internal/esy/ -run TestGolden -v

clean-fixtures:
	rm -f $(GOLDEN)/*.jsonl $(GOLDEN)/*.json $(GOLDEN)/rulepack.sha256 $(GOLDEN)/upstream.commit
