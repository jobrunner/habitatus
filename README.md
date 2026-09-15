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

## Die API im Gebrauch

Alle Ausgaben unten stammen aus einem Lauf von
`ghcr.io/jobrunner/habitatus:0.2.0`, nicht aus der Beschreibung.

### Bereitschaft

```sh
curl -fsS http://127.0.0.1:8080/health/ready
# {"status":"ready"}
```

Das `-f` gehört dazu: ohne es endet `curl` auch bei 5xx mit Status 0. Aus dem
Container heraus geht ebenso `docker exec <name> /habitatus -healthcheck`.

### Eine Aufnahme klassifizieren

```sh
curl -sS -X POST http://127.0.0.1:8080/api/v1/classify \
  -H 'content-type: application/json' -d '{
  "backbone": "euro+med",
  "records": [
    {"name": "Festuca ovina",        "cover": 30},
    {"name": "Potentilla argentea",  "cover": 10},
    {"name": "Dianthus deltoides",   "cover": 5},
    {"name": "Viola tricolor aggr.", "cover": 3}
  ],
  "header": {
    "Country": "Germany", "Coast_EEA": "N_COAST", "Dunes_Bohn": "N_DUNES",
    "Ecoreg": "664", "Altitude (m)": "1", "DEG_LAT": "53.06", "DEG_LON": "10.6"
  }
}'
```

Die Antwort, hier ohne den `resolution`-Block abgedruckt (er enthält einen
Eintrag je Art und ist unten für sich gezeigt):

```json
{
  "result": "R1P",
  "matches": [
    { "code": "R1P", "priority": 2 },
    { "code": "R",   "priority": 1 }
  ],
  "versions": {
    "backbone": "euro+med",
    "mode": "repaired",
    "rulepack": "EUNIS-ESy-2025-10-03.txt",
    "rulepack_sha256": "724ad8611e0ddb9e8a39e54aae840119a2219202f74a1e8b208d0a65d35d235f"
  },
  "truncated_at_10": false
}
```

Alle sieben Kopffelder sind **Pflicht** — `Country`, `Coast_EEA`,
`Dunes_Bohn`, `Ecoreg`, `Altitude (m)`, `DEG_LAT`, `DEG_LON` — und alle Werte
sind Zeichenketten, auch die Zahlen. `Country` erwartet den **englischen**
ESy-Namen (`Germany`, nicht `Deutschland`); die zulässigen Namen stehen in
`data/esy-country-names.csv`. `Coast_EEA` und `Dunes_Bohn` liefert ein
ortus-Paket (siehe `../geopackages/coast-eea`, `../geopackages/dunes-bohn`).

`mode` in `versions` ist **Teil des Ergebnisses**, nicht Beiwerk: `repaired`
und `faithful` liefern verschiedene Habitate. Wer Antworten protokolliert,
schreibt das Feld mit.

### Die beiden Fälle, die man beim Integrieren falsch versteht

Eine Aufnahme, auf die keine Regel passt — die vollständige Antwort:

```json
{
  "result": "?",
  "matches": null,
  "resolution": [
    { "input": "Fagus sylvatica", "after_backbone": "Fagus sylvatica",
      "final": "Fagus sylvatica", "resolved": true }
  ],
  "versions": { "backbone": "euro+med", "mode": "repaired",
                "rulepack": "EUNIS-ESy-2025-10-03.txt",
                "rulepack_sha256": "724ad8611e0ddb9e8a39e54aae840119a2219202f74a1e8b208d0a65d35d235f" },
  "truncated_at_10": false
}
```

**`"?"` heißt „keine Regel trifft", nicht „Fehler"** — eine korrekte Antwort
auf eine Aufnahme, für die ESy kein Habitat kennt. Beachte `"matches": null`:
das Feld verschwindet nicht, es ist `null`. Ein Client, der auf Anwesenheit
statt auf den Wert prüft, läuft hier in eine Null-Referenz.

**Ein unbekannter Name ergibt ebenfalls `"?"`** — und das ist der gefährlichere
Fall, weil er wie das obige aussieht. Nur der `resolution`-Block sagt es:

```json
"resolution": [
  { "input": "Quatschus erfundus", "after_backbone": "Quatschus erfundus",
    "final": "Quatschus erfundus", "resolved": false }
]
```

Wer wissen will, ob die eigene Nomenklatur zum Regelwerk passt, prüft dort auf
`resolved: false`. Das Ergebnisfeld allein verrät es nicht.

