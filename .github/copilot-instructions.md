# Copilot Instructions for os-image-composer

## Architecture Overview

OS Image Composer builds custom Linux images from pre-built packages. Key components:

- **Provider** (`internal/provider/`) - Orchestrates builds per OS (azl, elxr, emt, ubuntu). Implements `Provider` interface with `Init`, `PreProcess`, `BuildImage`, `PostProcess` methods
- **Image makers** (`internal/image/`) - Creates output formats: `rawmaker/`, `isomaker/`, `initrdmaker/`
- **Chroot** (`internal/chroot/`) - Isolated build environments with package installers for `deb/` and `rpm/`
- **Config** (`internal/config/`) - Template loading, merging defaults with user templates, validation
- **OsPackage** (`internal/ospackage/`) - Package utilities: `debutils/`, `rpmutils/` for dependency resolution

Data flow: CLI → Config loads template → Provider.Init → Provider.PreProcess (downloads packages) → Provider.BuildImage (creates chroot, installs packages, generates image) → Provider.PostProcess

## Build and Test

Always use **Earthly** for builds and testing. Do not run `go build` or `go test` directly.

| Task | Command |
|------|---------|
| Build | `earthly +build` |
| Test (fast) | `earthly +test-quick` |
| Test (coverage) | `earthly +test` |
| Lint | `earthly +lint` |

Coverage threshold: **64.2%** (enforced in CI)

## Adding a New OS Provider

1. Create package in `internal/provider/{osname}/`
2. Implement `provider.Provider` interface (see `internal/provider/provider.go`)
3. Register in `cmd/os-image-composer/build.go` switch statement
4. Add default configs in `config/osv/{osname}/`
5. Create example templates in `image-templates/`

## Testing Patterns

- Use **table-driven tests** with `t.Run()` for multiple cases:
```go
tests := []struct{ name string; input string; want error }{...}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) { ... })
}
```
- Test file naming: `*_test.go` in same package
- Reset shared state in tests (see `resetBuildFlags()` pattern in `cmd/os-image-composer/build_test.go`)

## Error Handling

- **Always wrap** with context: `fmt.Errorf("failed to X: %w", err)`
- Use named returns with defer for cleanup (see `docs/architecture/os-image-composer-coding-style.md`)
- Never ignore errors with `_`

## Code Style

- Imports: stdlib → third-party → local (blank line separated)
- Run `earthly +lint` (uses golangci-lint)
- Shell scripts: `set -euo pipefail`
- See `docs/architecture/os-image-composer-coding-style.md` for full guide

## Git Commits & PRs

- Sign commits: `git commit -S`
- Conventional commits: `type(scope): description` (feat, fix, docs, test, refactor, chore)
- **Always use** `.github/PULL_REQUEST_TEMPLATE.md` for PRs
- Branch prefixes: `feature/`, `fix/`, `docs/`, `refactor/`

## Key Files

- `image-templates/*.yml` - Example image templates
- `config/osv/` - OS-specific default configurations
- `internal/config/config.go` - `ImageTemplate` struct definition
- `internal/provider/provider.go` - Provider interface
- `docs/architecture/` - ADRs and design docs
