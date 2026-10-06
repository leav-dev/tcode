# Feature: Relanzar la validación de actualizaciones desde el editor

## Description
El chequeo de actualizaciones de extensiones corre solo al arrancar
(`prefetchExtensions` en segundo plano). El usuario quiere poder relanzarlo a
mano desde el editor, sin reiniciar, para validar si hay actualizaciones.

## Decisiones de diseño
- Disparo: tecla `r` con la ventana de extensiones abierta (cualquier pestaña).
  La ventana no se cierra y el editor sigue respondiendo.
- Reusa `prefetchExtensions` (goroutine + `EventInterrupt`, seq descarta lo
  viejo): no hay camino nuevo de lectura.
- Si hay un job de escritura en vuelo (instalando/actualizando), no se relanza:
  se avisa en la barra (leer el disco a mitad de una escritura daría un estado
  partido).
- El resultado manual SIEMPRE se reporta con toast (éxito con conteos, error, o
  "Extensiones al día"); el arranque sigue silencioso cuando no hay nada.
- Borrar/sincrónico no cambia.

## Tasks
- [x] `internal/view/ext_manager.go`: `ExtIntentRefresh` + tecla `r` (cualquier pestaña, no cierra) <!-- id: 0 -->
- [x] `internal/controller/app.go`: `refreshExtensions()` + flag `extRefreshManual` + reporte en `handleExtSnapshot` <!-- id: 1 -->
- [x] Tests: `r` propone refresh sin cerrar; relanzamiento manual reporta (con novedades y al día); con job en vuelo se rechaza <!-- id: 2 -->
- [x] Docs: README + docs/extension-system.md (tecla `r`) <!-- id: 3 -->

## Evidence
- Lectura hoy: solo `prefetchExtensions` al arrancar (`app.go`); la ventana
  deriva de `extSnapshot` sin releer.
- `handleExtSnapshot` solo avisa si hay algo o si hubo error; sin novedades es
  silencioso (bien para el arranque, mal para un relanzamiento manual).
