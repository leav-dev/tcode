# Chore: módulo instalable con un solo comando

## Description
Hoy instalar requiere shallow clone + build manual porque `go.mod` declara el
módulo como `tcode` a secas (sin path resolvible). El objetivo: `go install
github.com/leav-dev/tcode@latest` como comando único.

## Tasks
- [x] Commitear el trabajo pendiente de la sesión en unidades (features ya testeadas) <!-- id: 0 -->
- [x] Renombrar módulo a `github.com/leav-dev/tcode` (go.mod + imports) + README <!-- id: 1 -->
- [x] Tag `v0.1.0`, push main + tag, verificación end-to-end en env limpio <!-- id: 2 -->

## Evidence
- Commits pusheados: `efeadf6` (features) + `6c11a4c` (rename); tag `v0.1.0`.
- E2E: `GOBIN`+`HOME` temporales, `go install ...@latest` descargó v0.1.0,
  compiló y `--list-extensions` dio exit 0.

## Evidence
- 23 archivos `.go` importan `tcode/internal/...`; solo código + go.mod cambian.
- Repo público (el shallow clone https funcionó sin auth) → el proxy de Go lo resuelve.
