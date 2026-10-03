# Feature: Pestañas y explorador — U3b: árbol expandible desde la raíz

## Description
El explorador de U3 es una lista plana del directorio actual con `Enter` para
descender y `..` para subir. Quien usa el editor pidió poder **manejar el
worktree desde la ruta base**: que el panel quede anclado al root de la sesión
(donde se abre el editor) y muestre la jerarquía completa del proyecto, con
directorios anidados e indentados que se **colapsan y expanden** —nunca se
"cambia de carpeta", la base está siempre a la vista.

El árbol se carga **por nivel, al expandir**: un directorio colapsado no lee
sus hijos del disco. Es el lazy loading que pide la constitución: leer el árbol
completo al arrancar cargaría proyectos enteros (`node_modules`, `.git`) sin que
estén en pantalla.

## Tasks
- [x] `treeNode` en la vista: nombre, ruta, es dir, profundidad, expandido, hijos;
  el `FileBrowser` aplanado = nodos VISIBLES en orden (dirs expandidos + sus
  descendientes), root = cima implícita (no una fila) <!-- id: 0 -->
- [x] `SetRoot` + `SetRootEntries`: anclan el árbol al root de la sesión y cargan
  su primer nivel; cursor/scroll reseteados <!-- id: 1 -->
- [x] `HandleEvent` devuelve `(Action, handled)`: `ActionMove` (flechas/páginas/
  home/end, click), `ActionActivate` (Enter sobre archivo), `ActionExpand`
  (Enter/→ sobre dir colapsado con ruta); ← colapsa el dir expandido del cursor
  (la vista sola, sin E/S); Enter/→ sobre dir ya expandido → `ActionNone` <!-- id: 2 -->
- [x] `SetChildren(entries)`: inyecta los hijos leídos por el controlador en el
  dir del cursor, lo marca expandido y re-aplana; defensivo (no-op si el cursor
  no es un dir cargado) <!-- id: 3 -->
- [x] Colapsar re-aplana y deja el cursor en el dir colapsado (que sigue visible);
  expandir deja el cursor en el dir, con sus hijos a continuación <!-- id: 4 -->
- [x] `Draw`: indentación de 2 celdas por nivel, prefijo `▸`/`▾` en los directorios
  (2 celdas, alineado con 2 espacios en los archivos), `"`/"` al final de los dirs,
  fila del cursor revertida a todo el ancho, recorte por `writeString` <!-- id: 5 -->
- [x] Carga perezosa en el controlador: `ActionExpand` → `os.ReadDir` del dir del
  cursor → `SetChildren`; error de lectura → no-op silencioso (dir borrado) <!-- id: 6 -->
- [x] El arranque reemplaza `relist(explorerDir)` por `SetRoot(root)` +
  `SetRootEntries(hijos del root)`; `explorerDir` y la entrada sintética `..`
  desaparecen <!-- id: 7 -->
- [x] `ActionActivate` abre el archivo sin re-`stat` (el nodo ya sabe que es un
  archivo): `ws.Open` + foco al editor <!-- id: 8 -->
- [x] Tests de la vista reescritos para el árbol: dibujo con indentación y
  prefijos, expandir muestra los hijos, colapsar los oculta y re-ancla el cursor,
  clic en cualquier nivel, scroll, páginas, clamps, `Enter` sobre archivo →
  `ActionActivate`, `Enter`/`→` sobre dir colapsado → `ActionExpand`, `←`
  colapsa, Ctrl+Pg cae al controlador <!-- id: 9 -->
- [x] Tests del controlador: arranque con dir puebla el primer nivel, expandir un
  dir carga sus hijos (archivo en un subdir se abre desde el árbol), colapsar
  vuelve al root, apertura desde profundidad, y los tests de U3 que solo cambian
  de semántica se actualizan (descenso → expansión, `..` → colapso) <!-- id: 10 -->
- [x] Verificación: `go vet`, `gofmt -l` en las superficies, paquete `view` bajo
  `-race` con clang, tests de la unidad <!-- id: 11 -->
- [x] Commit de unidad de trabajo: `75a4098` <!-- id: 12 -->

