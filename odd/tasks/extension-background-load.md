# Feature: Lectura de extensiones en segundo plano (fluidez)

## Description
Abrir la ventana de extensiones bloquea el editor ~7 s: `CheckUpdates` y
`AvailableExtensions` leen cada uno el catálogo del proveedor, y cada lectura es un
partial clone contra GitHub (**medido: 3,84 s**; las dos juntas 7,05 s). El usuario
quiere que esa lectura corra **al inicio, en segundo plano**, y que el editor se sienta
fluido.

## Decisiones de diseño (confirmadas con el usuario)

| # | Decisión |
| --- | --- |
| 1 | La lectura cara corre **al inicio de la app, en una goroutine** (no bloquea el arranque) |
| 2 | **Sin prompt de arranque**: al llegar los datos se avisa en la **barra de estado** (`N actualizaciones, M novedades — Ctrl+P → Extensiones`) y se gestiona todo desde la ventana |
| 3 | La ventana abre **instantánea** con los datos cacheados; si todavía no llegaron, muestra "cargando…" |

## Decisiones de arquitectura (del agente)

- **Una sola lectura del proveedor**: `ext.LoadAll(providers, userRoot, fetcher)` lee el
  catálogo de cada proveedor UNA vez y devuelve un `Snapshot` con los proveedores, los
  catálogos por proveedor y las instaladas. Las actualizaciones y las novedades se
  **derivan** del snapshot (`Snapshot.Updates()`, `Snapshot.Available()`), no se vuelven
  a leer. Es el arreglo del I/O redundante: 7,05 s → 3,84 s.
- **El snapshot se cachea**: la ventana y el aviso de arranque lo consumen. Tras una
  acción (instalar/actualizar/borrar) **no se re-lee el proveedor**: el catálogo no
  cambió, solo la lista local de instaladas. Se re-lee `ext.List` (local, rápido) y se
  vuelve a derivar del mismo snapshot. Eso hace instantáneas las acciones.
- **Recarga segura de extensiones**: con el chequeo en segundo plano, aplicar cambios ya
  no puede correr antes de `loadExtensions`, así que hace falta poder recargar:
  `Registry.Unregister(id)` + `Manager.Reload(exts)` (desregistra solo los comandos que
  registraron las extensiones, no los built-ins). **`Reload` descarta también los hosts
  de Lua cacheados**: una extensión actualizada tiene código NUEVO, y el host cacheado
  ejecutaría el viejo.
- **Entrega desde la goroutine**: `tcell.NewEventInterrupt(snapshot)` + `screen.PostEvent`
  (thread-safe, despierta el loop). El controlador maneja `*tcell.EventInterrupt` en
  `handleEvent` y cachea el snapshot. Ninguna goroutine toca la UI directamente.
- Se elimina `checkExtensionsAndPrompt` y su prompt (decisión 2).

## Tasks
- [ ] `internal/ext/snapshot.go`: `Snapshot{Providers, Catalogs, Installed}` + `LoadAll` (una lectura por proveedor) + `Updates()` / `Available()` derivados <!-- id: 0 -->
- [ ] `internal/ext/registry.go` + `manager.go`: `Registry.Unregister(id)`; `Manager` trackea los ids que registró y `Manager.Reload(exts)` los desregistra, limpia `states`/`keymap`/`scriptHosts` y re-registra <!-- id: 1 -->
- [ ] `internal/controller/app.go`: `loadExtensions` pasa a usar `Reload` (re-llamable); prefetch en goroutine al arrancar con `PostEvent`; manejo de `EventInterrupt` (cachea + aviso en la barra); `openExtManager` usa el snapshot con estado "cargando…"; tras una acción, re-derivar del snapshot + recargar extensiones <!-- id: 2 -->
- [ ] Tests: `LoadAll` (una sola lectura — el fake cuenta las llamadas), `Updates`/`Available` derivados, `Manager.Reload` (re-registra y descarta el host Lua), el prefetch (el evento llega y cachea) <!-- id: 3 -->
- [ ] Docs: README + docs/extension-system.md (la lectura en segundo plano y el aviso) <!-- id: 4 -->

## Evidence
- Medido en este repo: `ListExtensions` 3,84 s · `CheckUpdates` 3,68 s · `AvailableExtensions` 4,00 s · **las dos juntas 7,05 s**.
- `tcell.NewEventInterrupt(data)` + `screen.PostEvent` existen en tcell v2.13.10 (`interrupt.go`, `screen.go`).
- `Manager` tiene `states`, `keymap`, `scriptHosts`; `Registry` tiene `Register`/`Has`/`Run` (sin desregistrar).
- `AddExtensions` no es idempotente: appendea `states` y el registry rechaza el re-registro.
