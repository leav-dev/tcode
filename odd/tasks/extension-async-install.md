# Feature: Instalación de extensiones en segundo plano (no bloquea el editor)

## Description
Instalar o actualizar una extensión desde la ventana (`Enter` → confirmación
`s/N`) corre `ext.InstallByID` / `ext.UpdateAll` sincrónico dentro de
`handlePromptKey`, en el hilo del loop de eventos. Eso es un `git clone` de
segundos con el editor congelado. El usuario quiere que la instalación corra
en un proceso aparte (goroutine background) para seguir editando.

## Decisiones de diseño
- Reusar el patrón existente: goroutine + `screen.PostEvent(tcell.NewEventInterrupt(...))`,
  ninguna goroutine toca la UI (como `prefetchExtensions` / `extSnapshotEvent`).
- El prompt se cierra de inmediato con toast "en segundo plano…"; al terminar
  el job avisa con toast éxito/error y repinta la ventana (`refreshExtData` +
  `reloadExtensions`).
- Borrar (`RemoveNamespaced`/`Remove`) es disco local y queda sincrónico.
- Un solo job de escritura a la vez (`extJobRunning`): si hay otro en curso se
  avisa en la barra y no se encola.

## Tasks
- [x] `internal/controller/app.go`: tipos `extJobKind`/`extJobEvent`, campos `extJobSeq`/`extJobRunning`, despacho en `case *tcell.EventInterrupt` + `handleExtJob` <!-- id: 0 -->
- [x] `internal/controller/app.go`: `promptInstallExtension`/`installExtension` async (goroutine + PostEvent, captura providers/userRoot/fetcher por valor) <!-- id: 1 -->
- [x] `internal/controller/app.go`: `promptUpdateExtension` async por el mismo camino <!-- id: 2 -->
- [x] Tests: el prompt retorna sin bloquear, el evento completa (toast + refresh + reload), job concurrente se rechaza <!-- id: 3 -->
- [x] Docs: README + docs/extension-system.md (instalación en segundo plano) <!-- id: 4 -->

## Evidence
- Bloqueo hoy: `installExtension` (`app.go:~2990`) llama `ext.InstallByID` sincrónico
  tras `showToast("Instalando…")`; `promptUpdateExtension` llama `ext.UpdateAll`
  sincrónico dentro de la acción de `openPrompt` (corre en `handlePromptKey`).
- Patrón a reusar: `prefetchExtensions` + `handleExtSnapshot` via `EventInterrupt`.