## Design decisions

### El nodo es el dato; el aplanado es la lista visible
El `FileBrowser` guarda un árbol de `treeNode` (name, path, isDir, depth,
expanded, children). `nodes` es el aplanado de lo VISIBLE en orden: los hijos
del root más, por cada dir expandido, sus descendientes, respetando el orden
del listado (dirs primero, luego archivos, alfabético — lo garantiza el
controlador al leer). El cursor, el scroll y el hit-testing del mouse trabajan
sobre el aplanado; expandir/colapsar lo reconstruye.

### El root es la cima implícita
No hay fila para el root: el árbol muestra sus hijos en el nivel 0. El root es
la referencia de "dónde estoy" que pide quien usa el editor: anclado en
`SetRoot` y nunca cambiado por la navegación.

### Carga perezosa por directorio expandido
`SetChildren` es la única puerta de entrada de datos del disco a la vista
(además de `SetRootEntries`). El controlador responde a `ActionExpand`
leyendo el dir del cursor; colapsar no cuesta nada (la vista descarta los
hijos del aplanado, conservando los nodos para re-expandir sin re-leer).
Un dir borrado entre el expand y el enter cae en un no-op silencioso.

### `Action` en vez de bools acoplados
U3 acoplaba el controlador al detalle ("`(true,true)` = actívame"). Con tres
destinos distintos —redibujar, abrir un archivo, pedir datos para expandir—
un enum es más legible y deja el controlador genérico: `ActionExpand` es
"tengo el cursor sobre un dir colapsado, dame sus hijos", sin que el
controlador sepa qué tecla lo disparó.

### Enter/→ expanden, ← colapsa en el lugar (y sube al padre)
Enter sobre un archivo lo abre (viene de U3); sobre un dir colapsado expande;
sobre un dir ya expandido no hace nada. `←` cierra la carpeta con la semántica
estándar de árbol: colapsa el dir expandido del cursor y la selección queda EN
ÉL —colapsar una subcarpeta anidada nunca se lleva el cursor a otro nivel—; si
el cursor está en un hijo, PRIMERO sube la selección al padre (el segundo `←`
lo colapsa). Con el foco en el panel, `←` es SIEMPRE del árbol —también en el
nivel raíz sin nada que colapsar ni subir—: la flecha nunca se escapa al editor
moviendo el cursor por sorpresa.

Con el mouse, el clic sobre la flecha de expansión (`▸`/`▾`, las celdas
`[depth*2, depth*2+2)` de la fila) alterna colapsado ↔ expandido; el clic en el
resto de la fila solo selecciona.

## Falsificación (tests que escriben primero contra el código roto)
1. Sin árbol (lista plana vieja) → los tests de la vista del árbol no compilan/
   fallan (no hay `treeNode` ni `Action`).
2. Expandir sin cargar hijos → `TestEnterExpandsADirectoryAndItsChildrenAppear`.
3. Colapsar sin re-aplanar → `TestLeftCollapsesAndTheChildrenDisappear`.
4. Colapsar sin re-anclar el cursor → `TestCollapseReturnsTheCursorToTheDirectory`.
5. Archivo en un subdir no abrible desde el árbol → `TestEnterOpensAFileFromDepth`.
6. Bloqueo de la carga total: `SetRootEntries` con un dir y el controlador
   NUNCA lee los subdirs hasta `ActionExpand` → `TestSubdirsAreNotReadUntilExpanded`.

## Evidence
- Rama: `feat/multi-buffer-workspace`.

### U3b — árbol, tests y verificación
- **Archivos:** `internal/view/file_browser.go` (reescrito: `treeNode`, `Action`,
  `SetRoot`/`SetRootEntries`/`SetChildren`, aplanado visible, colapso interno,
  `Draw` con indentación y prefijos `▸`/`▾`), `internal/controller/app.go` (sin
  `explorerDir`/`relist`/`..`; `readEntries` compartido, `explorerExpand`,
  ruteo por `Action`), más los tests: `file_browser_test.go` (17) y
  `app_test.go` (tests del explorador actualizados + 5 nuevos).
