# Habitatus — Design

**Datum:** 2026-09-10
**Status:** abgestimmt, Implementierung noch nicht begonnen

---

## 1. Zweck und Abgrenzung

Habitatus ist ein Go-Dienst, der einer Vegetationsaufnahme ein EUNIS-Habitat
zuordnet. Eingabe ist eine Liste von Pflanzennamen mit Deckungsgraden in Prozent,
die Angabe des Nomenklatur-Backbones, aus dem die Namen stammen, und acht
Kopfdaten zum Standort. Ausgabe sind alle zutreffenden Habitate mit ihrer
Priorität, der daraus abgeleitete Gewinner und ein Bericht darüber, wie die Namen
aufgelöst wurden.

Habitatus **portiert das Verhalten** des ESy-Expertensystems in der gepflegten
R-Implementierung. Es erfindet keine Regeln und interpretiert keine nach. Wo das
Original Eigenheiten oder Fehler hat, werden sie nachgebildet und dokumentiert,
nicht stillschweigend korrigiert.

### Nicht Bestandteil

Diese drei Schritte liegen beim Aufrufer und werden hier bewusst nicht
implementiert:

- **Ableitung der Kopfdaten aus einer Koordinate.** Ecoregion, Land,
  Küstenkategorie, Dünenlage und Höhe besorgt der Client, in unserem Umfeld über
  ortus. Habitatus bekommt fertige Werte.
- **Auswahl der Quellnomenklatur.** Der Client weiß, aus welchem Namensvorrat
  seine Namen stammen — im Regelfall EuroSL/Euro+Med, das die gesamte
  Westpaläarktis abdeckt — und teilt das mit.
- **Umrechnung der Deckungsskala.** Braun-Blanquet oder andere Skalen rechnet der
  Client in Prozent um.

Damit ist Habitatus eine reine Funktion: gleiche Eingabe, gleiche Ausgabe, ohne
Netzwerk, Uhr oder Zufallsquelle.

### Referenzmaterial

| Was | Wo | Rolle |
|---|---|---|
| Regelwerk `EUNIS-ESy-2025-10-03.txt` | Zenodo doi:10.5281/zenodo.3841729 | die Regeln |
| Nomenklatur-Übersetzungen (48 Dateien) | ebenda | Backbone-Tabellen |
| R-Implementierung v1.2 | `git.loe.auf.uni-rostock.de/misc/ESy` | **Verhaltensreferenz** |
| Testdatensatz Tüxen-Archiv | ebenda, `data/` | 10.717 Aufnahmen |
| Bruelheide et al. 2021, AVS 24:e12562 | doi:10.1111/avsc.12562 | Semantik der Operatoren |
| Chytrý et al. 2020, AVS 23:648–675 | doi:10.1111/avsc.12519 | fachlicher Hintergrund |

Maßgeblich ist die **R-Implementierung in Version 1.2** (Stand 2025-02-03, geklont
als `416bab9`), nicht das Paper von 2021 und nicht das ältere Einzelskript
`Expert_system_file_in_R12.R` von 2019. Die drei unterscheiden sich im Verhalten;
siehe §7.

---

## 2. Architektur

```
   ┌── vorgelagert, nicht Teil von Habitatus ───────────────────┐
   │  Kopfdaten aus der Koordinate (ortus)                      │
   │  Backbone-Auswahl                                          │
   │  Deckungsskala → Prozent                                   │
   └────────────────────────┬───────────────────────────────────┘
                            ▼
                    ┌──────────────┐
  REST / MCP  ──▶   │   classify   │  Anwendungsfall
                    └──────┬───────┘
                  ┌────────┴────────┐
                  ▼                 ▼
             ┌────────┐        ┌────────┐
             │  taxa  │        │  esy   │ ◀── rulepack
             └────────┘        └────────┘
```

**`rulepack`** parst das ESy-Dateiformat in ein unveränderliches Regelwerk-Objekt.
Weil die 48 Nomenklaturdateien dasselbe Format haben (nur Sektion 1 gefüllt),
bedient derselbe Parser auch die Backbone-Tabellen.

**`taxa`** bildet Rohnamen über die Backbone-Tabelle und anschließend über
Sektion 1 auf ESy-Zielkonzepte ab und verschmilzt dabei Deckungen.

**`esy`** wertet das Regelwerk aus und liefert Treffer, Gewinner und
die Zwischenwerte jeder Bedingung. Rein, ohne Seiteneffekte.

**`classify`** orchestriert, validiert und setzt die Antwort zusammen.

Schnittstellen sind **HTTP REST und MCP**. Ein CLI wird nicht ausgeliefert; der
Batch-Pfad für den Golden Master ist ein Test-Harness (§6).

---

## 3. Eingabe

### 3.1 Artenliste

Je Eintrag ein Name und eine Deckung in Prozent, `0 < c ≤ 100`. Was genau ein gültiger
Name ist, steht unmittelbar darunter — es ist die schwierigste Anforderung der
ganzen Schnittstelle.

