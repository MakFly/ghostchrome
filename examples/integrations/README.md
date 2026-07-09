# Intégrer ghostchrome — 2 voies distinctes (remplacer Playwright)

ghostchrome remplace Playwright pour piloter Chrome, avec une sortie taillée pour
le budget tokens des agents LLM. Deux voies d'intégration **distinctes**, à choisir
selon *qui* pilote le navigateur.

```
                        ghostchrome (1 binaire Go statique)
                                     │
        ┌────────────────────────────┴────────────────────────────┐
        │                                                          │
╔═══════▼═════════ Voie A — MCP ═════════╗        ╔════════════════▼═══ Voie B — CLI ════════╗
║  L'AGENT pilote (Claude Code / Codex)   ║        ║  TON CODE pilote (scripts, tests, jobs)   ║
║  ┌───────────────────────────────────┐  ║        ║  ┌──────────────────────────────────────┐ ║
║  │ claude mcp add ghostchrome …      │  ║        ║  │ ghostchrome preview|navigate|extract…  │ ║
║  └──────────────┬────────────────────┘  ║        ║  └──────────────────┬───────────────────┘ ║
║                 │ JSON-RPC (stdio)       ║        ║                     │ argv → sortie compacte║
║        ┌────────▼────────┐               ║        ║          (aucun agent, aucun JSON-RPC)     ║
║        │ ghostchrome mcp │ 16 outils     ║        ║                                            ║
╚═════════════════│════════════════════════╝        ╚═════════════════════│═════════════════════╝
                  └───────────────┬────────────────────────────────────────┘
                                  │ CDP / Rod
                         ┌────────▼─────────┐
                         │ Chrome (daemon    │  session "default", ~/.ghostchrome/profiles
                         │ headless persistant)
                         └──────────────────┘
```

**Légende** — Voie A : serveur MCP 2025-11-25, 16 outils, Chrome long-lived (les
refs `@1/@2` survivent entre appels). Voie B : CLI one-shot, daemon Chrome
transparent réutilisé entre commandes. Les deux partagent le même moteur CDP.

---

## Voie A — MCP (l'agent pilote)

Remplacement 1-pour-1 de `@playwright/mcp`. Wire-up **une fois** :

```bash
# Claude Code (scope user → dispo partout)
claude mcp add ghostchrome -s user -- ghostchrome mcp --stealth

# Codex
codex mcp add ghostchrome -- ghostchrome mcp --stealth
```

Config portable réutilisable : [`ghostchrome.mcp.json`](./ghostchrome.mcp.json)
(consommable par `claude -p --mcp-config … --strict-mcp-config`).

Outils : `snapshot navigate click type select press hover drag fill_form upload
tabs wait_for eval screenshot back forward`.

## Voie B — CLI pure (ton code pilote)

Aucun agent, aucun MCP. Le binaire fait tout, la sortie est compacte.
Démo runnable : [`cli-flow.sh`](./cli-flow.sh)

```bash
./cli-flow.sh https://example.com
```

---

## Validation e2e — Voie A prouvée 100×

[`e2e/run-e2e.sh`](./e2e/run-e2e.sh) lance N vrais runs `claude -p` (headless) qui
pilotent le navigateur via le MCP ghostchrome **isolé** (`--strict-mcp-config`),
avec assertion sur la sortie. Cibles RFC 2606 (`example.com/.org/.net`) : réelles,
jamais rate-limitées, contenu stable → e2e réel *et* déterministe.

```bash
./e2e/run-e2e.sh 100 4      # 100 cas, concurrence 4
```
