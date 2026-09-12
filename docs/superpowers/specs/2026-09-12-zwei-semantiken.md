# Zwei Semantiken: `repaired` und `faithful`

**Datum:** 2026-09-12
**Status:** umgesetzt

## Warum

habitatus bildet ESy v1.2 nach. In v1.2 steckt seit dem 2025-02-03 eine
Regression: `#NN Gruppe`-Ausdrücke („mindestens N Arten der Gruppe") werden
pauschal auf FALSE gesetzt, wodurch **100 der 312 Regeln nie feuern können** —
der gesamte Grasland-Block `R11`–`R57`, die Felsvegetation `U21`–`U72` und
weitere.

Belegt ist das dreifach:

- Die Zeile `logi1[which(unlist(lapply(logi1, is.numeric)))] <- FALSE` wurde in
  Commit `376cffc` (2025-02-03) **neu eingefügt**; davor existierte sie nicht.
- Vorher trug `logi1` numerische 1/0-Vektoren, die R in `&` und `|` automatisch
  in Wahrheitswerte wandelt — nachgerechnet: `c(1,0,3) & TRUE` ergibt
  `TRUE FALSE TRUE`. Genau das tut die auskommentierte Zeile direkt darüber.
- Ein Lauf beider Fassungen über das Tüxen-Archiv: 89,42 % eindeutige Zuordnung
  wie ausgeliefert, **93,61 %** mit der Korrektur. Bruelheide et al. 2021
  berichten **94 %** für denselben Datensatz — die veröffentlichte Zahl ist nur
  mit der Korrektur reproduzierbar.

Originaltreue hieße hier also: eine neunzehn Monate alte Regression nachbilden,
die ein Viertel aller Zuordnungen verfälscht. Für einen produktiven Dienst ist
das nicht tragbar. Den Paritätsnachweis gegen v1.2 wollen wir trotzdem behalten.

## Die Entscheidung

Zwei Betriebsarten, beide vollständig getestet.

| Modus | Semantik | Zweck |
|---|---|---|
| **`repaired`** (Standard) | wie ESy **vor** v1.2: numerische Bedingungswerte werden zu Wahrheitswerten gewandelt (0 = falsch, sonst wahr) | der Dienst, den wir betreiben |
| `faithful` | wie ESy **v1.2**: solche Ausdrücke sind immer falsch | Paritätsnachweis, Vergleich mit Ergebnissen anderer v1.2-Läufe |

`repaired` ist **nicht** unsere Erfindung: Es ist exakt das Verhalten der
R-Implementierung bis einschließlich v1.1, und exakt das, was die im Quelltext
auskommentierte Zeile herstellt. Beide Modi sind damit gegen ein reales Orakel
prüfbar — `faithful` gegen v1.2 wie ausgeliefert, `repaired` gegen dieselbe
Fassung mit getauschten Zeilen.

### Was sich zwischen den Modi unterscheidet

Genau die 128 Ausdrücke, die die R-Implementierung ohne Vergleichsoperator
lässt und deshalb numerisch bleiben:

- 127 der Form `#NN Gruppe` — in `repaired` gilt „mindestens N Arten vorhanden",
  in `faithful` immer falsch.
- 1 Sonderfall, `<#TC Cliff-ferns GR05>` in Regel `Q61`, dessen Operator ohne
  Leerzeichen geschrieben ist. In `repaired` wird sein numerischer Wert
  gewandelt, das heißt: wahr, sobald die Gruppe überhaupt Deckung hat. Das ist
  **nicht** die vermutlich gemeinte Schwelle von 5 %, sondern das, was R vor
  v1.2 gerechnet hat. Ein Defekt der Regeldatei bleibt es in beiden Modi; wir
  bilden ihn nach, statt ihn zu raten.

Nichts anderes unterscheidet die Modi. Das ist nachgemessen, nicht angenommen:
alle 128 numerisch gebliebenen Einträge wurden einzeln geprüft.

## Anforderungen

1. `esy.Env` trägt den Modus. In `faithful` liefern die betroffenen Ausdrücke
   falsch, in `repaired` den gewandelten Zahlenwert. Der berechnete Zahlenwert
   wird in beiden Modi geführt, weil der Golden Master ihn vergleicht.
2. `classify.NewService` nimmt den Modus entgegen und gibt ihn in `versions`
   jeder Antwort aus. Ein Aufrufer muss erkennen können, welche Semantik sein
   Ergebnis erzeugt hat — ohne das ist ein Ergebnis nicht interpretierbar.
3. `cmd/habitatus` bekommt `-mode repaired|faithful`, Standard `repaired`, und
   protokolliert die Wahl beim Start.
4. Zwei Fixture-Sätze und zwei Golden Master, beide bei null Abweichungen:
   `testdata/golden/` gegen v1.2 wie ausgeliefert, `testdata/golden-repaired/`
   gegen dieselbe Fassung mit getauschten Zeilen. Der Generator patcht dafür
   eine **Kopie** des Upstreams; der Klon unter `spike/ESy-upstream` bleibt
   unangetastet.
5. Die Erreichbarkeitsanalyse und die Kennzahl „unerreichbare Regeln" hängen vom
   Modus ab: in `repaired` sollte die Menge leer oder nahezu leer sein. Das ist
   zugleich die schärfste Probe, dass der Modus wirkt.

## Was wir uns damit einhandeln

Zwei Semantiken sind zwei Dinge, die auseinanderlaufen können. Dagegen steht,
dass beide gegen ein eigenes Orakel geprüft werden und beide bei null bleiben
müssen. Fällt einer von beiden, ist es sichtbar.

Wenn das RESY-Team die Korrektur übernimmt, wird `repaired` zur Normalfassung
und `faithful` zum historischen Modus. Die Umschaltung bleibt trotzdem
nützlich: Wer Ergebnisse aus der Zeit zwischen Februar 2025 und dem Fix
nachvollziehen muss, braucht sie.

## Umsetzung

Umgesetzt wie beschrieben. Zwei Abweichungen vom Text oben, beide am Orakel
geprüft:

1. `<#TC Cliff-ferns GR05>` ist **in beiden Modi falsch**, nicht nur in
   `faithful`. Oben steht, in `repaired` werde sein Zahlenwert gewandelt und
   sei damit wahr, sobald die Gruppe Deckung hat. Das trifft nicht zu: Weil
   der Upstream den Ausdruck nie zerlegt, sucht er die Gruppe
   `Cliff-ferns GR05` statt `Cliff-ferns`. Die gibt es nicht (`fmatch` → `NA`,
   `groups[[NA]]` → `NULL`), der Bedingungswert bleibt für jede Aufnahme 0,
   und 0 gewandelt ist falsch. `rulepack.ParseExpr` bildet das nach, indem es
   den angeklebten Operator bewusst nicht anwendet. Der Regeldatei-Defekt
   bleibt in beiden Modi bestehen — nur eben als konstantes Falsch.
2. Die Modi unterscheiden sich damit in der **Auswertung** von 127 der 128
   Ausdrücke; der 128. ist in beiden Modi falsch. Die Menge selbst ist
   unverändert die, die `Expr.AlwaysFalse` markiert.

Ergebnis der beiden Golden Master über je 11.337 Aufnahmen: beide bei **null**
Abweichungen. `Stats.Unreachable` umfasst in `faithful` 100 Regeln, in
`repaired` **keine**. Über die 10.295 Archiv-Aufnahmen: 89,42 % eindeutig
zugeordnet in `faithful`, **93,61 %** in `repaired`, 2.657 Aufnahmen mit
geändertem Gewinner — wie im Experiment vorhergesagt.