#### Was ein Artname erfüllen muss

**Der Abgleich ist ein exakter Zeichenkettenvergleich.** Der Upstream tut in
`step4_aggregate-taxon-levels.R` genau dies:

```r
index1 <- match(obs$TaxonName, AGG$values)
obs$TaxonName[!is.na(index1)] <- AGG$ind[index1[!is.na(index1)]]
```

Keine Normalisierung, keine Toleranz bei Groß-/Kleinschreibung, keine
Fuzzy-Suche, **ein einziger Durchlauf**. Daraus folgt zweierlei, was die
Entscheidungen in §8 von „konservativ gewählt" auf **belegt** hebt:

- `match()` liefert den **ersten** Treffer → „erster Eintrag gewinnt" ist
  Upstream-Verhalten.
- Ein Durchlauf → **keine transitive Auflösung** von Ketten.

Die „Herkunft" im Request bezeichnet deshalb die **Namens-Zeichenkettenquelle**,
nicht ein taxonomisches Konzept im Sinne einer Umgrenzung. Sie wählt aus, welche
Übersetzungstabelle vor Sektion 1 läuft.

#### EuroSL als Regelfall

EuroSL (F. Jansen, Univ. Rostock) ist eine flache Fassung der Euro+Med
PlantBase — also des Systems, das ESy als Zielnomenklatur erwartet. EuroSL-Daten
werden daher als `euro+med` deklariert und brauchen **keine**
Übersetzungstabelle; Sektion 1 erledigt den Rest.

Gemessen an den **8.477 Namen, die in Sektion-2-Gruppen vorkommen** — nur diese
beeinflussen ein Ergebnis, nicht alle 20.159 Zielkonzepte:

| | Anzahl | Anteil |
|---|---|---|
| EuroSL kennt sie als akzeptierten Namen | 7.941 | 93 % |
| EuroSL unbekannt | 528 | 6 % |

Die Lücke ist systematisch:

| Art der Lücke | Anzahl | Konsequenz |
|---|---|---|
| Flechten, Algen, Pilze, Hybriden, makaronesische Endemiten | 293 | außerhalb des EuroSL-Umfangs; der Upstream liefert `data/Various-names-bryo-lich-algae-fungi.xlsx` dafür |
| ESy-Aggregate (`aggr.`) | 145 | ESy führt eigene Aggregate; von 175 kennt EuroSL nur 18 |
| `<Gattung> species` | 90 | ESy-Konvention für Gattungsfunde; EuroSL kennt keinen davon |

#### Anforderungen an den Client

1. Auf den **akzeptierten** EuroSL-Namen auflösen. Synonyme greifen nur, wenn
   Sektion 1 sie führt. 7.706 akzeptierte EuroSL-Namen werden dort ohnehin weiter
   aggregiert — das ist der Zweck von Sektion 1, kein Fehler.
2. **EuroSL-Zähler abschneiden**: `Festuca ovina.1` → `Festuca ovina`. Ohne das
   trifft der Name nichts.
3. Gattungsfunde in ESy-Schreibweise: `Quercus species`, nicht `Quercus`.
4. Aggregate in ESy-Schreibweise, nicht in der von EuroSL.

#### Unauflösbare Namen sind nicht wirkungslos

Ein Name, der auf kein Zielkonzept fällt, gehört zu keiner Gruppe — **zählt aber
weiterhin in die Gesamtdeckung**. Er geht ein in `#T$` (Deckung ohne die
verglichene Gruppe), in `#$$` (höchste Deckung irgendeiner Art) und in alle
`NON`-Bedingungen. Ein Tippfehler verschiebt damit Nenner und kann Dominanztests
kippen.

Das ist der Grund, warum die Antwort einen Auflösungsbericht führt (§6): Ohne ihn
bliebe unsichtbar, dass ein Name stillschweigend durchgefallen ist, obwohl er das
Ergebnis beeinflusst hat.

### 3.2 Quellnomenklatur (Backbone)

Pflichtangabe. Zulässig sind `euro+med` (Identität, keine Tabelle nötig) und die
Kennungen der 48 mitgelieferten Übersetzungstabellen. Ein weiterer Backbone ist
eine Datei im selben Format, kein Code.

#### Verfügbare Tabellen

Die 48 Tabellen stammen aus dem Zenodo-Record
(`Nomenclature-translation-from-Turboveg-2-databases`) und bilden zusammen
**450.099 Quellnamen auf 254.952 Ziele** ab. Sie sind nach **Turboveg-2-Datenbanken**
benannt, nicht nach taxonomischen Backbones — `GermanSL 1.4` ist eine echte
Referenzliste, `Classify` oder `Sps_Iva` sind Projektdatenbanken. Der
Request-Parameter heißt deshalb sachlich „Quellnomenklatur", nicht „taxonomischer
Backbone" im Sinne von GBIF.

Die größten, nach Quellnamen:

