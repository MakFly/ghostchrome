# PRD — ghostchrome v1.0.0

> **Status**: Draft — 2026-04-20
> **Owner**: Kevin Aubree
> **Target release**: Q2 2026 (~5 weeks from kickoff)

---

## 1. Context

### 1.1 Where we are (v0.6.0)

ghostchrome est aujourd'hui une **CLI agent-first** pour piloter Chrome, complète sur les use cases observation + interaction + validation :

| Domaine | Couverture v0.6.0 |
|---|---|
| Observation | `preview`, `extract` (+ `--diff`), `collect` multi-URL, stealth CDP |
| Interaction | `click`, `type`, `hover`, `press`, `select`, `upload`, `fill-form`, `scroll` |
| Auth/State | `cookies`, `storage save/load` (Playwright-compat) |
| Émulation | `emulate --device`, `geolocation`, `viewport`, `color-scheme`, `timezone` |
| Network | `intercept` (block/fulfill), `--har` recording |
| Validation | `assert text/selector-visible/url/title/count/no-console-errors/no-network-4xx` |
| Visuel | `screenshot --baseline --threshold` (diff pixel) |
| Perf | `perf` avec budgets Web Vitals |
| Flows | `batch` (DSL 1-call), `serve` (persistent) |

**Economies de tokens côté agent** : ~4× en observation simple, ~300× sur flows multi-step vs Playwright MCP.

### 1.2 Le problème qui reste

Trois murs bloquent l'adoption à plus grande échelle :

1. **Pas de librairie importable**. Les dev qui veulent automatiser *dans leur code applicatif* (script Go de scraping, backend Node qui génère des PDFs à la demande, pipeline Python de tests visuels) ne peuvent pas. Ils doivent shell-out vers le binaire → friction, typage faible, overhead de process.

2. **Pas de codegen**. Pour écrire un flow `batch`, l'utilisateur doit naviguer mentalement l'a11y tree, noter les refs, écrire le script. Playwright a `playwright codegen` (record-and-replay). Absence = friction d'onboarding.

3. **Pas d'intégration IDE**. Claude Code peut invoquer ghostchrome via Bash mais sans contexte sémantique. Un serveur MCP natif donnerait à l'agent des outils typés avec completion + validation argumentaire.

### 1.3 Le prompt stratégique

> « Je veux que mon agent valide des flows front rapidement et à coût faible. »

v0.6.0 répond pour des utilisateurs *sachant installer et scripter une CLI*. v1.0.0 doit répondre pour :
- Un dev Node qui veut `import { navigate, assert } from 'ghostchrome'` dans un Jest light.
- Un dev Python qui veut piloter un scraping depuis un notebook.
- Un dev Go qui veut intégrer du browser automation dans son service.
- Un utilisateur Claude Code qui veut que les outils ghostchrome apparaissent comme des tool-calls typés, pas du shell.
- Un nouvel utilisateur qui veut record-and-replay en 1 min plutôt qu'apprendre le DSL.

---

## 2. Persona cibles

### 2.1 Personas existantes (servies par v0.6.0, reconduites)

- **Alice — Agent developer** : Claude Code, Cursor, ou agent custom. Utilise ghostchrome via Bash tool. v1.0.0 lui améliore l'UX via MCP natif.
- **Bob — Scraper ops** : runs de production, CLI via cron. v1.0.0 stable = confort.

### 2.2 Personas nouvelles (v1.0.0 cible)

- **Charlie — Dev fullstack Node.js** : teste son front React en développement. Aujourd'hui utilise Playwright, trop lourd pour ses smoke-checks. Veut un `npm install ghostchrome` qui wrap le binaire avec API TypeScript typée.
- **Dana — Data/research Python** : scrape avec Selenium/Playwright, veut plus léger. Veut `pip install ghostchrome` avec API pythonic.
- **Eve — Backend Go** : doit générer des PDFs marketing à la demande depuis son service Go. Veut `import "github.com/MakFly/ghostchrome/sdk"`.
- **Frank — Newcomer** : découvre ghostchrome, veut générer son premier flow en 30 secondes via record.

---

## 3. Objectifs v1.0.0

### 3.1 Objectifs (ranked)

1. **SDK Go importable** — exposer `engine/` comme package public stable avec API documentée. Pas une refonte, juste la promotion de sous-paquets existants en API SemVer.
2. **Binding Node** — package npm qui wrap le binaire. API TypeScript fournie, même surface que la CLI + sucre pour les cas fréquents.
3. **Binding Python** — package PyPI, même philosophie.
4. **`ghostchrome record`** — codegen. CDP event listener → conversion en script `batch` annoté avec locators sémantiques.
5. **MCP server natif** — `ghostchrome mcp` démarre un serveur stdio Anthropic MCP qui expose chaque commande comme un tool typé. Branchable dans `~/.claude/settings.json`.
6. **Docs publiques** — site statique (docs/ + GitHub Pages), 15+ exemples, guide migration depuis Playwright.
7. **Packaging distribuable** — Homebrew, npm, PyPI, installer script. Binaires pré-buildés pour linux amd64/arm64, macOS (Intel+Apple Silicon), windows amd64.
8. **Compatibilité SemVer v1.x** — API stable, pas de breaking avant v2.

