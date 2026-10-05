# Release Workflow + Homebrew Tap

## Objetivo
Configurar releases automáticos con GitHub Actions + GoReleaser, y publicar tcode en un Homebrew tap público para instalación con `brew install tcode`.

## Decisiones de producto
- **Release trigger:** Tags manuales (`v*`) — el usuario decide cuándo releasear
- **Homebrew tap:** Tap público en `leav-dev/homebrew-tcode`
- **Plataformas:** linux/darwin × amd64/arm64
- **CI:** Workflow separado para tests en cada push/PR

## Tareas

- [ ] 1. Crear repo `homebrew-tcode` en GitHub (tap público)
- [ ] 2. Crear `.goreleaser.yml` con builds, changelog y Homebrew tap
- [ ] 3. Crear `.github/workflows/release.yml` (trigger en tags `v*`)
- [ ] 4. Crear `.github/workflows/ci.yml` (tests en push/PR)
- [ ] 5. Documentar instalación con Homebrew en README
- [ ] 6. Commit + push de los archivos de configuración

## Evidencia
- _Pendiente: commit hash de cada tarea_

## Estado
🔄 En progreso