| Tabelle | Ziele | Quellnamen |
|---|---|---|
| `Classify` | 15.621 | 31.405 |
| `Europe_Lenoir` | 15.636 | 31.380 |
| `Classify_Halophyt` | 15.576 | 31.305 |
| `Europe_Pinus` | 15.695 | 31.061 |
| `Europe_weed` | 15.352 | 30.702 |
| `Sps_Iva` | 15.386 | 30.673 |
| `Europe` | 15.326 | 30.634 |
| `Europe_EDGG` | 15.265 | 30.540 |
| `GermanSL 1.3 GrassVeg.DE` | 9.418 | 20.170 |
| `Vegitaly` | 9.965 | 20.078 |
| `GermanSL 1.4` | 9.082 | 19.289 |

Die übrigen 37 sind Länder- und Regionaltabellen (Austria, Balkan, Britain,
Bulgaria, C_Europe und Varianten, Cyprus, Czechia_Slovakia_2015, Euskadi,
Floranld_2013, France_Sophy, Greece, Greece_Crete, Ireland2008, Italy,
Italy_Conti, Latvia, Lithuania, Natura, Poland, Poland_forest, Portugal_Estrela,
Romania, Romania_Indreica, Russia und vier Varianten, South_Slavic, Spain_sivim,
Switzerland, Turkey, Ukraine_Kiev, Vegitaly_HMMD) zwischen 317 und 11.146
Quellnamen.

**Nicht verfügbar** und damit vorerst nicht bedienbar: WCVP, Ehrendorfer, die
GBIF-Backbone und World Flora Online. Für sie müsste eine Tabelle im selben
Format erst erstellt werden — dieselbe Schwierigkeit wie zuvor, nur an anderer
Stelle.

#### Defektdichte in den Backbone-Tabellen

Die Übersetzungstabellen sind **deutlich fehlerhafter als die Haupt-Sektion 1**.
Gemessen an `GermanSL 1.4` (19.259 Quellnamen):

| | Haupt-Sektion 1 | GermanSL 1.4 | GermanSL 1.3 GrassVeg.DE |
|---|---|---|---|
| mehrdeutige Quellnamen | 7 von 100.703 | **26** von 19.259 | 36 von 20.128 |
| Ketten (Name ist Quelle und Ziel) | 2 | **51** | 63 |

Ein Beispiel: `Atriplex prostrata gr.` steht unter fünf verschiedenen Zielen —
`Atriplex prostrata aggr.` sowie vier Hybridkombinationen. `Bacidia
hegetschweileri` steht unter `Bacidia subincompta` und `Bacidia vermifera`.

Die Regeln aus §8 — erster Eintrag gewinnt, keine transitive Auflösung — gelten
unverändert, greifen hier aber viel häufiger. Beim Laden eines Backbones werden
die Zähler protokolliert (§9), damit sichtbar ist, wie stark eine Tabelle
betroffen ist.

**Parser-Detail:** In diesen Dateien stehen **keine Leerzeilen zwischen den
Blöcken** — anders als in der Haupt-Regelwerksdatei folgt auf die letzte
eingerückte Zeile direkt der nächste Blockkopf. Der Parser muss Blöcke an der
Einrückung erkennen, nicht an Trennzeilen.

### 3.3 Kopfdaten

Alle Felder sind Pflicht, `Dataset` ausgenommen.

| Feld | Typ | Wertebereich | Formeln, die es prüfen |
|---|---|---|---|
| `DEG_LAT` | Zahl | WGS 84, Dezimalgrad | 84 |
| `Coast_EEA` | Text | `ARC_COAST`, `ATL_COAST`, `BAL_COAST`, `BLA_COAST`, `MED_COAST`, `N_COAST` | 76 |
| `DEG_LON` | Zahl | WGS 84, Dezimalgrad, West negativ | 53 |
| `Country` | Text | englischer Ländername aus der ESy-Liste, siehe unten | 41 |
| `Ecoreg` | Ganzzahl | `ECO_ID` aus Ecoregions 2017 | 37 |
| `Altitude (m)` | Zahl | Meter über NN | 24 |
| `Dunes_Bohn` | Text | `Y_DUNES`, `N_DUNES` | 11 |
| `Dataset` | Text | optional | 1 |

204 der 314 Formelzeilen (65%) prüfen mindestens ein Kopfdatum; 123 davon (39%
des Regelwerks) hängen an Ecoregion, Koordinate oder Höhe. Deshalb sind Koordinate
und Höhe Pflicht und nicht optional.

#### `Country`

Erwartet wird der **englische Ländername in genau der Schreibweise, die ESy
verwendet** — kein ISO-Code, kein landessprachlicher Name, keine aktuelle
amtliche Bezeichnung. `Germany`, nicht `Deutschland`, nicht `DE`. Die
Vergleichsoperation im Regelwerk ist ein exakter Zeichenkettenvergleich
(`<$$C Country EQ Germany>`); jede Abweichung führt dazu, dass die Bedingung
stumm nie wahr wird.

