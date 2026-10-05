# Feature: Diagnostics — anotaciones por buffer con gutter de números de línea

## Description
El usuario pide anotaciones por buffer: una lista de {línea, mensaje,
severidad} por documento, renderizada con gutter de **números de línea +
marcador de severidad** (el editor no tenía números de línea; esta feature los
agrega) y **underline** en las líneas con diagnóstico. El mensaje de la línea
del cursor se muestra en la barra de estado. El **proveedor es el backend de
scripting Lua** (hito 2): `tcode.buffer.lineCount()/line(n)` para leer por
línea y `tcode.diagnostics.set(...)` para depositar las anotaciones (una
extensión/linter corre en `onDidSaveBuffer`).

## Hitos
- **A (render + estado):** `view.Diagnostic{Line, Message, Severity}` con
  `SeverityInfo/Warning/Error`; el EditorView guarda los diagnostics por buffer
  (`SetDiagnostics`); gutter de números (ancho según dígitos del LineCount)
  con marcador por severidad; underline en líneas marcadas; 3 roles de tema
  (`diagError/diagWarning/diagInfo`) en las 6 paletas y en `theme.json`; el
  controller muestra "linea N: mensaje" al mover el cursor (syncDiagStatus en
  el path de teclas del editor). Ajuste de viewport: el texto empieza tras el
  gutter; mouse-hit-testing descuenta el gutter.
- **B (proveedor Lua):** ScriptAPI gana LineCount/Line del buffer activo y
  SetDiagnostics; Lua: `tcode.buffer.lineCount()` y `tcode.buffer.line(n)`
  (1-indexed) + `tcode.diagnostics.set(lista)` y `clear()`; el App implementa
  (activo; límite documentado: anotaciones al buffer activo). Ejemplo
  end-to-end: extensión con main.lua que llena diagnostics en onDidSaveBuffer.

## Tasks (hito A)
- [x] `view/diagnostics.go`: Severity, Diagnostic, estado del EditorView,
  `SetDiagnostics`, helper de la línea del cursor y de la fila a marcar
  <!-- id: 0 -->
- [x] EditorView: gutter con número + marcador (primera fila de la línea
  lógica; continuaciones con blanco), ajuste de viewport.Resize y del Draw
  (texto desde x=gutter), underline en líneas con diagnóstico <!-- id: 1 -->
- [x] Tema: roles DiagError/DiagWarning/DiagInfo en las 6 paletas y en
  LoadTheme (`diagError`/`diagWarning`/`diagInfo`) <!-- id: 2 -->
- [x] Controller: `syncDiagStatus()` (mensaje "línea N: msg (severidad)" en la
  barra cuando el cursor está en una línea con diagnóstico) llamado en el path
  de teclas del editor <!-- id: 3 -->
- [x] Mouse: hit-testing del editor descuenta el gutter; ajustes de los tests
  de posiciones del editor <!-- id: 4 -->
- [x] Verificación y commit (hitos A y B juntos): `b5ee658` <!-- id: 5 -->

## Design decisions
- **Gutter siempre presente** (aunque no haya diagnostics): el editor gana
  números de línea, que es la base pedida; el ancho se calcula con los dígitos
  del `LineCount` (estable por buffer) + separador.
- **El color de severidad vive en el marcador del gutter y en el mensaje**; el
  texto de la línea solo se subraya (no pisa los colores de sintaxis).
- **Mensaje en la barra de estado** (sin hover en terminal): la línea del
  cursor con diagnóstico sobreescribe el rol del archivo con el mensaje.
- **Proveedor = scripting** (hito B): `diagnostics.set` reemplaza las
  anotaciones del buffer activo; límites documentados (buffer activo;
  `content`/`line` por línea, sin `onDidChangeText` por diseño).