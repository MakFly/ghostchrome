# Sprint plan — ghostchrome v1.0.0

> Plan d'exécution des requirements listés dans `PRD.md`.
> **Durée cible** : 5 semaines, découpées en 3 sprints de 1-2 semaines.
> **Mode** : solo dev + agent en paire ; lignes de code estimées approximatives.

---

## Architecture visée

```
╔════════════════════════════════════════════════════════════════════════════════════╗
║                          ghostchrome v1.0.0 — distribution surface                 ║
╚════════════════════════════════════════════════════════════════════════════════════╝

      ┌──────────────────────────────────────────────────────────────────────┐
      │                    Consommateurs (adoption cible)                    │
      │   Go app       Node app      Python app     Claude Code    CLI user  │
      └─────┬─────────────┬──────────────┬────────────────┬─────────────┬────┘
            │             │              │                │             │
            ▼             ▼              ▼                ▼             ▼
    ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌────────────┐   ┌────────┐
    │  Go SDK  │   │ npm pkg  │   │ pip pkg  │   │ MCP server │   │ binary │
    │ sdk/*.go │   │ node/    │   │ python/  │   │ (stdio)    │   │        │
    └─────┬────┘   └────┬─────┘   └────┬─────┘   └─────┬──────┘   └────┬───┘
          │             │              │                │               │
          │             └── wraps ─────┴────spawns─────►│               │
          │                                             │               │
          │                                             ▼               ▼
          │                                    ┌─────────────────────────┐
          └────────────────────────────────────▶│   cmd/ (cobra) + engine/│
                                                │   (existing, stable)    │
                                                └────────────┬────────────┘
                                                             │
                                                             ▼
                                                ┌────────────────────────┐
                                                │ Rod + CDP → Chrome     │
                                                └────────────────────────┘

Distribution :
  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────┐ ┌──────────────────┐
  │Homebrew  │ │   npm    │ │   PyPI   │ │ curl|sh      │ │ GitHub Releases  │
  │  tap     │ │ package  │ │ package  │ │ installer    │ │ prebuilt binaries│
  └──────────┘ └──────────┘ └──────────┘ └──────────────┘ └──────────────────┘
```

---

## Sprint S1 — Go SDK & API stability (Semaine 1)

**Goal** : Expose `engine/` comme package public, stable, documenté. Servir la persona **Eve (Go dev)** et fondation pour R2/R3.

### S1.1 Audit API et godoc — 1 jour
- Passer chaque symbole exporté de `engine/` en revue.
- Marquer internal ce qui doit l'être (`engine/internal/`).
- Ajouter godoc sur tous les types et fonctions publics.
- Livrable : `engine/doc.go` avec présentation du package.

### S1.2 Créer `sdk/` package haut-niveau — 2 jours
- `sdk/browser.go` : `NewBrowser(Options)` → `*Browser`.
- `sdk/page.go` : méthodes `Navigate`, `Click`, `ClickByRef`, `ClickByText`, `ClickByRole`, `Type*`, `Assert*`.
- `sdk/flow.go` : `Flow.Run(steps []Step)` = wrapper Go autour de `cmd/batch.go`.
- API objectif :
  ```go
  b, err := sdk.NewBrowser(sdk.Options{Headless: true, Stealth: true})
  defer b.Close()
  p, _ := b.Navigate("https://example.com")
  p.ClickByText("Learn more")
  assert(p.URLContains("/iana/"))
  ```
- Tests : `sdk/sdk_test.go` avec 3 scénarios (navigate, form fill, assertion).

### S1.3 Exemples + tutoriel — 0.5 jour
- `docs/v1.0.0/examples/go-quickstart.md`
- `docs/v1.0.0/examples/go-scrape-hn.go`
- `docs/v1.0.0/examples/go-pdf-generator.go` (pour Eve)

### S1.4 CI : test Go SDK — 0.5 jour
- GitHub Actions job pour `go test ./sdk/...` sur matrice (linux, macos).

### S1.5 Tag snapshot `v1.0.0-beta.1` — 0.5 jour
- Premier draft SDK publique, pour retours early adopters.

**Deliverable S1** : SDK Go fonctionnel, documenté, testé. `go get github.com/MakFly/ghostchrome/sdk` fonctionne.

**LOC estimé** : ~600 (sdk package + tests + godoc).

---

## Sprint S2 — Bindings Node & Python (Semaine 2-3)