### Zurückgewiesene Anfragen

```
HTTP 400  {"error":"Country \"Deutschland\" is not an ESy country name; see data/esy-country-names.csv"}
HTTP 400  {"error":"cover for \"Festuca ovina\" is 0, must be in (0, 100]"}
HTTP 400  {"error":"unknown backbone \"unbekannt\""}
```

Ungültige Kopfwerte werden **abgelehnt statt als unbekannt behandelt**. Ein
falsch geschriebenes Land würde sonst stillschweigend jede Regel unterdrücken,
die es prüft — ohne Fehler und mit plausibel aussehendem Ergebnis.

### Metriken

```sh
curl -fsS http://127.0.0.1:8080/metrics
# {"total":0,"question":0,"plus":0,"question_share":0,"plus_share":0,
#  "unreachable_rules":[],"never_fired_rules":["MA","MA211", … 312 Einträge]}
```

`question_share` ist der Anteil der Aufnahmen ohne Zuordnung — die Zahl, an der
man sieht, ob die eigenen Daten zum Regelwerk passen. `unreachable_rules` ist
in `repaired` leer und listet in `faithful` die 100 Regeln, die dort nie feuern
können.

### Browser-Zugriff prüfen

```sh
docker run -d --name habitatus -p 127.0.0.1:8080:8080 \
  --read-only --cap-drop=ALL --security-opt=no-new-privileges \
  -e HABITATUS_CORS='https://app.example' \
  ghcr.io/jobrunner/habitatus:0.2.0

curl -sS -i -X OPTIONS http://127.0.0.1:8080/api/v1/classify \
  -H 'Origin: https://app.example' \
  -H 'Access-Control-Request-Method: POST' \
  -H 'Access-Control-Request-Headers: content-type'
# HTTP/1.1 204 No Content
# Access-Control-Allow-Origin: https://app.example
```

**Der Preflight ist der aussagekräftige Test, nicht der POST.**
`application/json` löst ihn immer aus, und ohne CORS beantwortet der Mux ihn
mit 405 — der Browser sendet die eigentliche Anfrage dann nie ab. Ein `curl`
auf den POST wäre trotzdem erfolgreich und würde nichts beweisen.

### Vom Docker-Host aus

Der Port hängt bewusst auf `127.0.0.1` (siehe Deployment weiter unten). Auf dem
Host selbst funktionieren alle Aufrufe oben unverändert; von außen führt der
Weg über den Reverse Proxy. Zum Prüfen ohne Proxy:

```sh
ssh -L 8080:127.0.0.1:8080 user@host
```

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

