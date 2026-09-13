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

Beide Kommandos laufen inzwischen auch in der CI (siehe Qualitäts-Harness);
lokal vor dem Push sind sie schneller zu haben:

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

`main` ist durch das Ruleset `protect-main` geschützt: gemergt wird nur, wenn
**alle 18** Pflicht-Checks grün sind — plus CodeQL, das über die
`code_scanning`-Regel des Rulesets greift statt über die Check-Liste.
Nicht erzwungen wird `gremlins`: der Job ist auf `internal/esy/**` gefiltert
und würde jeden PR, der diesen Pfad nicht berührt, dauerhaft auf „pending"
stehen lassen. `make quality` deckt davon die Gates ab,
die nichts außer Go brauchen — **nicht** `sbom` (braucht syft), `codecharta`
(braucht ccsh und eine JRE) und `golden` (elf Minuten). Diese drei laufen in
der CI ohnehin; lokal gezielt vor einer Änderung, die sie betrifft.

| Check | Werkzeug | Was er verhindert |
|---|---|---|
| Lint | golangci-lint (25 Linter) | die üblichen Fehlerklassen |
| Lint / depguard | golangci-lint | Verletzung der Schichtgrenzen aus der Spec — `cover` ← `taxa` ← `esy` ← `classify` ← Adapter, keine Adapter-Querkopplung |
| Test | `go test -race` + Real-File-Tests | Regression gegen die echte Regeldatei |
| Test / Coverage-Ratchet | `scripts/coverage-gate.sh` | stilles Absinken der Testabdeckung je Paket |
| Benchmarks | `go test -bench` + benchstat | Bit-Rot der Benchmarks; der Zeitvergleich ist Lesestoff, kein Gate |
| Fuzz (smoke) | `go test -fuzz`, 30 s je Ziel | Panics der Parser auf fremden Regeldateien |
| Security | govulncheck | bekannte Schwachstellen in Go-Code |
| License Compliance | go-licenses | eine Abhängigkeit unter unpassender Lizenz — erstpartei eingeschlossen, keine Ausnahmen |
| SBOM | syft (SPDX + CycloneDX), grype | fehlende Stückliste; bekannte Lücken darin |
| Secret Scan | gitleaks | Zugangsdaten in der Historie |
| CodeQL | github/codeql-action | Datenflüsse quer durch das Programm — was ein Linter, der je Funktion urteilt, nicht sehen kann |
| Architecture | `go mod tidy -diff`, Abhängigkeitsfreiheit, Regeldatei-Prüfsumme | eine unbemerkt eingeführte Abhängigkeit; eine stille Änderung der vendorierten Regeldatei, die jede Verifikationsaussage entwertet |
| Build | `go build`, `gofmt -l` | nicht übersetzbarer oder unformatierter Stand |
| Docker Lint | hadolint | Fehler im Dockerfile |
| Actions Lint | actionlint | Fehler und Script-Injection in den Workflows |
| Docker Build | Buildx + Smoke-Test | ein Image, das zwar baut, aber unter `--read-only --cap-drop=ALL` nicht antwortet |
| Docker Security Scan | Trivy | behebbare CRITICAL/HIGH im Image |
| Golden Master | `make golden` | jede Abweichung von der R-Implementierung — beide Modi, 11.337 Aufnahmen, 642.000 Ausdruckswerte |
| CodeCharta | ccsh + `scripts/codecharta-ratchet.py` | Komplexitätswachstum je Datei **und** je Funktion; neue komplexe und ungetestete Dateien |
| commitlint | commitlint | nicht-konventionelle Commits, an denen release-please die Version falsch ableitet |

Nicht bei jedem PR, sondern nach Zeitplan: Fuzzing über zehn Minuten je Ziel
(nächtlich), Mutationstests des Evaluators mit gremlins (wöchentlich und bei
Änderungen an `internal/esy/`), sowie govulncheck und ein Trivy-Scan des Images
gegen neu veröffentlichte CVEs (wöchentlich).

`make golden` läuft **mit** — als eigener Job. Er braucht nur die vendorierte
Regeldatei und die eingecheckten Fixtures, beide im Repository; R und der
Upstream-Klon werden gebraucht, um die Fixtures **neu zu erzeugen**
(`make fixtures`), nicht um gegen sie zu vergleichen. Die Parität mit der
R-Implementierung ist die zentrale Aussage dieses Projekts — sie gehört
abgesichert, nicht geglaubt. Rund elf Minuten je Lauf.

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

### Deployment auf einem Docker-Host

`compose.deploy.yaml` zieht ein veröffentlichtes Image, statt wie
`compose.yaml` aus diesem Checkout zu bauen:

```sh
docker compose -f compose.deploy.yaml up -d
docker compose -f compose.deploy.yaml logs -f
```

Es setzt voraus, dass ein `v*`-Tag existiert — erst der löst
`docker-release.yml` aus, das nach `ghcr.io/jobrunner/habitatus` veröffentlicht
(multi-arch, cosign-signiert, mit SPDX-SBOM). Solange keiner gesetzt ist, die
`image:`-Zeile durch einen `build:`-Block ersetzen; der Kommentar in der Datei
sagt, wie.

Drei Entscheidungen darin, die man kennen sollte:

- **Der Port ist an `127.0.0.1` gebunden.** Docker schreibt eigene
  iptables-Regeln; ein schlichtes `8080:8080` wäre aus dem Internet erreichbar,
  auch wenn `ufw` es verbietet. Davor gehört ein Reverse Proxy.
- **Der Modus steht explizit in der Datei**, nicht auf dem Image-Default. Er
  entscheidet, was eine Antwort bedeutet, und taucht in `versions` jeder
  Antwort auf.
- **Speichergrenze 512 MB**, gegen gemessene Werte: 36 MB nach Start, 52 MB
  nach 20 Anfragen, 80 MB nach 200 parallelen. `stop_grace_period: 15s` liegt
  über dem 10-Sekunden-Drain aus `cmd/habitatus/main.go` — bei Dockers Default
  von 10 s würde der Container mitten in einer Anfrage abgeschossen.

## Lizenz

Der Code steht unter der **MIT-Lizenz** (`LICENSE`).

Die mitgelieferten **Daten nicht**: `data/esy/EUNIS-ESy-2025-10-03.txt` ist das
EUNIS-ESy-Regelwerk unter **CC BY 4.0** (Zenodo
doi:10.5281/zenodo.3841729) — Autoren, geforderte Namensnennung und Prüfsumme
stehen in `data/esy/ATTRIBUTION.md`. Die Fixtures unter `testdata/golden/` und
`testdata/golden-repaired/` sind daraus abgeleitet und tragen dieselben
Bedingungen. Wer habitatus weitergibt, gibt beides weiter und muss die
Namensnennung mitführen.