Das Vokabular umfasst **52 Namen**, festgelegt im User-Guide (Appendix S5 zu
Chytrý et al. 2020). Mehrere davon sind historische oder Langformen:

| ESy verlangt | nicht |
|---|---|
| `Czech Republic` | Czechia |
| `Slovak Republic` | Slovakia |
| `Russian Federation` | Russia |
| `Turkey` | Türkiye |
| `Bosnia-Herzegovina` | Bosnia and Herzegovina |
| `Svalbard and Jan Mayen Is` | Svalbard and Jan Mayen Islands |

Die vollständige Zuordnung **ISO 3166-1 alpha-2 → ESy-Name** liegt als
`data/esy-country-names.csv` im Repo (52 Zeilen, gegen den User-Guide geprüft).
ISO ist der empfohlene Übergabeweg für Clients, weil er sprachunabhängig und
stabil ist; ein landessprachlicher Name ist es nicht.

Von den 52 Namen werden **22 tatsächlich von Regeln geprüft**. Die übrigen sind
gültige Eingaben ohne Wirkung auf das Ergebnis. Die Validierung akzeptiert
deshalb alle 52 — ein Plot in Schweden ist kein Fehler, nur weil keine Regel nach
Schweden fragt.

**Bekannter Defekt: `Britain` neben `United Kingdom`.** Das Regelwerk 2025
verwendet beide Werte, nie in derselben Formel: `United Kingdom` in 11 Q-, R- und
T-Regeln, `Britain` in 9 U-Regeln (Schutthalden und Felsen). `Britain` steht
nicht im User-Guide-Vokabular.

Die Wirkung ist asymmetrisch. Alle `Britain`-Vorkommen stehen in positiven
ODER-Ketten; alle Ausschlüsse (`NOT (… OR <$$C Country EQ United Kingdom>)`)
nutzen `United Kingdom`. Mit `United Kingdom` als Eingabe feuern die neun
U-Regeln nicht — **fehlende** Treffer. Mit `Britain` greifen die Ausschlüsse
nicht — **falsche** Treffer.

**Entscheidung: `GB` wird auf `United Kingdom` abgebildet**, dem
User-Guide-Vokabular folgend und weil fehlende Treffer dem stillen Falschtreffer
vorzuziehen sind. Britische Plots erhalten damit für die neun U-Regeln kein
Ergebnis. Der Punkt ist in der Notiz an die Autoren aufgeführt.

---


## 4. Datenfluss

**1. Validierung.** Kopfdatenwerte gegen ihr Vokabular, Deckungen gegen ihren
Bereich. Verstöße werden abgewiesen. Artnamen werden **nicht** validiert.

Diese Asymmetrie ist beabsichtigt: Ein falsch geschriebener Ländername verfälscht
das Ergebnis systematisch und still, ein unbekannter Artname nur graduell — und
Feldlisten enthalten immer Namen, die keine Tabelle kennt.

**2. Backbone-Übersetzung.** Namen durch die Tabelle des angegebenen Backbones.
Namen ohne Eintrag bleiben unverändert; sie könnten bereits Euro+Med sein.

**3. Aggregation nach Sektion 1.** Abbildung auf die 20.159 Zielkonzepte.

Sektion 1 ist mehr als Synonymauflösung: Ränge werden eingezogen und eigenständige
Arten zu Aggregaten verschmolzen. `Empetrum nigrum` und `E. hermaphroditum` werden
beide zu `Empetrum nigrum aggr.` — und nur unter diesem Namen fragt Regel `N18`
sie ab. Ohne diesen Schritt könnten solche Regeln nie feuern.

Nach **jeder** der beiden Stufen werden Konzepte zusammengeführt und Deckungen
nach Jennings-Fischer verschmolzen (§5.2).

**4. Auswertung.** Von innen nach außen: Bedingungen → Ausdrücke →
Formeln. Alle Regeln werden bewertet.

**5. Gewinner.** Nach der Logik von v1.2 (§5.4).

---

## 5. Semantik

### 5.1 Bedingungstypen

| Präfix | Bedeutung |
|---|---|
| `###`, `##D` | Artenzahl der Gruppe (`##D` kommt im EUNIS-Regelwerk nicht vor, wird vom Upstream aber gleichwertig behandelt) |
| `##C` | Summe der Deckungen |
| `##Q` | Summe der Wurzeln der Deckungen |
| `#TC` | Gesamtdeckung der Gruppe (Jennings-Fischer) |
| `#NN` (`#01`…`#05`) | gemeint ist „Artenzahl der Gruppe ≥ NN"; im Upstream **immer FALSE**, siehe §8 |
| `#SC` | Deckung einer Einzelart der Gruppe |
| `#T$` | Gesamtdeckung **ohne** die Arten der verglichenen Gruppe |
| `#$$` | höchste Deckung irgendeiner Art im Plot, **einschließlich** der verglichenen Gruppe; nur die Form `#$$ EXCEPT <Gruppe>` schließt jene aus |
| `$NN` (`$25`, `$50`) | Prozentsatz der Gesamtdeckung |
| `NON` | dasselbe Maß über die **übrigen Gruppen derselben `+NN`-Menge**, die Gruppe selbst ausgenommen — nicht über die Arten außerhalb der Gruppe |
| `$$C`, `$$N` | Kopfdaten, kategorial bzw. numerisch |

