# Feature: Reveal del archivo activo en el explorador

## Description
Shift+Tab devuelve el **foco** al selector de archivos, pero el cursor del
árbol queda donde estaba: si el archivo que se está editando está en un
subdirectorio colapsado, no se ve —el usuario pide que al volver al panel el
selector señale el **archivo activo**.

El revelead es cooperativo, respetando el lazy loading de la constitución: la
vista no lee el filesystem. `FileBrowser.Reveal(path)` busca el nodo en el
aplanado; si su ancestro visible más profundo está colapsado, devuelve
`(false, dirPath)` y el controlador lee ese dir (`readEntries`) y lo expande
con `ExpandDir(path, entries)` (expansión por RUTA, independiente del cursor,
a diferencia de `SetChildren`). El controlador itera un nivel por pasada hasta
que el nodo queda visible y seleccionado, o hasta probar que no existe / está
fuera de la raíz (cursor intacto).

## Tasks
- [x] Vista: `Reveal(path) (bool, string)` — nodo en el aplanado → cursor sobre
  él; ancestro colapsado → `(false, dirPath)`; ancestro expandido sin el nodo,
  o fuera del árbol → `(true, "")` sin mover el cursor <!-- id: 0 -->
- [x] Vista: `ExpandDir(path, entries)` por ruta (no el cursor) + refactor de
  `SetChildren` a un `expandNode` común; no duplica hijos ya cargados;
  cursor/scroll estables en la expansión por ruta <!-- id: 1 -->
- [x] Controlador: `revealActiveInExplorer()` — bucle `Reveal` → `readEntries`
  → `ExpandDir` sobre `activeBuffer().Path()`; errores de lectura = no-op
  silencioso <!-- id: 2 -->
- [x] Disparadores de ENTRADA del foco: `Shift+Tab` y `Ctrl+B` (mostrar) llaman
  al reveal; el clic en el panel NO revela (el usuario ya elige la fila) <!-- id: 3 -->
- [x] Tests de la vista: reveal directo, paso a paso sobre ancestros colapsados,
  varios niveles, fuera del árbol (cursor intacto), re-expandir sin re-leer
  <!-- id: 4 -->
- [x] Tests del controlador: Shift+Tab revela el buffer activo con E/S real
  (dir colapsado sin hijos), Ctrl+B mostrar revela, el clic no interfiere
  <!-- id: 5 -->
- [x] Verificación y commit de unidad con Conventional Commit <!-- id: 6 -->

## Design decisions
- **La vista dice QUÉ expandir; el controlador hace la E/S.** `Reveal` nunca
  toca disco: devuelve el siguiente dir colapsado del camino. Cada iteración
  del bucle expande un nivel; los dirs ya expandidos no se re-leen (mismo
  diseño de "re-expandir sin releer" del árbol).
- **`ExpandDir` por ruta:** `SetChildren` inyecta en el nodo del CURSOR (el
  contrato de la navegación), pero en el reveal el cursor puede estar en otro
  lado. La expansión por ruta mantiene cursor y scroll estables; el cursor se
  posiciona al final con el `setCursor` de `Reveal`.
- **Solo en la entrada del foco:** Shift+Tab y Ctrl+B mostrar revelan; un clic
  en el árbol ya expresa la intención del usuario y no debe ser pisado por el
  reveal. Navegar el árbol (up/down/expandir) nunca re-revela.
- **Fuera del árbol o inexistente → cursor intacto:** si el ancestro expandido
  no contiene el target (o el target escapa del root visible), el reveal
  termina sin tocar la selección.