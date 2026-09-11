UPSTREAM ?= spike/ESy-upstream
ESY_FILE ?= $(HOME)/work/projects/eunis/EUNIS-ESy/EUNIS-ESy-2025-10-03.txt
GOLDEN   ?= testdata/golden

.PHONY: test fixtures golden clean-fixtures

test:
	go test ./...

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