Lokal vor dem Push: `make quality` — es enthält seit zwei Fehlschlägen auch
`make commitlint`, weil ein Akronym am Satzanfang („CORS, off by default",
„NaN and a malformed map") sich einwandfrei liest, gegen `subject-case`
verstößt und erst nach dem Push auffällt. Beide Male kostete das eine
Umschreibung der Historie.

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
| Fuzz (smoke) | `go test -fuzz`, 1.000.000 Ausführungen je Ziel | Panics der Parser auf fremden Regeldateien |
| Security | govulncheck | bekannte Schwachstellen in Go-Code |
| License Compliance | go-licenses | eine Abhängigkeit unter unpassender Lizenz — erstpartei eingeschlossen, keine Ausnahmen |
| SBOM | syft (SPDX + CycloneDX), grype | fehlende Stückliste; bekannte Lücken darin |
| Secret Scan | gitleaks | Zugangsdaten in der Historie |
| CodeQL | github/codeql-action | Datenflüsse quer durch das Programm — was ein Linter, der je Funktion urteilt, nicht sehen kann |
| Architecture | `go mod tidy -diff`, Abhängigkeitsfreiheit, Regeldatei-Prüfsumme, Ratchet-Basisvergleich | eine unbemerkt eingeführte Abhängigkeit; eine stille Änderung der vendorierten Regeldatei, die jede Verifikationsaussage entwertet; ein im selben PR abgesenkter Floor oder angehobener Komplexitäts-Cap |
| Build | `go build`, `gofmt -l` | nicht übersetzbarer oder unformatierter Stand |
| Docker Lint | hadolint | Fehler im Dockerfile |
| Actions Lint | actionlint (+ shellcheck) | Fehler und Script-Injection in den Workflows |
| CodeQL / Actions | `actions/unpinned-tag` | eine Action, die per beweglichem Tag statt per Commit-SHA eingebunden ist |
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
docker run --rm -p 127.0.0.1:8080:8080   --read-only --cap-drop=ALL --security-opt=no-new-privileges   habitatus:latest
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
docker run --rm -p 127.0.0.1:8080:8080 habitatus:latest -mode faithful
docker run --rm -p 127.0.0.1:8080:8080 -e HABITATUS_MODE=faithful habitatus:latest
```

Wer eine andere Regelwerksversion mounten will, setzt `-rules` bzw.
`HABITATUS_RULES` auf den gemounteten Pfad. Der eingebaute Default ist ebenfalls `127.0.0.1:8080` — wer das Binary ohne
Container startet, veröffentlicht nichts. Das Image überschreibt das mit
`HABITATUS_ADDR=:8080`, weil der Port dort nur über ein explizites `-p`
erreichbar ist.

Alle Beispiele binden bewusst an `127.0.0.1`: Docker schreibt eigene
iptables-Regeln, und ein schlichtes `8080:8080` veröffentlicht den
unauthentifizierten Dienst auf allen Interfaces — auch wenn `ufw` es verbietet.
Die Härtungsflags schützen den Prozess, nicht den Netzzugang, und CORS ist keine
Zugriffskontrolle. Der Container läuft als
`nonroot` (uid 65532) — eine gemountete Datei muss für dieses uid lesbar
sein, sonst scheitert der Start sichtbar mit einer Fehlermeldung, die die
Datei nennt.

### Healthcheck

Das Image bringt einen mit. Weil es distroless ist — keine Shell, kein `curl`,
und Docker führt eine Probe ausschließlich **im** Container aus —, prüft das
Binary sich selbst:

```yaml
healthcheck:
  test: ["CMD", "/habitatus", "-healthcheck"]
  interval: 30s
  timeout: 5s
  start_period: 30s
  retries: 3
```

`/habitatus -healthcheck` fragt `GET /health/ready` auf der konfigurierten
`-addr` ab und endet mit 0 oder 1. `CMD` in Exec-Form, nicht als Zeichenkette:
es gibt keine Shell, die sie zerlegen könnte. Die Regeldatei wird dabei
**nicht** geladen — die Probe fragt einen laufenden Server, und ein 8 MB großes
Regelwerk bei jeder Prüfung zu parsen würde jeden Healthcheck so teuer machen
wie einen Start.

Wer die Lauschadresse überschreibt, muss dafür **`HABITATUS_ADDR` nehmen, nicht
ein `-addr`-Argument**: Docker führt das Health-Kommando als eigenen Prozess
aus und reicht ihm die Argumente des Containers nicht weiter. `docker run
habitatus -addr :9090` würde also auf 9090 lauschen, während die Probe
weiterhin `:8080` fragt — und der Container gälte dauerhaft als ungesund. Die
Umgebungsvariable sehen beide Prozesse, das Argument nur einer.

Dasselbe gilt für **`-mcp`**: dieser Modus spricht JSON-RPC über stdio und
startet keinen HTTP-Server, es gibt also nichts zu proben. Mit `HABITATUS_MCP`
gesetzt erkennt die Probe das und endet erfolgreich. Als **Argument** übergeben
kann sie es nicht sehen — dann den Healthcheck abschalten
(`--no-healthcheck`, in Compose `healthcheck: disable: true`), sonst gilt ein
einwandfrei arbeitender stdio-Prozess dauerhaft als ungesund.

`start_period` deckt genau diesen Start ab; Fehlschläge in diesem Fenster
zählen nicht gegen `retries`. Ist `-addr` auf `:8080` oder `0.0.0.0:8080`
gesetzt, fragt die Probe `127.0.0.1` — für einen Lauscher heißt das „alle
Interfaces", für einen Aufrufer nichts.

Zwei Dinge, die man dazu wissen sollte: ein einfacher Docker-Host **startet
einen ungesunden Container nicht neu** — `restart: unless-stopped` reagiert
darauf, dass der Prozess endet. Der Gesundheitszustand ist das, was `docker ps`
anzeigt, worauf `depends_on: condition: service_healthy` wartet und was ein
Monitor auslesen kann. Und von außen bleibt `GET /health/ready` unverändert
erreichbar.

### CORS

Standardmäßig **aus**. Eingeschaltet wird sie mit einer Liste erlaubter
Herkünfte:

```sh
docker run ... -e HABITATUS_CORS='https://app.example, http://localhost:5173'
docker run ... habitatus:latest -cors '*'      # jede Herkunft
```

Ohne diese Angabe ist die API **aus einem Browser heraus nicht erreichbar** —
nicht eingeschränkt, sondern gar nicht: `POST /api/v1/classify` nimmt
`application/json`, und das ist kein CORS-einfacher Content-Type, also schickt
jeder Browser zuerst einen `OPTIONS`-Preflight. Ohne CORS beantwortet der Mux
den mit `405`, und der Browser sendet die eigentliche Anfrage nie. Ein Client
serverseitig (curl, Go, R) ist davon nicht betroffen.

Warum trotzdem aus als Default: der Dienst bindet an `127.0.0.1` und erwartet
einen Reverse Proxy davor. Ein permissiver Default würde bei einer internen
Installation jeder beliebigen Webseite, die ein Nutzer öffnet, den Zugriff
darauf erlauben. Die Entscheidung gehört dem Betreiber — sie ist eine
Umgebungsvariable weit.

Gesetzt werden `Access-Control-Allow-Origin` (die Herkunft des Aufrufers,
nicht die Liste), `Vary: Origin` — damit ein Cache davor nicht die Antwort für
eine Herkunft an eine andere ausliefert — und beim Preflight zusätzlich
`Allow-Methods`, `Allow-Headers` und `Max-Age`. **Kein**
`Allow-Credentials`: der Dienst hat weder Sitzungen noch Authentifizierung,
also gibt es keine Rechte, die ein Browser mittragen könnte.

Eine unbrauchbare Angabe (`a.example` ohne Schema, `*` mit benannten Herkünften
gemischt) stoppt den Start mit einer Meldung, die den Wert nennt, statt mit
einer halb konfigurierten CORS weiterzulaufen.

### Deployment auf einem Docker-Host

`docker-compose.deploy.yml` zieht ein veröffentlichtes Image, statt wie
`docker-compose.yml` aus diesem Checkout zu bauen:

```sh
docker compose -f docker-compose.deploy.yml up -d
docker compose -f docker-compose.deploy.yml logs -f
```

Ein `v*`-Tag löst `docker-release.yml` aus, das nach
`ghcr.io/jobrunner/habitatus` veröffentlicht. Am Release `v0.1.0` nachgemessen,
nicht behauptet:

```
Index  sha256:d3296548495135a5f3eadedd6b5ae2b394f62a31e278fa141b982568dc643bdf
       linux/amd64  sha256:7d3534d691fba6280242e850ec9b192901c70667e23b3f4678ced27e22a498ae
       linux/arm64  sha256:3fa06fc2e819fef109b249baa6394a08e1ec95da4632845106398655866ecc04
```

Beide Attestierungen — SPDX-SBOM und SLSA-Provenance — nennen als Subjekt den
**Index-Digest**, nicht die Architektur-Kandidaten. Das ist der Punkt, an dem
es schiefgeht, wenn man es nicht beachtet: `imagetools create` erzeugt einen
neuen Index mit neuem Digest, und Nachweise folgen keinem Digest, für den sie
nicht ausgestellt wurden.

Prüfen — über den **unveränderlichen Digest**, nicht über den Tag, der
umgehängt werden kann:

```sh
gh attestation verify \
  oci://ghcr.io/jobrunner/habitatus@sha256:d3296548495135a5f3eadedd6b5ae2b394f62a31e278fa141b982568dc643bdf \
  -R jobrunner/habitatus --format json
```

Die Provenance bindet das Image an den Bauvorgang: Workflow-Datei und Tag
(`docker-release.yml@refs/tags/v0.1.0`), Quell-Commit
`a80d656184d36a987ca3476e7fda96ee5746d0e4` — derselbe, den das Binary als
`commit` meldet — und `runnerEnvironment: github-hosted`, gegengezeichnet im
Rekor-Transparenzlog.

Ein Hinweis zur Bedienung: ohne Terminal gibt `gh attestation verify` bei
Erfolg nichts aus und endet mit 0. Dass das keine leere Zustimmung ist, zeigt
die Gegenprobe — mit `-R jobrunner/ortus` endet derselbe Aufruf mit 1 und
`failed to fetch attestations`.

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
