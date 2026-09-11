# Habitatus

Zuordnung von EUNIS-Habitaten zu Vegetationsaufnahmen — eine Go-Portierung des
ESy-Expertensystems (Chytrý et al. 2020, Bruelheide et al. 2021).

## Pre-Merge-Gate

Vor jedem Merge werden **lokal** beide Kommandos ausgeführt, und beide müssen
grün sein:

```sh
make check    # Unit-Suite + alle Real-File-Tests gegen die echte Regeldatei
make golden   # Golden Master gegen die R-Implementierung, 11.337 Aufnahmen
```

`make golden` muss **0 winner mismatches, 0 match-set mismatches, 0 differing
expressions** melden. Das ist das Abnahmekriterium der Portierung; eine
Abweichung ist ein Blocker, kein Rundungsfehler.

Voraussetzungen:

- `ESY_FILE` — Pfad zur ESy-Regeldatei. Sie liegt nicht in diesem Repository
  und darf von uns nicht weitergegeben werden. Default siehe `Makefile`.
- Für `make fixtures` zusätzlich eine R-Installation und der Upstream-Klon
  unter `spike/ESy-upstream`. Die Fixtures unter `testdata/golden/` sind
  eingecheckt; neu erzeugt werden sie nur, wenn sich Regeldatei oder Upstream
  ändern (`rulepack.sha256` und `upstream.commit` erzwingen das).

## CI

Die GitHub-Action (`.github/workflows/ci.yml`) führt nur den Teil aus, der ohne
externe Daten auskommt: `go build ./...`, `go vet ./...`, eine
`gofmt -l .`-Prüfung und `go test ./...`. Die Tests hinter `ESY_FILE` sowie der
Golden Master laufen dort **nicht** — die Regeldatei ist nicht öffentlich. Dafür
ist das lokale Pre-Merge-Gate oben zuständig.
