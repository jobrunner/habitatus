# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Projektziel

In diesem Repo soll eine teilweise, aber funktionale Portierung des RESY-Expertensystems nach Go stattfinden: aus einer Pflanzenliste mit Deckungswerten sollen EUNIS-Habitate ermittelt werden.

Fachliche Begriffe, die durchgehend gelten:
- **Aufnahme** — eine Vegetationsaufnahme: Liste von Taxa mit Deckungsgrad.
- **Deckung** — Deckungsgrad eines Taxons (RESY arbeitet mit Braun-Blanquet-artigen Skalen; die konkrete Skala/Umrechnung ist beim Portieren explizit festzulegen und zu dokumentieren).
- **EUNIS-Habitat** — Zielklassifikation der Zuordnung (hierarchische Codes, z. B. `R1`, `R1A`).
- **RESY** — das ursprüngliche Expertensystem, dessen Regelwerk hier portiert wird; die Original-Regeln sind die fachliche Referenz.

## Aktueller Stand

Das Repo ist ein frisches Gerüst: es gibt **noch keinen Go-Code**, kein `go.mod`, keine Tests und noch keinen Commit auf `main`. Wer hier als Erstes Code anlegt, legt damit auch Modulpfad, Layout und Build-/Test-Kommandos fest — diese Datei ist dann entsprechend zu ergänzen (Build, Lint, Test, Einzeltest-Aufruf).

Beim Anlegen des Service-Gerüsts ist die Skill `new-go-service` der vorgesehene Weg (hexagonale Architektur, Quality-Gates, Observability, Docs) — sie ist über das Submodul bereits eingebunden.

## Skills-Submodul

`vendor/claude-skills` ist ein Git-Submodul (`git@github.com:jobrunner/claude-skills.git`, Branch `main`). Die Einträge unter `.claude/skills/` sind **Symlinks** in dieses Submodul — Skills werden also nie hier im Repo bearbeitet, sondern upstream. Zum Aktualisieren des Pointers die Skill `update-skills-submodule` verwenden.

Nach einem frischen Clone:

```bash
git submodule update --init --recursive
```

## Sprache

Code, Bezeichner und Commit-Messages auf Englisch; Doku, Kommentare zur Fachlogik und Kommunikation auf Deutsch. Fachbegriffe (Aufnahme, Deckung, EUNIS) bleiben in ihrer etablierten Form.