**Goal** : Packages npm + PyPI qui wrap le binaire. Servir **Charlie (Node)** et **Dana (Python)**.

### S2.1 Node binding — 3 jours
- Scaffolding `node/` à la racine :
  ```
  node/
  ├── package.json
  ├── tsconfig.json
  ├── src/
  │   ├── browser.ts
  │   ├── page.ts
  │   ├── types.ts
  │   └── index.ts
  ├── scripts/
  │   └── postinstall.js         (download binary from GitHub Releases)
  └── test/
      └── *.test.ts
  ```
- `browser.ts` : `Browser.launch(opts)` spawne `ghostchrome serve` en arrière-plan, capture `ws://`.
- `page.ts` : méthodes qui invoquent `ghostchrome <cmd> --connect ws://... --format json` et parse stdout.
- Error handling : exit code 1 → throw error typé (`InterceptError`, `AssertionError`, `StaleRefError`).
- Publishing : `npm publish` (package `ghostchrome`).

### S2.2 Python binding — 2 jours
- Scaffolding `python/` :
  ```
  python/
  ├── pyproject.toml
  ├── src/ghostchrome/
  │   ├── __init__.py
  │   ├── browser.py
  │   ├── page.py
  │   └── types.py
  └── tests/
  ```
- Stratégie miroir de Node : subprocess + --format json.
- Publish : `uv publish` (ou `twine`).

### S2.3 Tests bindings + CI — 1 jour
- Node : `bun test` dans CI.
- Python : `pytest` + `mypy --strict` dans CI.
- Matrice : ubuntu-latest + macos-latest.

### S2.4 Docs bindings — 0.5 jour
- `docs/quickstart/node.md`
- `docs/quickstart/python.md`
- Snippet migration 10-lignes depuis Playwright (node) et pyppeteer (python).

### S2.5 Packaging release tooling — 1.5 jour
- GitHub Actions workflow :
  - Job `release-binaries` : build pour linux-amd64, linux-arm64, darwin-amd64, darwin-arm64, windows-amd64. Upload sur Release.
  - Job `release-npm` : bump version `node/package.json`, `npm publish`.
  - Job `release-pypi` : bump version `python/pyproject.toml`, build + publish.
  - Job `release-homebrew` : met à jour le tap avec le nouveau SHA.
- Tout déclenché par `git tag v1.0.0` + push.

### S2.6 Tag `v1.0.0-beta.2` — 0.5 jour
- Beta pour Charlie + Dana testers.

**Deliverable S2** : `npm i ghostchrome` et `pip install ghostchrome` fonctionnent, tous deux avec TypeScript/types + tests.

**LOC estimé** : ~1 500 (node+python+CI+docs).

---

## Sprint S3 — Codegen, MCP, docs complètes (Semaine 4-5)

**Goal** : Expérience dev finale. Servir **Frank (newcomer)** via record, **Alice** via MCP natif.

### S3.1 `ghostchrome record` — 3 jours
- Fichier `cmd/record.go` + `engine/recorder.go`.
- Au démarrage : lance Chrome non-headless + active `Input.dispatchMouseEvent` + `Input.dispatchKeyEvent` listeners.
- Chaque event : résout le locator sémantique (role+name > label > text > CSS fallback) via a11y tree.
- Accumule steps en mémoire, écrit `.gcb` sur SIGINT.
- Cases à couvrir : navigate (via `frameNavigated`), click, type (par `beforeinput`), submit, select, scroll.
- Output exemple :
  ```
  # recorded 2026-04-30T10:12 — https://github.com/login
  navigate https://github.com/login
  type --by-label "Username or email address" "kev@x.fr"
  type --by-label "Password" "<redacted>"
  click --by-role button --by-name "Sign in"
  wait-selector "#user-nav"
  ```
- Test : record sur 3 sites (GitHub login, Amazon search, form httpbin), replay OK.

### S3.2 MCP server — 2 jours
- Fichier `cmd/mcp.go` + `engine/mcp_server.go`.
- Implémente spec MCP stdio JSON-RPC 2.0.
- Enregistre chaque cobra command comme tool MCP :
  - Nom : `ghostchrome_<verb>` (ex `ghostchrome_click`).
  - Schema : généré depuis les flags cobra via reflection/mapping.
  - Handler : appelle la fonction Run directement (pas de re-spawn).
- Tool composite `ghostchrome_flow` pour batch-like usage.
- Test : serveur lancé + `claude` config + tools listés + appels fonctionnels.