### 3.2 Non-objectifs (explicites)

| Non-objectif | Raison |
|---|---|
| Playwright Test runner clone | Hors positionnement, frais énorme |
| Support Firefox/WebKit | CDP-only, casserait single-binary |
| Trace viewer UI web | Duplique outils existants |
| Video recording | Nécessite ffmpeg runtime dep |
| GUI interactive | Pas le public cible |
| Browser contexts isolés (v0.7.0+ peut-être) | Pas bloquant pour v1.0.0 |

### 3.3 Success criteria mesurables

| KPI | Cible | Comment mesurer |
|---|---|---|
| SDK Go : temps "hello world" | < 5 min depuis `go get` | Tutorial timed |
| Node SDK : même | < 3 min depuis `npm install` | Tutorial timed |
| Python SDK : idem | < 3 min depuis `pip install` | Tutorial timed |
| `ghostchrome record` → script réutilisable | 10-étapes réel sans éditer | Test sur 3 sites |
| MCP server latency | < 150 ms overhead par call | Bench local |
| Docs coverage | 100 % commands documentées | Grep CLI vs docs/ |
| Binaire < 15 MB | OK (actuel : ~12 MB) | `ls -l` |
| Install time sur M1 | < 10 s end-to-end | `time brew install` |
| Test cross-platform pass | 100 % matrice CI | GitHub Actions |

---

## 4. Requirements détaillés

### 4.1 [R1] Go SDK

**Description** : Exposer `engine/` comme package public importable à l'API stable.

**Scope** :
- Refactor `engine/` pour que tous les symboles publics soient documentés et SemVer-stables.
- Créer un sous-package `sdk/` qui ajoute des helpers haut-niveau (`sdk.Quick`, `sdk.Flow`).
- Séparer les types internes (commencent par lettre minuscule) des types API (majuscule, commentés).
- API sketch :
  ```go
  import "github.com/MakFly/ghostchrome/sdk"

  b, _ := sdk.NewBrowser(sdk.Options{Headless: true, Stealth: true})
  defer b.Close()

  page, _ := b.Navigate("https://example.com")
  refs, _ := page.Extract(sdk.LevelSkeleton)
  _ = page.ClickByText("Learn more")
  ok := page.AssertURLContains("/iana/")
  ```
- Rétro-compatibilité CLI : la CLI reste, `cmd/` continue à utiliser `engine/` et `sdk/`.

**Acceptance** :
- `go test ./sdk/...` passe sur un exemple end-to-end.
- Tutorial `docs/v1.0.0/examples/go-quickstart.md` compile et tourne.
- Pas de type exporté sans godoc.

### 4.2 [R2] Binding Node.js

**Description** : Package `npm install ghostchrome` avec API TypeScript.

**Scope** :
- Wrap le binaire ghostchrome via `child_process.spawn` pour chaque commande.
- Détection du binaire : (a) à côté du module (installé par postinstall script), (b) dans le PATH, (c) téléchargement au premier run.
- API :
  ```ts
  import { Browser } from 'ghostchrome'

  const browser = await Browser.launch({ stealth: true })
  const page = await browser.navigate('https://example.com')
  await page.clickByText('Learn more')
  await page.assertUrlContains('/iana/')
  await browser.close()
  ```
- TypeScript declarations exhaustives, JSDoc sur chaque méthode.
- Support de `--format json` en interne pour parser les sorties.

**Acceptance** :
- `bun test` + `npm test` sur le package passent.
- `npm install` télécharge le bon binaire pour la plateforme active.
- Playwright migration snippet (10 lignes) fonctionne.

### 4.3 [R3] Binding Python

**Description** : Package `pip install ghostchrome` avec API pythonic.

**Scope** :
- Stratégie identique à R2 : wrap binary via `subprocess`.
- API :
  ```python
  from ghostchrome import Browser

  with Browser.launch(stealth=True) as browser:
      page = browser.navigate('https://example.com')
      page.click_by_text('Learn more')
      assert page.url_contains('/iana/')
  ```
- Type hints (PEP 484), compat Python 3.9+.
- `pyproject.toml` avec `setuptools-rust`-style binary fetching (ou wheels par plateforme).

**Acceptance** :
- `pytest` passe, `mypy` strict passe.
- `pip install` sur linux/mac récupère le binaire approprié.

### 4.4 [R4] `ghostchrome record` — codegen

**Description** : Enregistrer les actions utilisateur et générer un script `batch` réutilisable.

**Scope** :
- Active `Input.dispatchMouseEvent` + `Input.dispatchKeyEvent` listeners via CDP.
- Écoute `DOM.inputEvent` / `Page.frameNavigated` pour capturer navigate / type / click.
- Pour chaque action : résout le locator sémantique le plus robuste (role+name > label > text > CSS).
- Output : un `.gcb` batch script avec commentaires temporels :
  ```
  # Recorded 2026-04-20T14:30 — https://github.com
  navigate https://github.com
  # clicked "Sign in" at 2.1s
  click --by-role link --by-name "Sign in"
  type --by-label "Username" "kev"
  ...
  ```