Der `+NN`-Qualifier (`+01`…`+12`, ohne `+05`) benennt eine Vergleichsmenge von
Gruppen; `GR NON <dieselbe Gruppe>` heißt „größer als jede andere Gruppe dieser
Menge". Die Einschränkung auf dieselbe Menge steht im Upstream an einer Stelle
für beide Fälle — den operatorlosen Ausdruck und das `NON`-Atom
(`step3and5…R:361-380`); nachgebildet in `esy.bestOfComparisonSet`.

### 5.2 Deckungsverschmelzung

Deckung ist projizierte Fläche, nicht Menge. Fallen mehrere Namen auf dasselbe
Konzept, werden ihre Deckungen unter Annahme zufälliger Überlagerung vereinigt:

```
total_cover(x) = round((1 - Π(1 - xᵢ/100)) · 100, 10)
```

Die Rundung auf zehn Nachkommastellen ist kein Schönheitsdetail, sondern
Verhaltensbestandteil — der Upstream hat sie in `416bab9` eigens eingeführt, um
Gleitkomma-Grenzfälle zu stabilisieren.

Die Formel ist kommutativ; die Artenreihenfolge des Originals muss nicht
nachgebildet werden. Dieselbe Formel gilt überall dort, wo `#TC` eine
Gruppendeckung berechnet.

Wirkung an der Schwelle, Beispiel `<#TC Trees GR 25>` bei 10 %, 9 %, 8 %:

| Rechnung | Ergebnis | Regel erfüllt |
|---|---|---|
| Addition | 27 % | ja |
| Jennings-Fischer | 24,65 % | **nein** |

### 5.3 Logik

`AND` → `&`, `OR` → `|`, `NOT` → `&!`, ausgewertet mit **R-Präzedenz**: `!` vor
`&` vor `|`, `NOT` binär. Ein Parser mit gleichrangigen linksassoziativen
Operatoren läge bei jeder unparenthesierten Kombination falsch.

**Korrektur gegenüber der ursprünglichen Fassung dieser Spec:** Hier stand
dreiwertige Kleene-Logik mit Fortpflanzung von „unbekannt". Das beschreibt das
Einzelskript von 2019, **nicht** die portierte Version 1.2. Diese zwingt vor
jeder logischen Auswertung alle unauflösbaren Werte auf null:

```r
if(any(is.na(plot.cond))) warning('NA in plot.cond')
plot.cond[is.na(plot.cond)] <- 0   # step3and5…R:450-452, ebenso prep.R:62-63
plot.cond[plot.cond == -Inf] <- 0
```

Es gibt in v1.2 also **keine** Fortpflanzung von Unbekanntheit. Ein fehlendes
Kopfdatum, eine nicht auflösbare Gruppe und ein `max()` über die leere Menge
ergeben alle **0**; der Vergleich liefert danach ganz gewöhnlich wahr oder
falsch. Die Auswertung ist damit zweiwertig.

Der dreiwertige Typ bleibt im Code erhalten und getestet — er ist korrekt und
stünde bereit, falls ein künftiges Regelwerk oder eine spätere Upstream-Version
`NA` wieder durchreicht —, aber der Bedingungslayer erzeugt kein „unbekannt".

Das ändert nichts an der Zusicherung aus §3: Wer ein Kopfdatum nicht liefert,
bekommt kein Ergebnis für die davon abhängigen Regeln. Nur der Weg dorthin ist
ein anderer — die Bedingung wird falsch, nicht unbekannt.

### 5.4 Gewinnerermittlung (v1.2)

```
kein Treffer                     → "?"
genau ein Treffer                → dieser
sonst: Prioritätsstufen absteigend durchgehen;
       erste Stufe mit genau einem Treffer → dieser
       keine solche Stufe                  → "+"
```

Prioritäten laufen von 1 bis 8, höher gewinnt. Verteilung im Regelwerk 2025:
Stufe 4 mit 141 Regeln und Stufe 2 mit 91 dominieren.

Der Abstieg auf die nächstniedrigere Stufe ist die Neuerung von v1.2. Ältere
Fassungen lieferten sofort `+`.

---

## 6. Antwort

- `result` — EUNIS-Code, `?` oder `+`
- `matches` — **alle** Treffer mit Code, Priorität und Regelvariante (`N15`,
  `N15!`, `N15!!`)
- `resolution` — je Eingabename: Zielkonzept, über welche Stufe, oder unaufgelöst
- `versions` — Regelwerk und Backbone-Tabelle

