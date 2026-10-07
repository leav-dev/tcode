# Feature: el canal preview solo detecta previews

## Description
El binario preview lleva versión `preview-vX.Y.Z` (ldflags en release.yml),
pero el chequeo de updates mira `releases/latest` (estable) y `NeedsUpdate`
lo considera "más nuevo" por parse fallido. Resultado: el preview avisa
`tcode v0.1.2 disponible` y `tcode update` lo **degrada a estable**.

## Decisión de usuario (2026-10-07)
El preview solo debe detectar versiones nuevas del preview.

## Decisiones de arquitectura (del agente)
- **Track por versión propia**: `IsPreviewVersion` (prefijo `preview-`).
  Preview → feed de previews; estable/dev → igual que hoy.
- **Feed preview**: `CheckLatestPreview` lista `/releases` y toma el primer
  tag `preview-v*` (la API devuelve lo más nuevo primero). Misma tolerancia
  que `CheckLatest` (sin red/rate limit → `""`, sin error).
- **Compare**: `parse` pela el prefijo `preview-` antes de `v`; así
  `preview-v0.1.3 < preview-v0.1.4` numérico (hoy ambos caen a "distintas →
  -1" y hasta un downgrade avisaría). Cross-track queda numérico y
  documentado; el ruteo por canal es lo que los separa.
- **Descarga**: `UpdateTo(ctx, tag, dest, base)` con base explícita;
  `Update` delega con `downloadBase` (firma intacta para tests). La base
  preview es `.../releases/download/<tag>` vía `downloadBaseFor`.
- **Ruteo**: `runUpdate` (main.go) y `editorUpdateCheck` (controller) eligen
  feed + base según la versión propia. El aviso sigue diciendo
  `tcode update`, que ahora rutea bien.

## Tasks
- [x] `internal/update/update.go`: `IsPreviewVersion`, `parse` con `preview-`,
  `CheckLatestPreview`, `UpdateTo`, `downloadBaseFor` <!-- id: 0 -->
- [x] `internal/update/update_test.go`: preview no ve estable, preview ve bump,
  no ve downgrade, `CheckLatestPreview`, `UpdateTo` <!-- id: 1 -->
- [x] `main.go` `runUpdate`: ruteo por canal <!-- id: 2 -->
- [x] `internal/controller/app.go` `editorUpdateCheck`: ruteo por canal <!-- id: 3 -->
- [x] Docs: README (fila Versión + nota de canal en update) <!-- id: 4 -->

## Evidence
- `CheckLatest` → `/releases/latest` (estable); `downloadBase` → `latest/download`.
- `parse("preview-v0.1.3")` no tiene números antes del `-` → ok=false →
  `NeedsUpdate` devuelve true contra cualquier estable distinto.
- Versión del binario: `-X .../internal/update.version=${{ github.ref_name }}`.