- Mode interactif : `ghostchrome record --output flow.gcb` lance Chrome non-headless, l'utilisateur interagit, Ctrl-C termine.

**Acceptance** :
- Record + replay sur GitHub login : le script généré est idempotent.
- Actions couvertes : navigate, click, type, scroll, select.
- Locators générés préfèrent role+name (plus robuste que CSS).

### 4.5 [R5] MCP server natif

**Description** : `ghostchrome mcp` démarre un serveur stdio qui expose chaque commande comme un MCP tool.

**Scope** :
- Implémente la [spec MCP](https://modelcontextprotocol.io) stdio transport.
- Chaque cobra command → un `Tool` avec schema JSON auto-généré depuis les flags.
- Batching : expose un tool composite `ghostchrome_flow` qui accepte un tableau de steps.
- Config Claude Code :
  ```json
  {
    "mcpServers": {
      "ghostchrome": {
        "command": "ghostchrome",
        "args": ["mcp"]
      }
    }
  }
  ```

**Acceptance** :
- Claude Code reconnaît le serveur, liste les tools, les appelle sans erreur.
- Bench : overhead stdio < 150 ms vs invocation CLI directe.

### 4.6 [R6] Documentation

**Description** : Site statique avec quickstarts, reference, examples.

**Structure** :
```
docs/
├── README.md                         (landing)
├── quickstart/
│   ├── cli.md
│   ├── go.md
│   ├── node.md
│   └── python.md
├── guides/
│   ├── agent-usage.md                (pour LLM agents)
│   ├── frontend-validation.md        (FE devs)
│   ├── scraping.md                   (data extraction)
│   └── migration-from-playwright.md
├── reference/
│   ├── commands/                     (one file per verb)
│   └── sdk/                          (godoc + node + python)
├── examples/
│   └── [15+ use cases]
└── v1.0.0/                           (ce dossier)
    ├── PRD.md
    └── SPRINT.md
```

**Acceptance** :
- GitHub Pages actif à `https://makfly.github.io/ghostchrome/`.
- 100 % des commands ont une page reference.
- 15 exemples tournables en one-liner copiés.

### 4.7 [R7] Packaging

**Scope** :
- Homebrew tap `makfly/ghostchrome` (déjà retiré en v0.1.0, à remettre).
- npm package (`ghostchrome`).
- PyPI package (`ghostchrome`).
- Install script `curl | sh` (déjà existant, à maintenir).
- GitHub Releases avec 4+ binaires pré-buildés : linux amd64, linux arm64, darwin amd64, darwin arm64. Windows amd64 best-effort.

**Acceptance** :
- `brew install makfly/ghostchrome/ghostchrome` fonctionne sur macOS.
- `npm i -g ghostchrome && ghostchrome --version` fonctionne.
- `pip install ghostchrome && ghostchrome --version` fonctionne.
- CI GitHub Actions produit les 5 binaires sur chaque tag `v*`.

### 4.8 [R8] SemVer v1.x stability guarantee

- CHANGELOG.md publique avec section Keep a Changelog.
- Breaking changes refusées dans v1.x sauf security patch.
- Commits flag `breaking:` → refusés jusqu'à v2.0.0.

---

## 5. Dépendances et risques

### 5.1 Dépendances

| Dépendance | Usage | Risque |
|---|---|---|
| Rod v0.116.2 | Core | Stable, peu de risque |
| Cobra | CLI | Stable |
| MCP spec | R5 | Spec officielle, stable |
| Node v20+ | R2 | Choix utilisateur |
| Python 3.9+ | R3 | Choix utilisateur |

### 5.2 Risques

| Risque | Impact | Mitigation |
|---|---|---|
| MCP spec évolue pendant dev | Moyen | Pinner la version spec et ajouter PR de bump en suivi |
| Godoc revealing trop de types internes | Moyen | Audit du pkg engine/ avant release |
| npm postinstall cassé sur corporate proxy | Haut | Fallback téléchargement manuel documenté |
| Codegen produit des flows fragiles | Haut | Test sur 5 sites réels avant merge |
| Breaking changes non détectées | Moyen | `go vet` + `gorelease` + tests snapshot API |
| Windows support imparfait | Bas (non cible principale) | Best-effort, non bloquant pour v1.0.0 |

---

## 6. Métriques post-launch (30 jours)

- **Adoption** : ≥ 500 installs cumulés (brew + npm + pip).
- **Engagement** : ≥ 50 stars GitHub, ≥ 10 issues avec label `use-case` (pas de bug).
- **Stabilité** : 0 bug critique (crash, data loss), < 5 bugs majeurs.
- **Docs** : moyenne temps-sur-page > 2 min sur quickstart (via plausible.io ou équiv).
