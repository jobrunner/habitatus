# Habitatus

Zuordnung von EUNIS-Habitaten zu Vegetationsaufnahmen — eine Go-Portierung des
ESy-Expertensystems (Chytrý et al. 2020, Bruelheide et al. 2021).

## Zwei Semantiken

Der Dienst kennt zwei Auswertungsarten, umgeschaltet mit `-mode`:

| Modus | Semantik | Zweck |
|---|---|---|
| `repaired` (Standard) | wie ESy **vor** v1.2: Ausdrücke ohne Vergleichsoperator werden über ihren Zahlenwert gewandelt (0 = falsch, sonst wahr) | der Dienst, den wir betreiben |
| `faithful` | wie ESy **v1.2**: dieselben Ausdrücke sind immer falsch | Paritätsnachweis gegen v1.2 |

Unterschiedlich ausgewertet werden genau 128 Ausdrücke; alles andere ist in
beiden Modi identisch. In `faithful` können dadurch 100 der 312 Regeln nie
feuern (`Stats.Unreachable`), in `repaired` keine. Der Modus steht in
`versions` jeder Antwort und im Startlog. Begründung und Belege:
[`docs/superpowers/specs/2026-09-12-zwei-semantiken.md`](docs/superpowers/specs/2026-09-12-zwei-semantiken.md).

## Pre-Merge-Gate

Vor jedem Merge werden **lokal** beide Kommandos ausgeführt, und beide müssen
grün sein:

```sh
make check    # Unit-Suite + alle Real-File-Tests gegen die echte Regeldatei
make golden   # beide Golden Master gegen die R-Implementierung, je 11.337 Aufnahmen
```

`make golden` prüft **beide** Modi gegen je ein eigenes Orakel — `faithful`
gegen den unveränderten Upstream, `repaired` gegen dieselbe Fassung mit
getauschten Zeilen in Schritt 8 — und muss für beide **0 winner mismatches,
0 match-set mismatches, 0 differing expressions** melden. Das ist das
Abnahmekriterium der Portierung; eine Abweichung ist ein Blocker, kein
Rundungsfehler. Einzeln laufen die Hälften mit `make golden-faithful` bzw.
`make golden-repaired`.

Voraussetzungen:

- `ESY_FILE` — Pfad zur ESy-Regeldatei. Sie liegt derzeit nicht in diesem
  Repository; Default siehe `Makefile`. Weitergabe ist erlaubt: der
  Zenodo-Record doi:10.5281/zenodo.3841729 steht in allen drei Versionen
  (2020-06-08, 2021-06-01, 2025-10-03) unter **CC BY 4.0** bei offenem Zugang,
  verlangt also nur Namensnennung.
- Für `make fixtures` zusätzlich eine R-Installation und der Upstream-Klon
  unter `spike/ESy-upstream`. Die Fixtures unter `testdata/golden/` und
  `testdata/golden-repaired/` sind eingecheckt; neu erzeugt werden sie nur,
  wenn sich Regeldatei oder Upstream ändern (`rulepack.sha256`,
  `upstream.commit` und der Eintrag `mode` in `meta.json` erzwingen das).
  Für den `repaired`-Satz patcht `spike/resy/patch-upstream.sh` eine **Kopie**
  des Upstreams unter `build/`; der Klon selbst bleibt unangetastet.

## CI

Die GitHub-Action (`.github/workflows/ci.yml`) führt nur den Teil aus, der ohne
externe Daten auskommt: `go build ./...`, `go vet ./...`, eine
`gofmt -l .`-Prüfung und `go test ./...`. Die Tests hinter `ESY_FILE` sowie der
Golden Master laufen dort **nicht** — die Regeldatei ist nicht öffentlich. Dafür
ist das lokale Pre-Merge-Gate oben zuständig.
