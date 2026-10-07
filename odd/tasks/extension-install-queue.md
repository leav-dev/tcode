# Feature: Cola de instalación de extensiones (una a la vez)

## Description
Pedir una instalación mientras otra está en vuelo se rechazaba con "ya hay una
instalación en curso": el usuario tenía que esperar mirando la barra para pedir
la siguiente. Ahora la instalación pedida queda EN COLA y al terminar la vigente
arranca sola la siguiente, siempre DE A UNA (un solo `git clone` a la vez para
mantener el bajo consumo).

## Decisiones de diseño
- La cola vive en el App (`extInstallQueue []extPendingInstall`, un pedido por
  entrada con su origen: ventana o catálogo) y solo corre en el hilo de los
  eventos: `installExtension` encola, `handleExtJob` drena al terminar cada
  job (instalación o actualización, que también toma el lock).
- Sin paralelismo a propósito: dos escrituras concurrentes sobre la misma raíz
  se pisarían y N clones a la vez suben el consumo. La cola es secuencial.
- Deduplicación por `Ref`: pedir dos veces la misma extensión encolada avisa
  "ya está en cola" en vez de duplicar.
- Tope de 32 encoladas: una cola sin tope es memoria sin cota; 32 sobra para
  extensiones y el aviso dice que espere a que se vacíe.
- La actualización (`UpdateAll`) y la revalidación manual (`r`) siguen
  rechazándose con un job en vuelo: tocan la misma raíz y no son encolables.
- El panel del catálogo usa LA MISMA cola (`extPendingInstall` con `catalogID`):
  un solo funnel secuencial, sin instalación sincrónica en loop. Cada entrega
  marca su entrada como Installed con su toast.

## Tasks
- [x] `internal/controller/app.go`: campo `extInstallQueue` + tope <!-- id: 0 -->
- [x] `internal/controller/app.go`: `installExtension` encola con dedup en vez de rechazar <!-- id: 1 -->
- [x] `internal/controller/app.go`: `handleExtJob` drena la cola + `drainInstallQueue` <!-- id: 2 -->
- [x] Tests: pedir con job en vuelo encola; la cola se drena de a una; duplicada no duplica <!-- id: 3 -->
- [x] Docs: README (cola en vez de rechazo) <!-- id: 4 -->
- [x] Cola única: `installCatalogEntries` async por `extJobCatalogInstall` <!-- id: 5 -->
- [x] Tests panel: batch en cola en orden + cola cruzada ventana/catálogo <!-- id: 6 -->

## Evidence
- Rechazo hoy: `installExtension` (`app.go`) devuelve "ya hay una instalación
  en curso" si `extJobRunning`; `TestExtensionWindowRejectsConcurrentJob` lo
  afirma.
- Lock existente: `extJobSeq`/`extJobRunning` + entrega por `EventInterrupt`
  (`extension-async-install.md`).
