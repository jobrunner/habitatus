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

- `ESY_FILE` — Pfad zur ESy-Regeldatei. Sie liegt unter `data/esy/` in
  diesem Repository; der `Makefile`-Default zeigt darauf, überschreibbar für
  eine andere Version. Weitergabe ist erlaubt: der Zenodo-Record
  doi:10.5281/zenodo.3841729 steht in allen drei Versionen (2020-06-08,
  2021-06-01, 2025-10-03) unter **CC BY 4.0** bei offenem Zugang, verlangt
  also nur Namensnennung — siehe `data/esy/ATTRIBUTION.md`.
- Für `make fixtures` zusätzlich eine R-Installation und der Upstream-Klon
  unter `spike/ESy-upstream`. Die Fixtures unter `testdata/golden/` und
  `testdata/golden-repaired/` sind eingecheckt; neu erzeugt werden sie nur,
  wenn sich Regeldatei oder Upstream ändern (`rulepack.sha256`,
  `upstream.commit` und der Eintrag `mode` in `meta.json` erzwingen das).
  Für den `repaired`-Satz patcht `spike/resy/patch-upstream.sh` eine **Kopie**
  des Upstreams unter `build/`; der Klon selbst bleibt unangetastet.

## Qualitäts-Harness

`main` ist durch ein Ruleset geschützt: gemergt wird nur, wenn **alle**
Pflicht-Checks grün sind. Lokal entspricht dem `make quality`; die einzelnen
Ziele lassen sich auch einzeln laufen.

| Check | Werkzeug | Was er verhindert |
|---|---|---|
| Lint | golangci-lint (25 Linter) | die üblichen Fehlerklassen |
| Lint / depguard | golangci-lint | Verletzung der Schichtgrenzen aus der Spec — `cover` ← `taxa` ← `esy` ← `classify` ← Adapter, keine Adapter-Querkopplung |
| Test | `go test -race` + Real-File-Tests | Regression gegen die echte Regeldatei |
| Test / Coverage-Ratchet | `scripts/coverage-gate.sh` | stilles Absinken der Testabdeckung je Paket |
| Benchmarks | `go test -bench` + benchstat | Bit-Rot der Benchmarks; der Zeitvergleich ist Lesestoff, kein Gate |
| Fuzz (smoke) | `go test -fuzz`, 30 s je Ziel | Panics der Parser auf fremden Regeldateien |
| Security | govulncheck | bekannte Schwachstellen in Go-Code |
| License Compliance | go-licenses | eine Abhängigkeit unter unpassender Lizenz |
| SBOM | syft (SPDX + CycloneDX), grype | fehlende Stückliste; bekannte Lücken darin |
| Secret Scan | gitleaks | Zugangsdaten in der Historie |
| Architecture | `go mod tidy -diff`, Abhängigkeitsfreiheit, Regeldatei-Prüfsumme | eine unbemerkt eingeführte Abhängigkeit; eine stille Änderung der vendorierten Regeldatei, die jede Verifikationsaussage entwertet |
| Build | `go build`, `gofmt -l` | nicht übersetzbarer oder unformatierter Stand |
| Docker Lint | hadolint | Fehler im Dockerfile |
| Actions Lint | actionlint | Fehler und Script-Injection in den Workflows |
| Docker Build | Buildx + Smoke-Test | ein Image, das zwar baut, aber unter `--read-only --cap-drop=ALL` nicht antwortet |
| Docker Security Scan | Trivy | behebbare CRITICAL/HIGH im Image |
| CodeCharta | ccsh + `scripts/codecharta-ratchet.py` | Komplexitätswachstum je Datei **und** je Funktion; neue komplexe und ungetestete Dateien |
| commitlint | commitlint | nicht-konventionelle Commits, an denen release-please die Version falsch ableitet |

Nicht bei jedem PR, sondern nach Zeitplan: Fuzzing über zehn Minuten je Ziel
(nächtlich), Mutationstests des Evaluators mit gremlins (wöchentlich und bei
Änderungen an `internal/esy/`), sowie govulncheck und ein Trivy-Scan des Images
gegen neu veröffentlichte CVEs (wöchentlich).

`make golden` bleibt bewusst draußen: es braucht R, den Upstream-Klon und rund
zehn Minuten je Modus. Es ist das **lokale** Pre-Merge-Gate oben — und das
schärfste, das dieses Projekt hat.

### Zwei Ausnahmen, die benannt gehören

Die Benchmarks messen, sie urteilen nicht: geteilte CI-Runner schwanken zu
stark für eine belastbare Schwelle pro PR. Der benchstat-Vergleich gegen den
Base-Branch landet in der Job-Zusammenfassung und ist für Menschen gedacht.

Der Container-Scan blockiert nur bei **behebbaren** Funden. Eine CVE ohne
Upstream-Fix lässt sich in einem PR nicht beheben; sie steht im Security-Tab,
statt jeden Merge zu blockieren.

## Container

```sh
make docker-build      # baut habitatus:<version> und habitatus:latest
make docker-run        # startet es gehärtet: --read-only --cap-drop=ALL
                        # --security-opt=no-new-privileges, Port 8080 gemappt
```

oder von Hand:

```sh
docker run --rm -p 8080:8080   --read-only --cap-drop=ALL --security-opt=no-new-privileges   habitatus:latest
```

Das Image enthält die vendorierte Regeldatei unter einem festen Pfad und
läuft ohne weitere Argumente. Defaults kommen aus Umgebungsvariablen
(`HABITATUS_ADDR`, `HABITATUS_RULES`, `HABITATUS_BACKBONES`,
`HABITATUS_MODE`), nicht aus `CMD` — Docker ersetzt das ganze `CMD`-Array,
sobald `docker run` irgendein Argument bekommt, und ein `CMD`, das `-addr`
und `-rules` trägt, wäre dann verschwunden. Ein Flag überschreibt die
gleichnamige Umgebungsvariable, die wiederum den eingebauten Default
überschreibt:

```sh
# beide Wege setzen faithful; beide behalten -rules/-addr aus der ENV
docker run --rm -p 8080:8080 habitatus:latest -mode faithful
docker run --rm -p 8080:8080 -e HABITATUS_MODE=faithful habitatus:latest
```

Wer eine andere Regelwerksversion mounten will, setzt `-rules` bzw.
`HABITATUS_RULES` auf den gemounteten Pfad. Der Container läuft als
`nonroot` (uid 65532) — eine gemountete Datei muss für dieses uid lesbar
sein, sonst scheitert der Start sichtbar mit einer Fehlermeldung, die die
Datei nennt.

Kein `HEALTHCHECK`: das Image hat weder Shell noch `curl`. Orchestratoren
sollen stattdessen direkt gegen `GET /health/ready` prüfen.