Zwei bewusste Erweiterungen gegenüber dem Original, beide additiv: Das Original
schneidet die Trefferliste bei zehn ab — wir liefern alle und vermerken, wo
abgeschnitten worden wäre, damit der Golden Master vergleichbar bleibt. Und einen
Auflösungsbericht gibt es im Original nicht; ohne ihn bliebe unsichtbar, dass ein
Name stillschweigend durchgefallen ist.

---

## 7. Verifikation

### Referenz

Der Upstream wird **getrieben, nicht geforkt**: `spike/ESy-upstream/` enthält den
Klon (per `.gitignore` ausgeschlossen), `spike/resy/generate-fixtures.R` lädt
dessen Code, speist unsere Fälle ein und schreibt Fixtures. Keine Zeile am
Upstream wird geändert, damit er per `git pull` aktualisierbar bleibt.

R ist Entwicklerwerkzeug hinter `make fixtures`, niemals Laufzeitabhängigkeit.

### Eingaben

1. **Tüxen-Archiv** aus vegetweb. Der Lauf verwendet **10.295** Aufnahmen: von
   den 10.717 des Datensatzes sind 422 Nullinsel-Plots (Koordinate 0/0)
   ausgeschlossen. Erreichte Eindeutigkeitsquote: **89,42 %**. Die
   veröffentlichte Marke von 94 % wird also nicht erreicht; der Abstand von
   3,8 Punkten wird darauf zurückgeführt, dass das Upstream-Beispiel den
   GermanSL-Backbone nicht anwendet. Die 94 % sind **keine** Abnahmebedingung
   und werden von keinem Test geprüft.
2. **Regelgetriebene synthetische Plots**, 1.042 Stück — je Bedingung ein Fall
   knapp über und knapp unter der Schwelle. Einzige Quelle mit systematischer
   Abdeckung; das Tüxen-Archiv ist deutschlandlastig und rührt Mittelmeer- und
   Schwarzmeerregeln nie an.
3. **Die Defektfälle aus Sektion 1** mit der in §8 entschiedenen Auflösung.

Zusammen 11.337 Aufnahmen.

### Fixtures

```
testdata/golden/
  rulepack.sha256      Hash der Regelwerksdatei
  upstream.commit      Commit des R-Referenzstands
  meta.json            Manifest: n_plots, n_rules, R-Version, Erzeugungszeit
  cases.jsonl          Eingaben
  synthetic.jsonl      die regelgetriebenen Eingaben allein
  expected.jsonl       winner und matches[] je Aufnahme
  intermediates.jsonl  die Bedingungswerte und Ausdruckswahrheiten je Aufnahme
  conditions.json,     die Bedingungs- und Ausdruckstexte des Upstreams,
  expressions.json,    an denen die Indizes in intermediates.jsonl hängen
  rule-exprs.json
```

Erzeugt werden sie von `make fixtures`; beschrieben ist der Vorgang in
`spike/resy/README.md`.

Beide Hashes sind Teil der Fixtures. Ändert sich einer, schlägt der Test fehl und
der Diff zeigt, was die neue Version bewirkt.

### Abnahme

Übereinstimmung in `result`, in der **vollständigen** Trefferliste **und in jedem
Bedingungswert**. Das Abnahmekriterium ist **null Abweichung** über alle 11.337
Aufnahmen: 0 Gewinner-Abweichungen, 0 Trefferlisten-Abweichungen, 0 abweichende
Ausdruckswahrheiten. Eine Abweichung gilt als Fehler in Habitatus, bis das
Gegenteil gezeigt ist.

### Grenze des Verfahrens

Der Golden Master beweist **Übereinstimmung mit der R-Implementierung**, nicht
fachliche Richtigkeit. Ein öffentlicher Gold-Standard „Aufnahme → EUNIS,
unabhängig von ESy vergeben" existiert nicht; die EVA-Plots aus Chytrý et al. 2020
tragen Zuordnungen, die das Expertensystem selbst erzeugt hat. Als
Plausibilitätsproben — nicht als Orakel — dienen die charakteristischen
Artenkombinationen und die Verbreitungskarten aus dem Zenodo-Record.

---

## 8. Entschiedene Grenzfälle

### Widersprüchliche Einträge in Sektion 1

Fünf Quellnamen verweisen auf zwei verschiedene Zielkonzepte. **Regel: der erste
Eintrag gewinnt**, mit einer begründeten Ausnahme.

