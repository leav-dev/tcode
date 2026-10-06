# Feature: auto-actualización del editor (aviso + tcode update)

## Description
El editor no detecta sus propias actualizaciones. Se pide: chequeo en segundo
plano al arrancar (aviso en barra, sin descargar nada solo) + subcomando
`tcode update` que instala el release latest.

## Decisiones de diseño
- Versión propia: `-X main.version` en release.yml > `debug.ReadBuildInfo` (go
  install) > "" (dev). Sin versión conocida no hay aviso (un dev naggearía
  siempre), pero `tcode update` funciona igual.
- Chequeo: `api.github.com/repos/leav-dev/tcode/releases/latest` en goroutine
  del controller (mismo patrón EventInterrupt que extensiones); tolerante y sin
  prompt. Comparación semver numérica propia, sin deps.
- `tcode update`: descarga el asset `tcode-<os>-<arch>[.exe]` + verifica sha256
  contra checksums.txt + reemplaza `os.Executable()` (dice la ruta). Sin Go
  necesario, igual que install.sh.
- Tests sin red (httptest con base inyectable).

## Tasks
- [x] `internal/update/update.go`: versión, Compare, CheckLatest, DownloadAndInstall <!-- id: 0 -->
- [x] `main.go`: subcomando `update` <!-- id: 1 -->
- [x] `.github/workflows/release.yml`: ldflags con `-X main.version` <!-- id: 2 -->
- [x] `internal/controller/app.go`: prefetch del chequeo + aviso en barra <!-- id: 3 -->
- [x] Tests (sin red) + README <!-- id: 4 -->

## Evidence
- Commit `3e6cd5c` pusheado; `go vet` + `go test ./... -race` verdes.
- La versión viaja en `internal/update.version` (ldflags del release).

## Evidence
- Assets: `tcode-<os>-<arch>[.exe]` + checksums.txt en releases/latest/download.
- CLI usa flags `--...`; `update` será subcomando posicional (pedido explícito).
- `ScriptAPI`/ext no se tocan: es chequeo del editor, no de extensiones.