- **Falsificaciones observadas (RED):** contra la lista plana los tests del árbol
  no compilaban (`undefined: SetRoot/SetRootEntries/SetChildren/ActionMove/...`)
  y 5 tests del controlador fallaban contra el código viejo
  (`TestStartupWithDirectoryArgument`, `TestEnterExpandsADirectoryAndItsChildrenAppear`,
  `TestLeftCollapsesADirectory`, `TestSubdirsAreNotReadUntilExpanded`,
  `TestEnterOnAnExpandedDirectoryDoesNothing`). Verdes tras la implementación.
- **Tests reescritos (cambio de diseño autorizado, sin debilitar aserciones):**
  `TestEnterDescendsIntoADirectory` → `TestEnterExpandsADirectoryAndItsChildrenAppear`
  y `TestUpEntryGoesBackToTheRoot` → `TestLeftCollapsesADirectory` (el descenso y
  el `..` desaparecen con la lista plana).
- **Verificación observada (host Windows):** `go vet ./...` limpio, `gofmt -l` sin
  salida en las 4 superficies, paquete `view` completo bajo
  `CGO_ENABLED=1 CC=clang go test -race` en verde (2.12s), 20 tests del
  controlador del explorador en verde; el controller completo solo falla los
  ambientales preexistentes (guardado/renombres sobre mmap en Windows).
- **Tradeoff documentado:** un dir colapsado conserva sus hijos, así que
  re-expandir no relee cambios en disco; es el costo del "re-expandir sin
  re-leer" y la carga perezosa (el toggle `Ctrl+B` tampoco relee: el árbol es
  estado en vivo).
- **Commit de unidad de trabajo:** `75a4098`.

### U3b-fix — cerrar carpetas con `←` (reportado por quien usa el editor)
- **Bug:** `←` solo colapsaba con el cursor sobre el dir expandido; sobre un hijo
  devolvía `(ActionNone, false)` y el evento caía al editor, que movía el cursor
  del documento —"no me permite cerrar una carpeta"—.
- **Fix:** `selectParentAtCursor` sube la selección al ancestro visible (el último
  nodo anterior del aplanado con menor profundidad); con el foco en el panel, `←`
  es siempre del árbol (`ActionMove`), también en el nivel raíz.
- **Tests:** `TestFileBrowserLeftClosesDirectories` (colapsa sobre el dir, sube al
  padre, segundo `←` colapsa, nivel raíz comido, re-expandir sin releer); también
  corrige la aserción vieja que fijaba la fuga al editor.
- **Falsificación RED:** la versión nueva del test fallaba contra el código viejo
  (`(ActionNone, false)` sobre un hijo) y pasó tras el fix.
- **Verificación:** `go test ./internal/view/` verde completo,
  `go test -race ./internal/view/` con clang verde, `go vet` y `gofmt` limpios,
  controller del explorador en verde.

### U3b-fix 2 — colapsar una SUBCARPETA (reportado de nuevo por quien usa el editor)
- **Bug (de mi propia "simplificación" del fix 1):** el `←` corría
  `selectParentAtCursor` SIEMPRE, también después de colapsar. Al colapsar una
  subcarpeta de nivel ≥1 la selección saltaba al directorio padre —"colapsás
  pero terminás en otro lado"—; con anidamiento profundo el usuario no podía
  cerrar la subcarpeta.
- **Fix:** cortocircuito en `←` (`if !fb.collapseAtCursor() { fb.selectParentAtCursor() }`):
  si colapsó, la selección queda EN el dir colapsado; solo sube al padre cuando
  no había nada que colapsar.
- **Mouse:** clic sobre la flecha `▸`/`▾` de un directorio alterna
  colapsado ↔ expandido; el clic en el resto de la fila solo selecciona.
- **Tests:** `TestFileBrowserLeftCollapsesASubdirectoryInPlace` (RED contra el
  código viejo: el cursor caía en el abuelo; GREEN tras el fix) y
  `TestFileBrowserClickOnTheExpansionArrowToggles`.
- **Verificación:** paquete `view` completo en verde, `-race` con clang en
  verde (2.40s), `go vet`/`gofmt` limpios, controller del explorador en verde.