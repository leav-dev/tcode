# Feature: Save As dedup (último pendiente de pestañas)

## Description
Pendiente anotado en `tab-menu-session.md`: "un Save As a una ruta ya abierta
en otra pestaña no deduplica entre buffers". Guardar como a una ruta abierta
dejaba dos buffers sobre el mismo archivo (uno con la ruta vieja re-apuntada,
otro con la ruta original).

Resolución: tras un `SaveAs` exitoso, si la ruta destino ya estaba abierta en
otra pestaña, el buffer recién guardado **se cierra** (quedó limpio: su
documento ya está en disco) y el existente queda **activo y recargado** desde
disco para ver lo que el Save As acaba de escribir. El flujo declina en dos
casos: si la ruta no estaba abierta (no hay nada que consolidar) y si el target
del Save As ya no es la pestaña activa (cerrar la activa cerraría la pestaña
equivocada — caso raro del prompt, cubierto por defensa).

Nota honesta sobre ediciones: si el buffer existente tenía cambios sin guardar,
el Save As ya los pisó en disco (guardar a esa ruta es una decisión explícita);
recargarlo refleja la realidad, no oculta la pérdida.

## Tasks
- [x] `App`: `bufferAtPath(path)` + `dedupSaveAsConsolidates(target, path)`
  (cierre del duplicado con `closeTab`, `Reload` del existente, `ClampCursor`);
  `saveAs` llama al helper tras el guardado exitoso y anuncia la consolidación
  <!-- id: 0 --> · `5caa99f`
- [x] Tests: consolidación (una pestaña, la existente activa y limpia), destino
  nuevo sin efecto, target no activo declina <!-- id: 1 --> · `5caa99f`
- [x] Verificación: `go vet`, `gofmt`, suite completa sin fallos nuevos vs base
  <!-- id: 2 -->

## Design decisions

### El helper se prueba sin el SaveAs real
En este host, Windows impide escribir sobre un archivo con una sección mapeada
abierta (`ERROR_USER_MAPPED_FILE`); el SaveAs de integración (a una ruta
abierta) no se puede ejecutar acá. El helper aísla exactamente la parte que el
controlador aporta —buscar la ruta abierta, cerrar el duplicado, activar y
recargar el existente— y se prueba directo; el SaveAs del modelo (escribir y
cambiar de ruta) ya está cubierto por los tests del modelo. En entornos donde
el FS lo permite, el flujo completo vale igual: `saveAs` llama al mismo helper.

### Cerrar el recién guardado, no el existente
El target del Save As quedó limpio (el modelo fija `savedAt` tras escribir);
cerrar su pestaña con `closeTab` reutiliza todo el manejo ya probado (emite
`onDidCloseBuffer`, limpia `editors`/`forceSave`, reencuadra pestañas). El
existente se recarga (¡el feature de recarga recién entregado!) para mostrar el
contenido nuevo. La alternativa de cerrar el existente pisaría ediciones sin
aviso; la elegida recarga, que es la verdad del disco.

## Falsificación
1. Sin `dedupSaveAsConsolidates` → los tests no compilan.
2. Consolidación sin `Reload` del existente → el contenido viejo quedaría en
   memoria (el test no lo detecta sin disco; queda como riesgo documentado).

## Evidence
- **RED:** no compilaban `bufferAtPath`/`dedupSaveAsConsolidates`.
- **GREEN:** 3 tests (consolidación, destino nuevo, target no activo).
- **Verificación:** delta completo vs base: 0 fallos nuevos, 0 arreglados;
  `go build`/`go vet` límpios.
- **Commit:** `5caa99f`.