# Versioning Rules

ghostchrome follows SemVer (vMAJOR.MINOR.PATCH). Tag and release via `git tag vX.Y.Z && git push --tags` which triggers CI/CD.

## When to bump

| Change type | Version bump | Examples |
|---|---|---|
| New command | MINOR | `ghostchrome upload`, `ghostchrome scroll` |
| New flag on existing command | MINOR | `--proxy`, `--user-agent` |
| Bug fix | PATCH | Fix eval async, fix ref resolution |
| Performance improvement | PATCH | Faster extraction, smaller binary |
| Breaking CLI change | MAJOR | Rename command, change output format, remove flag |
| New extraction level | MINOR | Add `minimal` level |
| Dependency update (Rod, Cobra) | PATCH | Unless it changes behavior |

## Release checklist

1. Update version in code if hardcoded anywhere
2. `go build` + test on example.com + a real app
3. `git tag vX.Y.Z` with annotated tag
4. `git push --tags` → CI builds + creates GitHub Release
5. Update CHANGELOG.md if it exists

## Commit prefixes

- `feat:` → triggers MINOR bump consideration
- `fix:` → triggers PATCH bump consideration  
- `breaking:` or `feat!:` → triggers MAJOR bump consideration
- `chore:`, `docs:`, `ci:` → no version bump
