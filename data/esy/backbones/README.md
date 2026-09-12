# Backbone tables (not yet vendored)

This directory is intentionally empty. The 48 nomenclature-translation
tables from the Zenodo record (doi:10.5281/zenodo.3841729,
`Nomenclature-translation-from-Turboveg-2-databases.zip`, ~43 MB unpacked)
are not vendored yet — see `../ATTRIBUTION.md`.

To add them: extract the tables into this directory, then add one `COPY`
line for this directory to `Dockerfile` (see the matching comment there).
`-backbones` already accepts a directory path, so nothing else needs to
change.