### S3.3 Docs complètes — 2 jours
- `docs/README.md` — landing page, pitch en 5 lignes + comparaison Playwright.
- `docs/quickstart/cli.md` — 10 commandes essentielles.
- `docs/guides/agent-usage.md` — comment Claude Code utilise ghostchrome (batch + diff + asserts).
- `docs/guides/frontend-validation.md` — flow complet smoke-check d'un front.
- `docs/guides/scraping.md` — collect multi-URL + intercept + HAR.
- `docs/guides/migration-from-playwright.md` — tableau équivalences + snippets.
- `docs/reference/commands/*` — une page par verbe (auto-génération à envisager).
- `docs/reference/sdk/*` — godoc HTML, TypeDoc, Sphinx.
- `docs/examples/*` — 15+ use cases.
- Déploiement GitHub Pages via Actions (mkdocs ou Docusaurus).

### S3.4 Screencasts + blog post — 1 jour
- 2-3 gif/vidéos courts (asciinema pour le CLI).
- Post Medium/blog : « ghostchrome v1.0 : the agent-first Playwright alternative ».
- Soumission à Hacker News + r/programming le jour J.

### S3.5 Release v1.0.0 — 0.5 jour
- Tag `v1.0.0`.
- CI push automatique sur brew/npm/pypi.
- GitHub Release notes avec highlights + migration guide link.

**Deliverable S3** : Utilisable dès le premier quickstart, sans lecture docs préalable pour les cas courants.

**LOC estimé** : ~1 200 (record + mcp + docs).

---

## Récap total

| Sprint | Durée | LOC | Livrables clés |
|---|---|---|---|
| S1 | 1 sem | ~600 | Go SDK, examples Go, v1.0.0-beta.1 |
| S2 | 2 sem | ~1 500 | Node + Python bindings, CI release, v1.0.0-beta.2 |
| S3 | 2 sem | ~1 200 | record, MCP, docs complètes, v1.0.0 |
| **Total** | **5 sem** | **~3 300 LOC** | Release stable v1.0.0 SemVer |

---

## Backlog hors v1.0.0 (candidats v1.1.0+)

Items identifiés mais reportés pour ne pas retarder le release :

| Item | Raison du report |
|---|---|
| Browser contexts isolés | Demande architectural réécrire `engine.Browser` ; bénéfice marginal pour use cases agents |
| HAR replay (rejouer un HAR comme mock) | Complément intercept, pas critique |
| Network intercept continue/modify (pas juste block/fulfill) | Demande refactor hijack router |
| Locators chainable (`.within()`, `.nth()`) | API plus riche, pas bloquant |
| Trace file format compatible Playwright | Ouvre la porte au trace viewer Playwright mais coûte cher |
| CLI autocompletion (bash/zsh/fish) | Nice to have, cobra le supporte nativement |
| Installation via Scoop (Windows) | Public Windows petit |

---

## Risk & contingency

| Si... | Alors... |
|---|---|
| S1 prend 2 jours de plus | Reporter S3.4 (blog) en post-release |
| MCP spec change pendant S3 | Pinner v0.1 de la spec et shipper, PR de bump en v1.1.0 |
| `ghostchrome record` génère des flows fragiles | Shipper avec label "experimental" ; ne pas le mentionner en front page |
| Binary size > 15 MB | Stripper symboles Go (`-ldflags="-s -w"`), évaluer UPX |
| Un binding casse sur une plateforme | Release partielle (Linux/Mac) + doc ajoutée "Windows best-effort" |

---

## Checklist release v1.0.0

**J-3** :
- [ ] Tous les sprints mergés sur `main`
- [ ] CI verte sur toutes plateformes
- [ ] `CHANGELOG.md` à jour
- [ ] Docs site déployé et vérifié
- [ ] Tags beta ramenés sur le mainline

**J-1** :
- [ ] Dry run du workflow release (`v1.0.0-rc.1`)
- [ ] Vérifier `brew`, `npm i`, `pip install` en local sur une VM propre
- [ ] Revue blog post + screencasts

**J (release day)** :
- [ ] `git tag v1.0.0 && git push --tags`
- [ ] Attendre CI : brew / npm / pypi / Release assets OK
- [ ] Poster blog + HN + Twitter/Discord
- [ ] Monitorer issues GitHub 48 h

**J+7** :
- [ ] Review métriques adoption
- [ ] Patch v1.0.1 si bugs critiques
- [ ] Ouvrir planning v1.1.0