| Quellname | Zeilen | gewählt | Begründung |
|---|---|---|---|
| `Bupleurum commutatum subsp. glaucocarpus` | 17875, 17954 | `Bupleurum commutatum` | erster Eintrag |
| `Cirsium pseudopersonata subsp. pseudopersonata` | 20704, 26782 | `Carduus pseudopersonata` | erster Eintrag |
| `Oenothera syrticola` | 86696, 86758 | `Oenothera biennis aggr.` | erster Eintrag |
| `Populus x canadensis + P. nigra` | 100543, 100794 | `Populus x canadensis` | **Ausnahme**: das Ziel `Polypogon monspeliensis x viridis` ist ein Süßgras und eindeutig ein Datenfehler; *Polypogon* und *Populus* stehen alphabetisch benachbart |
| `Rumex arifolius subsp. amplexicaulis` | 114226, 114421 | `Rumex arifolius` | erster Eintrag |

`Marrubium astracanicum subsp. astracanicum` und `Minuartia mesogitana
s. kotschyana` sind doppelt mit identischem Ziel — ohne Wirkung.

Jeder Fall liegt als Fixture mit dieser Begründung vor. Antworten die Autoren
anders, ist es eine Änderung an einer Tabelle.

### Ketten

`Elytrigia species` und `Cf. Verbascum species` sind zugleich Quelle und Ziel.
**Regel: einmalige Anwendung, keine transitive Auflösung** — konservativ, weil
transitive Auflösung `Cf. Verbascum species (rosette leaf)` in `ZZZ Noise` laufen
ließe und damit einen Datensatz verwirft. Ebenfalls als Fixture hinterlegt und in
der Notiz an die Autoren angefragt.

### `#NN Gruppe` ist im Upstream immer FALSE

> **Nachtrag 2026-09-12.** Dieser Abschnitt beschreibt den Modus `faithful`.
> Seit [`2026-09-12-zwei-semantiken.md`](2026-09-12-zwei-semantiken.md) läuft
> der Dienst standardmäßig im Modus `repaired`, in dem genau diese 128
> Ausdrücke wieder über ihren Zahlenwert gewandelt werden (0 = falsch, sonst
> wahr) — so wie ESy es bis v1.1 tat. Die unten aufgezählten 100
> unerreichbaren Regeln sind dort erreichbar; `Stats.Unreachable` ist im
> Modus `repaired` leer. Die Nachbildung in `rulepack.isAlwaysFalse` bleibt
> unverändert: sie markiert die Ausdrücke, der Modus entscheidet, was daraus
> folgt.

Die folgenreichste nachgebildete Eigenheit. Gemeint ist „die Gruppe hat
mindestens NN Arten"; ausgewertet wird sie nie.

Der Upstream zerlegt Ausdrücke ausschließlich an den **mit Leerzeichen
umgebenen** Operatoren (`ParsingExpertFile.R:432-434`):

```r
membership.conditions2 <- unlist(strsplit(membership.expressions, " GR "))
```

Ein Ausdruck ohne Operator bekommt normalerweise `" GR NON <sich selbst>"
angehängt und wird regulär ausgewertet — die `#NN`-Form ist von dieser
Ergänzung ausgenommen, weil `ParsingExpertFile.R:236-237` prüft, ob Zeichen 3
eine Ziffer ist. Sie bleibt damit eine einzelne Bedingung, ihr ausgewerteter
Wert ist eine blanke Zahl, und Schritt 8 verwirft jede Zahl
(`step3and5…R:493`):

```r
logi1[which(unlist(lapply(logi1, is.numeric)))] <- FALSE
```

Wirkung im Regelwerk 2025-10-03: **100 der 312 Regeln können nie feuern**, weil
jeder Weg zu TRUE durch eine solche Bedingung führt. Den Schwerpunkt bilden die
Fels- (`R…`) und Grasland-Blöcke (`U…`); vollständig sind es

```
N15! N15!! N16! N17! N34
Q12 Q21 Q23 Q31 Q45 Q46 Q54 Qa
R11 R12 R13 R14 R16 R17 R18 R19 R1A R1B R1B! R1C R1D R1E R1F R1G R1H R1K
R1M R1P R1Q R1R R21 R22 R23 R23! R24 R24! R31 R32 R33 R34 R35 R36 R37 R41
R41! R42 R43 R44 R45 R51 R52 R57
S12 S61 S62 S66 S67 S68
T36 T3C
U21 U22 U23 U24 U25 U26 U27 U28 U29 U2A U31 U32 U33 U34 U35 U36 U37 U38
U3C U3D U52 U61 U62 U71 U71! U72
V11! V12 V12! V13 V13! V15 V32 V34 V35
```

Nachgebildet in `rulepack.isAlwaysFalse`; die Liste wird beim Laden statisch
berechnet und als `Stats.Unreachable` ausgewiesen, damit sie nicht mit „Regel
ist bisher nicht vorgekommen" verwechselt wird. Sie gilt für `faithful`; in
`repaired` ist sie leer.

Der 128. dieser Ausdrücke ist `<#TC Cliff-ferns GR05>` in Regel `Q61`, dessen
Operator ohne Leerzeichen geschrieben ist. Weil der Upstream ihn nie zerlegt,
sucht er die Gruppe **`Cliff-ferns GR05`** — die es nicht gibt (`fmatch`
liefert `NA`, `groups[[NA]]` ist `NULL`), und der Bedingungswert bleibt für
jede Aufnahme 0. Der Ausdruck ist damit in **beiden** Modi falsch, nicht nur
in `faithful`; `ParseExpr` bildet das nach, indem es den Operator in dieser
Form bewusst nicht anwendet. Am Orakel des `repaired`-Laufs geprüft.

