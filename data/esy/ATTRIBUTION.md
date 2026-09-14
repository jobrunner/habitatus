# Attribution — EUNIS-ESy rule file

`EUNIS-ESy-2025-10-03.txt` in this directory is vendored, unmodified, from:

> Chytrý, M., Tichý, L., Hennekens, S. M., Knollová, I., Janssen, J. A. M.,
> Rodwell, J. S., Peterka, T., Marcenò, C., Landucci, F., Danihelka, J., et al.
> (2020, updated 2025). *EUNIS-ESy: Expert system for automatic classification
> of European vegetation plots to EUNIS habitats.* Version 2025-10-03. Zenodo.
> https://doi.org/10.5281/zenodo.3841729

- **License:** Creative Commons Attribution 4.0 International (CC BY 4.0) —
  https://creativecommons.org/licenses/by/4.0/
- **DOI:** 10.5281/zenodo.3841729 (concept DOI; resolves to the versioned
  record for 2025-10-03, currently https://zenodo.org/records/16895007)
- **Version:** 2025-10-03
- **File:** `EUNIS-ESy-2025-10-03.txt`, 8,467,096 bytes
- **SHA-256:** `724ad8611e0ddb9e8a39e54aae840119a2219202f74a1e8b208d0a65d35d235f`

This checksum must match `testdata/golden/rulepack.sha256` and
`testdata/golden-repaired/rulepack.sha256` — both golden masters refuse to run
against a rule file whose digest has drifted from the one the fixtures were
generated against (see `README.md`, "Pre-Merge-Gate").

CC BY 4.0 requires attribution on redistribution. This file satisfies that
both ways the rule file travels: it sits next to the vendored copy in the
repository, and the container image built from `Dockerfile` carries the same
file at `/data/esy/ATTRIBUTION.md` alongside the rule file it describes.

## Backbone tables (not yet vendored)

The 48 nomenclature-translation ("backbone") tables that ship in the same
Zenodo record (`Nomenclature-translation-from-Turboveg-2-databases.zip`,
~43 MB unpacked) are not vendored here yet. To add them later: drop the
extracted tables under `data/esy/backbones/` and add one `COPY` line for that
directory to `Dockerfile` — nothing else needs to change, `-backbones`
already accepts a directory path. See the matching comment in `Dockerfile`.
