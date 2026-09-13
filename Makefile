UPSTREAM ?= spike/ESy-upstream
# The vendored copy under data/esy/ — CC BY 4.0, see data/esy/ATTRIBUTION.md.
# Override to run against a different rule-file version; its checksum must
# then match what testdata/golden{,-repaired}/rulepack.sha256 expect, or the
# golden masters' staleness guard refuses to run.
ESY_FILE ?= $(CURDIR)/data/esy/EUNIS-ESy-2025-10-03.txt
GOLDEN   ?= testdata/golden

# The repaired oracle: a patched COPY of the upstream tree (see
# spike/resy/patch-upstream.sh) and its own fixture set. The clone under
# $(UPSTREAM) stays untouched.
UPSTREAM_REPAIRED ?= build/ESy-upstream-repaired
GOLDEN_REPAIRED   ?= testdata/golden-repaired

.PHONY: test check fixtures fixtures-faithful fixtures-repaired synthetic \
	golden golden-faithful golden-repaired clean-fixtures \
	lint cover bench fuzz mutation licenses sbom codecharta quality

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

# release-please owns the VERSION file; the git fallback keeps a checkout
# without one buildable.
VERSION ?= $(shell head -n1 VERSION 2>/dev/null || git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)

.PHONY: docker-build docker-run

docker-build:
	docker build --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) -t habitatus:$(VERSION) -t habitatus:latest .

# Runs the hardened image the same way `make docker-build`'s output is meant
# to be operated: read-only root fs, no capabilities, no privilege
# escalation. If this doesn't work, the Dockerfile is wrong, not the flags.
docker-run:
	docker run --rm -p 127.0.0.1:8080:8080 \
	  --read-only --cap-drop=ALL --security-opt=no-new-privileges \
	  habitatus:latest

# =============================================================================
# Quality harness. `make quality` is the local equivalent of the CI gate — run
# it before opening a PR and the required checks should pass.
# =============================================================================

BENCHCOUNT ?= 6
# Executions, not wall-clock. A duration makes the gate do less work on a slow
# runner and, worse, races Go's own fuzzing shutdown: when the timer fires
# mid-execution the coordinator can report "context deadline exceeded" as a
# test failure with no crasher to show for it. That turned a required check
# into a coin flip. An execution count is deterministic across machines.
FUZZTIME   ?= 1000000x

lint:
	golangci-lint run --timeout=5m

# cover runs the suite with the race detector and enforces the per-package
# floors in .coverage-floors. The floors are a raise-only ratchet: the fix for
# a failure is to add tests.
cover:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	./scripts/coverage-gate.sh coverage.out

# Benchmarks are a compile-and-run gate, not a timing threshold: shared CI
# runners are far too noisy for a per-PR latency bound. CI posts a benchstat
# delta against the base branch for a human to read.
bench:
	ESY_FILE=$(ESY_FILE) go test -run '^$$' -bench . -benchmem -count=$(BENCHCOUNT) ./...

# The rule file is third-party input. These targets fuzz the parsers that read
# it; a crash found here is a start-up crash avoided in production.
# FUZZTIME takes either form — `make fuzz FUZZTIME=10m` for a long hunt.
fuzz:
	@for t in FuzzParseExpr FuzzParseFormula FuzzSplitSections FuzzLoad; do \
	  echo "==> $$t"; \
	  go test -run '^$$' -fuzz "$$t" -fuzztime=$(FUZZTIME) ./internal/rulepack/ || exit 1; \
	done

# Mutation testing: coverage says a line ran, mutation says a test would have
# caught a bug in it. The thresholds are a ratchet, raised as tests improve.
mutation:
	gremlins unleash ./internal/esy --timeout-coefficient=20 \
	  --threshold-efficacy=78 --threshold-mcover=88

# Every dependency of the shipped binary must carry a permissive licence. The
# core has no third-party dependencies at all, so this gate mainly guards
# against one being added unnoticed.
# Every package reachable from the binary must carry a permissive licence,
# first-party included: go-licenses classifies each package by the nearest
# LICENSE file, and this repository's is MIT. The core has no third-party
# dependencies at all, so in practice this guards against the first one being
# added under a licence we cannot ship.
licenses:
	go-licenses check ./cmd/habitatus \
	  --allowed_licenses=Apache-2.0,BSD-2-Clause,BSD-3-Clause,MIT,ISC,MPL-2.0,Unlicense

# A Software Bill of Materials for the source tree, in both formats consumers
# ask for. The release image carries its own SBOM attestation as well.
sbom:
	mkdir -p build
	syft scan dir:. -o spdx-json=build/sbom.spdx.json -o cyclonedx-json=build/sbom.cyclonedx.json
	@echo "wrote build/sbom.spdx.json and build/sbom.cyclonedx.json"

# The CodeCharta map plus its ratchet gate. Needs ccsh (npm install -g
# codecharta-analysis), a JRE and gcov2lcov.
codecharta:
	mkdir -p build
	ccsh unifiedparser . -fe=go -e='_test\.go,vendor,spike,build' -nc -o build/base.cc.json
	go test -coverprofile=build/cc.out ./... || true
	gcov2lcov -infile=build/cc.out -outfile=build/coverage.info
	ccsh coverageimport build/coverage.info -f lcov -nc -o build/coverage.cc.json
	ccsh gitlogparser repo-scan --repo-path=. --add-author --silent -nc -o build/git.cc.json
	ccsh merge build/base.cc.json build/git.cc.json build/coverage.cc.json -o build/habitatus.cc.json.gz
	python3 scripts/codecharta-ratchet.py build/habitatus.cc.json.gz .codecharta-ratchet.json

# The CI gates that need no tooling beyond Go: lint, tests with the coverage
# ratchet, the real-file tests, benchmarks, a short fuzz pass and the licence
# check. Deliberately NOT the same set as CI — `sbom` needs syft, `codecharta`
# needs ccsh and a JRE, and `golden` takes eleven minutes. Run those before a
# change that touches what they cover; CI runs all of them regardless.
quality: lint cover check bench licenses
	$(MAKE) fuzz FUZZTIME=200000x