### Bekannte Eigenheiten, die nachgebildet werden

- Trefferliste im Original bei zehn abgeschnitten
- Prioritätsstufen vergleicht der Upstream als Zeichenketten (Faktorstufen,
  `prep.R:71`); `esy.winner` sortiert ganzzahlig. Beides stimmt überein,
  solange die Stufen einstellig bleiben (im Regelwerk 2025: 1–8). Ab Stufe 10
  würden die Verfahren auseinanderlaufen — die Zeichenkettenordnung ist also
  **nicht** nachgebildet, sondern nur im heutigen Wertebereich äquivalent.
- `N_Dunes` im Testdatensatz gegen `Y_DUNES` im Regelwerk — uneinheitliche
  Schreibung, folgenlos, weil Regeln nur auf `Y_DUNES` prüfen

---

## 9. Parser

### Dateistruktur

Vier Sektionen `SECTION n: <Titel>` … `SECTION n: End`. Sektion 1 als Blöcke aus
nicht eingerückter Kopfzeile und eingerückten Quellnamen; Sektion 2 als Gruppen
`### Name` mit eingerückten Arten; Sektion 3 als Regeln; Sektion 4 leer.

**Formeln können über mehrere Zeilen laufen.** Regel `S63` (Eastern garrigue)
verteilt sich über drei Zeilen mit Umbrüchen nach `OR` und vor `NOT`: 312 Regeln,
aber 314 Formelzeilen. Ein zeilenweiser Parser läse das als drei kaputte Formeln,
und zwar ohne Fehler, weil die Fragmente syntaktisch plausibel aussehen.
Formelzeilen werden bis zur nächsten Kopfzeile gesammelt.

Trennzeilen zwischen Blöcken enthalten Leerzeichen, sind also nicht leer.

### Grammatik

```
Formel    := Term { ("AND" | "OR" | "NOT") Term }
Term      := "(" Formel ")" | Ausdruck
Ausdruck  := "<" Menge [ Op Rechts ] ">"
Menge     := Atom { "|" Atom } [ "EXCEPT" Atom { "|" Atom } ]
Atom      := Präfix [ "+" NN ] Bezeichner | Artname | ("$$C"|"$$N") Feldname
Rechts    := Zahl | Menge | Sondertoken
Op        := "GR" | "GE" | "EQ"
```

2140 Ausdrücke, 931 verschieden. `|` vereinigt Gruppen (183 Ausdrücke), `EXCEPT`
bildet die Differenz (75).

Zwei Tokenisierungsfallen: Feldnamen enthalten Leerzeichen und Klammern
(`<$$N Altitude (m) GR 1000>`), und Artnamen stehen direkt als Atom
(`<Empetrum nigrum aggr. GR 05>`). Trennung an Leerzeichen zerlegt beides falsch.

### Konsistenzprüfung beim Laden

Ein Ladefehler beendet den Start — ein Dienst mit halb geparstem Regelwerk liefert
stille Falschergebnisse. Regelwerks-*Defekte* dürfen den Start dagegen **nicht**
verhindern, sonst ließe sich Habitatus mit der Originaldatei nicht betreiben. Die
Prüfung zählt und protokolliert: doppelt vergebene Quellnamen, Ketten, Gruppen
ohne Definition, Regeln mit Verweis auf unbekannte Gruppen.

Als Testfall dient ein Fehler, den die Upstream-Autoren selbst hatten (`fb86835`):
die letzte Art der letzten Gruppe in Sektion 2 fiel beim Parsen weg — der typische
Blockparser-Fehler am Dateiende.

---

## 10. Beobachtbarkeit

Zwei Kennzahlen mit fachlicher Aussage: der Anteil `?` und `+` an allen Anfragen,
und welche Regeln über die Zeit nie feuern. Beides deckt Fehler auf, die kein
Testfall findet — etwa einen Client, der ein Kopfdatum systematisch falsch
befüllt.

---

## 11. Offene Punkte

- **Antwort der Autoren** zu Auflösungsreihenfolge und Ketten (§8). Die Notiz ist
  entworfen und noch nicht versandt.
- **Weitere Backbones.** Für WCVP und Ehrendorfer gibt es keine mitgelieferte
  Tabelle. Solange sie fehlt, sind diese Backbones nicht bedienbar.
- **Dokumentationsdrift im User-Guide.** Appendix S5 beschreibt `ECOREG_WWF` als
  Dreibuchstaben-Code, das Regelwerk 2025 nutzt numerische `ECO_ID`. Bei jedem
  Regelwerks-Update ist das Kopfdatenformat neu zu prüfen.
