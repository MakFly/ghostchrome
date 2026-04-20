# ghostchrome v1.0.0 — Release planning

Documents de planification pour la release v1.0.0 (cible Q2 2026).

| Fichier | Contenu |
|---|---|
| [`PRD.md`](./PRD.md) | Product Requirements Document — contexte, personas, objectifs, requirements détaillés, non-objectifs, métriques de succès, risques |
| [`SPRINT.md`](./SPRINT.md) | Sprint plan exécutable — 3 sprints (1 + 2 + 2 semaines), breakdown task-level, backlog reporté, checklist release |

## Résumé en 30 secondes

**v0.6.0 (actuel)** : CLI agent-first complète (observation, interaction, auth, émulation, validation, perf, visual diff).

**v1.0.0 cible** : produit distribuable.
- **Go SDK** importable (`go get github.com/MakFly/ghostchrome/sdk`).
- **Node binding** (`npm i ghostchrome`) avec types TypeScript.
- **Python binding** (`pip install ghostchrome`).
- **`ghostchrome record`** : codegen record-and-replay.
- **`ghostchrome mcp`** : serveur MCP natif pour Claude Code & assimilés.
- **Docs publiques** sur GitHub Pages.
- **Packaging** : Homebrew, npm, PyPI, curl installer, 5 binaires pré-buildés.
- **SemVer v1.x stable** : pas de breaking avant v2.

## Non-objectifs (pour mémoire)

- Playwright Test runner clone.
- Support Firefox/WebKit.
- Trace viewer GUI.
- Video recording.

## Effort estimé

~5 semaines solo / ~3 300 LOC, découpé en 3 sprints mergeables individuellement.
