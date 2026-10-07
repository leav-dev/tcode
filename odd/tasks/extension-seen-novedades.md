# Feature: Control de novedades ya listadas (sin falsos positivos)

## Description
El aviso de arranque (`N actualizaciones, M novedades`) reporta TODAS las
disponibles no instaladas en cada arranque. Si el usuario ya vio la novedad
y decidió no instalarla, el próximo arranque la vuelve a anunciar: falso
positivo. Se quiere control de extensiones no instaladas ya listadas.

## Decisión de usuario (2026-10-07)
Solo nuevas no vistas: el arranque solo avisa novedades nunca vistas;
las ya listadas no repiten aviso. Una versión nueva de la misma
extensión SÍ vuelve a avisar (es realmente nueva).

## Decisiones de arquitectura (del agente)
- **Clave de visto**: `proveedor/id@versión` — un bump de versión re-avisa,
  la misma versión no.
- **Store**: `internal/ext/seen.go` puro + persistencia en
  `~/.tcode/extensions-seen.json` (estado, no config: como `providers.json`,
  no como `config.json`). JSON tolerante: ausente/corrupto = vacío.
- **Filtro**: `FilterUnseen(available, seen)` devuelve solo no vistas.
  `Snapshot.Available()` sigue devolviendo todo (la ventana Disponible
  lista todo); solo el AVISO usa el filtro.
- **Marcado**: tras derivar en `handleExtSnapshot` (vía automática, no manual),
  se persiste el set actual como visto. Instalar poda la entrada.
- **Poda**: al persistir se dropean claves cuya extensión ya está instalada
  o ya no está en el catálogo, para que el archivo no crezca sin cota.

## Tasks
- [x] `internal/ext/seen.go`: `SeenKey`, `FilterUnseen`, `MarkSeen`, `LoadSeenFile`, `SaveSeenFile`, `PruneSeen` <!-- id: 0 -->
- [x] `internal/ext/seen_test.go`: no vista pasa, vista no pasa, bump re-avisa, prune <!-- id: 1 -->
- [x] `internal/controller/app.go`: `handleExtSnapshot` filtra aviso por unseen y persiste; path inyectable para tests <!-- id: 2 -->
- [x] Tests controller (`ext_seen_test.go`): segundo snapshot igual no avisa; bump sí avisa <!-- id: 3 -->
- [x] Docs: `docs/extension-system.md` nota de comportamiento <!-- id: 4 -->

## Evidence
- Aviso hoy: `handleExtSnapshot` cuenta `len(a.extAvailable)` sin memoria (`internal/controller/app.go`).
- Derivación: `Snapshot.Available()` = catálogo menos instalado, sin estado (`internal/ext/snapshot.go`).
- Persistencia análoga: `providers.json` vía `LoadProviders/SaveProviders` (`internal/ext/provider.go`).
