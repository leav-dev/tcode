# Feature: Scroll centrado del cursor en el editor

## Description
El usuario quiere que la línea del cursor **tienda a quedar al centro del
viewport** (vertical), *solo cuando el archivo tiene suficiente contenido
arriba y abajo*; cerca del inicio o del final (o con un documento más corto
que la pantalla), la línea queda pegada al borde, no en la mitad. Hoy el
editor hace scroll **mínimo** (mueve `TopLine` solo cuando el cursor se sale
de la ventana).

Comportamiento nuevo en `ensureCursorVisible` (solo la parte vertical):
- `target := cursor.Line - Height/2`, `TopLine = clamp(target, 0, max(0,
  LineCount - Height))`.
- Con Word wrap: el centrado es por LÍNEA lógica (modelo nano v1 del repo,
  documentado); si tras el centrado la fila visual del cursor quedara fuera
  por la diferencia de filas del wrap, la lógica mínima vieja corre como red
  de seguridad (nunca se pierde el cursor). El scroll horizontal (LeftColumn)
  no cambia.
- El ÁRBOL (FileBrowser) mantiene su scroll mínimo: el cambio es del editor.

## Tasks
- [x] `EditorView.ensureCursorVisible`: centrado vertical por línea lógica con
  clamp; red de seguridad de filas visuales; horizontal intacto <!-- id: 0 -->
- [x] Ajustar los tests del editor que esperan scroll mínimo (los del
  viewport/top de cursor_test.go, wrap_render_test.go y editor_view_test.go)
  a la nueva semántica, sin cambiar el resto de la aserción <!-- id: 1 -->
- [x] Tests nuevos: cursor en el medio → top = línea - mitad; en la 1ª línea
  de un doc largo → top 0 (borde); en la última → top = LineCount - Height
  (borde abajo); doc más corto que el viewport → top 0 <!-- id: 2 -->
- [x] Verificación y commit: `5b79a72` <!-- id: 3 -->

## Design decisions
- **Por línea lógica (no por fila visual):** coherente con el modelo de scroll
  del repo (nano v1) y evita recalculcar filas envueltas en cada movimiento;
  la red de seguridad cubre el caso de wrap extremo.
- **El clamp resuelve el criterio del usuario:** sin líneas arriba → top 0
  (cursor arriba); sin líneas abajo → top máximo (cursor abajo); doc corto →
  top 0. "Centrar" es solo el objetivo, no una ley.
- **Sin toggle:** comportamiento estándar; si el usuario pide el mínimo
  después, se agrega una opción en la ventana de configuración.