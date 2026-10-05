package view

import (
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// isDescendantOf dice si node es descendiente estricto del directorio dir: su
// ruta empieza con dir.path y el primer carácter tras el prefijo es un
// separador de ruta. Acepta "/" y "\\" porque los nodos guardan rutas del SO
// (filepath.Join en Windows) y los tests usan rutas de juguete con "/": un
// prefijo a mitad de nombre —dir "/a", node "/ab"— no cuenta.
func isDescendantOf(dir, node string) bool {
	if !strings.HasPrefix(node, dir) || len(node) <= len(dir) {
		return false
	}
	switch node[len(dir)] {
	case '/', '\\':
		return true
	}
	return false
}

// wheelScroll es el desplazamiento de la rueda del mouse en filas, el mismo
// paso de 3 que usa el editor.
const wheelScroll = 3

// Entry describe lo que el controlador leyó del disco para depositar en el
// árbol: el nombre visible, la ruta completa y si es un directorio (los
// directorios se dibujan con el sufijo "/"). La vista solo dibuja lo que
// recibe; el controlador es quien lee con os.ReadDir y rellena Path.
type Entry struct {
	Name  string
	Path  string
	IsDir bool
}

// Action es lo que el explorador le pide al controlador tras manejar una
// tecla. U3 acoplaba al controlador con bools ("(true,true) = actívame"); con
// tres destinos distintos —redibujar, abrir un archivo, pedir datos para
// expandir— un enum es más legible y deja al controlador genérico: ActionExpand
// es "tengo el cursor sobre un dir colapsado, dame sus hijos", sin que el
// controlador sepa qué tecla lo disparó.
type Action int

const (
	ActionNone     Action = iota // el evento no cayó en el panel, o no pidió nada
	ActionMove                   // solo redibujar (movimiento, colapso interno, click)
	ActionActivate               // abrir el archivo del nodo del cursor
	ActionExpand                 // leer los hijos del dir del cursor y depositarlos
)

// treeNode es el nodo del árbol: el dato mutable que conserva la jerarquía
// (nombre, ruta, es dir, profundidad, expandido, hijos), colapsado o no. El
// aplanado visible —fb.nodes— es otra cosa: la PROYECCIÓN de los nodos que se
// ven en pantalla, en orden, con los dirs expandidos seguidos de sus
// descendientes. Los dirs colapsados conservan sus hijos en el nodo —fuera del
// aplanado— y re-expandir no los relee. El cursor, el scroll y el hit-testing
// del mouse trabajan sobre el aplanado; expandir/colapsar lo reconstruye.
type treeNode struct {
	name     string
	path     string
	isDir    bool
	depth    int
	expanded bool
	children []*treeNode
}

// FileBrowser es el árbol jerárquico del panel lateral, anclado al root de la
// sesión (la cima implícita: no hay fila para él, el árbol muestra sus hijos
// en el nivel 0).
//
// La vista NO abre archivos (eso es decisión de ws.Open en el controlador) ni
// lee el filesystem: el controlador lee con os.ReadDir y deposita los
// listados con SetRootEntries (primer nivel) y SetChildren (hijos de un dir al
// expandir). Estas dos son las ÚNICAS puertas de entrada de datos del disco a
// la vista: un dir colapsado no se lee, que es el lazy loading que pide la
// constitución. Colapsar es gratuito: la vista lo hace sola, sin E/S.
//
// Quien compone —el controlador— elige dónde cae el panel con la Surface; la
// vista dibuja desde (0,0) en coordenadas propias, como el editor.
type FileBrowser struct {
	root   string      // el root de la sesión: la cima implícita del árbol
	nodes  []*treeNode // el aplanado de lo VISIBLE, en orden
	cursor int         // índice del nodo activo en el aplanado
	top    int         // primer nodo visible
	width  int
	height int
	theme  Theme
}

// SetTheme reemplaza la paleta del componente.
func (fb *FileBrowser) SetTheme(th Theme) { fb.theme = th }

func NewFileBrowser() *FileBrowser {
	return &FileBrowser{}
}

// SetRoot ancla el árbol al root de la sesión y descarta el estado anterior
// (nodes, cursor y scroll vuelven a cero): el root es la base fija sobre la
// que SetRootEntries carga el primer nivel. El root nunca cambia por la
// navegación —el panel no tiene subida ni "cambiar de carpeta"—.
func (fb *FileBrowser) SetRoot(root string) {
	fb.root = root
	fb.nodes = nil
	fb.cursor, fb.top = 0, 0
}

// SetRootEntries reemplaza los hijos del primer nivel con entries (convertidas
// a *treeNode con depth 0), reconstruye el aplanado y resetea cursor y scroll:
// es el reemplazo del relist de U3. Un nivel vacío dibuja nada, como una lista
// vacía.
func (fb *FileBrowser) SetRootEntries(entries []Entry) {
	roots := make([]*treeNode, 0, len(entries))
	for _, e := range entries {
		roots = append(roots, &treeNode{
			name:  e.Name,
			path:  e.Path,
			isDir: e.IsDir,
			depth: 0,
		})
	}
	fb.nodes = roots
	fb.flatten()
	fb.cursor, fb.top = 0, 0
}

// SetChildren inyecta los hijos leídos por el controlador en el nodo del
// cursor (convertidas con depth+1), lo marca expandido y reconstruye el
// aplanado manteniendo el cursor visible: es la respuesta a ActionExpand.
//
// Defensiva: es no-op si el cursor está fuera de rango o el nodo del cursor no
// es un dir. Si el dir ya tiene hijos cargados —expandido, o colapsado
// conservándolos tras un expand previo— no los duplica: solo re-marca
// expandido y re-aplana, que es el "re-expandir sin re-leer" del diseño.
func (fb *FileBrowser) SetChildren(entries []Entry) {
	if fb.cursor < 0 || fb.cursor >= len(fb.nodes) {
		return
	}
	node := fb.nodes[fb.cursor]
	if !node.isDir {
		return
	}
	fb.expandNode(node, entries)
	fb.ensureCursorVisible()
}

// expandNode marca expandido el nodo dir e inyecta sus entries como hijos si
// no los tenía cargados, re-aplana y clampa: el núcleo común de SetChildren
// (expansión del CURSOR) y ExpandDir (expansión por RUTA). Los hijos ya
// cargados no se duplican: re-expandir un dir guardado es gratis, sin E/S.
func (fb *FileBrowser) expandNode(node *treeNode, entries []Entry) {
	if len(node.children) == 0 {
		for _, e := range entries {
			node.children = append(node.children, &treeNode{
				name:  e.Name,
				path:  e.Path,
				isDir: e.IsDir,
				depth: node.depth + 1,
			})
		}
	}
	node.expanded = true
	fb.flatten()
	fb.clamp()
}

// ExpandDir marca expandido el dir con path dado —no hace falta que sea el del
// cursor— e inyecta sus hijos si no los tenía: la expansión por RUTA que usa
// el reveal del controlador (Reveal + ExpandDir alternan), donde la selección
// puede estar en cualquier otro nodo. Cursor y scroll quedan estables: el
// reveal posiciona el cursor recién al terminar, en Reveal. Devuelve false si
// el dir no está en el árbol.
func (fb *FileBrowser) ExpandDir(path string, entries []Entry) bool {
	for _, n := range fb.nodes {
		if n.path == path && n.isDir {
			fb.expandNode(n, entries)
			return true
		}
	}
	return false
}

// Reveal acerca el cursor al nodo de path —absoluto, como los que guardan los
// nodos— si está en el árbol, y devuelve:
//   - (true, ""): el nodo quedó visible y seleccionado; o el nodo no existe
//     o no está bajo el árbol actual, y el cursor NO se movió (el controlador
//     termina igual en ambos casos).
//   - (false, dir): falta expandir el ancestro visible más profundo del path
//     (un dir colapsado); el controlador lee sus hijos con readEntries, llama
//     ExpandDir y vuelve a llamar a Reveal.
//
// El controlador itera: cada pasada expande un nivel más, perezosamente —un
// dir ya expandido no se relee—, hasta que el nodo queda visible o se prueba
// que no está. La vista nunca toca el filesystem.
func (fb *FileBrowser) Reveal(path string) (bool, string) {
	// El nodo objetivo ya está en el aplanado: solo posicionar el cursor.
	if idx, ok := fb.findNodeIndex(path); ok {
		fb.setCursor(idx)
		return true, ""
	}
	// Ancestro visible más profundo del path: el que hay que seguir abriendo.
	var dir *treeNode
	for _, n := range fb.nodes {
		if n.isDir && isDescendantOf(n.path, path) && (dir == nil || n.depth > dir.depth) {
			dir = n
		}
	}
	if dir != nil && !dir.expanded {
		return false, dir.path
	}
	// Sin ancestro visible (fuera del árbol) o ancestro expandido sin el nodo
	// (el dir ya se leyó y no está ahí): no hay nada que revelar.
	return true, ""
}

// findNodeIndex devuelve el índice del nodo con la ruta dada en el aplanado.
func (fb *FileBrowser) findNodeIndex(path string) (int, bool) {
	for i, n := range fb.nodes {
		if n.path == path {
			return i, true
		}
	}
	return 0, false
}

// flatten reconstruye el aplanado de lo VISIBLE desde el árbol que ya vive en
// los nodos: los hijos del root son los nodos de profundidad 0 en orden, y
// cada dir expandido va seguido de sus descendientes. Los dirs colapsados
// quedan en el aplanado pero sin sus hijos, que se conservan en el nodo.
// expandir/colapsar llama a flatten; el resultado es lo que dibuja Draw.
func (fb *FileBrowser) flatten() {
	roots := make([]*treeNode, 0, 8)
	for _, n := range fb.nodes {
		if n.depth == 0 {
			roots = append(roots, n)
		}
	}
	flat := make([]*treeNode, 0, len(fb.nodes))
	var appendVisible func(list []*treeNode)
	appendVisible = func(list []*treeNode) {
		for _, n := range list {
			flat = append(flat, n)
			if n.isDir && n.expanded {
				appendVisible(n.children)
			}
		}
	}
	appendVisible(roots)
	fb.nodes = flat
}

// clamp mantiene cursor y top dentro del aplanado actual. Sobre un aplanado
// vacío ambos vuelven a 0.
func (fb *FileBrowser) clamp() {
	if len(fb.nodes) == 0 {
		fb.cursor, fb.top = 0, 0
		return
	}
	n := len(fb.nodes)
	fb.cursor = min(max(fb.cursor, 0), n-1)
	maxTop := n - fb.height
	if maxTop < 0 {
		maxTop = 0
	}
	fb.top = min(max(fb.top, 0), maxTop)
}

// ensureCursorVisible corre top lo mínimo para que el nodo activo quede
// dentro del alto del panel, como hace el editor con su viewport.
func (fb *FileBrowser) ensureCursorVisible() {
	if len(fb.nodes) == 0 || fb.height <= 0 {
		return
	}
	if fb.cursor < fb.top {
		fb.top = fb.cursor
	}
	if fb.cursor >= fb.top+fb.height {
		fb.top = fb.cursor - fb.height + 1
	}
	fb.clamp()
}

// setCursor coloca el cursor en el índice i (clampeado) y mantiene el nodo
// activo visible.
func (fb *FileBrowser) setCursor(i int) {
	fb.cursor = i
	fb.clamp()
	fb.ensureCursorVisible()
}

// moveCursor desplaza el cursor en delta y devuelve si cambió de posición.
func (fb *FileBrowser) moveCursor(delta int) bool {
	n := len(fb.nodes)
	if n == 0 {
		return false
	}
	target := fb.cursor + delta
	if target < 0 {
		target = 0
	}
	if target >= n {
		target = n - 1
	}
	if target == fb.cursor {
		return false
	}
	fb.cursor = target
	fb.ensureCursorVisible()
	return true
}

// page es el salto de página: lo que cabe en el alto del panel, mínimo 1.
func (fb *FileBrowser) page() int {
	if fb.height > 1 {
		return fb.height
	}
	return 1
}

// Resize actualiza las dimensiones del panel y reencuadra el scroll, como el
// resize del editor: el nodo activo queda visible y el top dentro del rango.
func (fb *FileBrowser) Resize(width, height int) {
	if width > 0 {
		fb.width = width
	}
	if height > 0 {
		fb.height = height
	}
	fb.clamp()
	fb.ensureCursorVisible()
}

// CursorPath devuelve la ruta del nodo activo, o "" si no hay nodos.
func (fb *FileBrowser) CursorPath() string {
	if len(fb.nodes) == 0 {
		return ""
	}
	return fb.nodes[fb.cursor].path
}

// CursorDir devuelve el DIRECTORIO destino de una creación contextual: la ruta
// del nodo activo si es un directorio, el directorio que lo contiene si es un
// archivo, o "" si no hay nodos (el controlador cae al root de la sesión). Es la
// contraparte de CursorPath para el controlador, que decide dónde crear el
// archivo o la carpeta nueva.
//
// La vista solo devuelve la ruta calculada: no toca el filesystem ni el estado
// del workspace, como el resto del panel.
func (fb *FileBrowser) CursorDir() string {
	if len(fb.nodes) == 0 {
		return ""
	}
	n := fb.nodes[fb.cursor]
	if n.isDir {
		return n.path
	}
	return filepath.Dir(n.path)
}

// AddChild inserta e como hijo del directorio con ruta parentPath y devuelve si
// la inserción ocurrió: es la tercera puerta de entrada de datos a la vista (con
// SetRootEntries y SetChildren), la que usa el controlador después de crear un
// archivo o una carpeta en disco para que el nodo nuevo aparezca sin re-leer el
// directorio.
//
// Es defensiva: si el padre no está VISIBLE en el árbol (dentro de un nivel que
// nunca se expandió, o en otra rama) devuelve false y no toca nada —el
// controlador no expande por la espalda; el nodo aparecerá al expandir el
// padre—. Un padre que no es un directorio, o una ruta vacía, también la
// rechazan. parentPath igual al root inserta en el primer nivel, que es donde
// vive el root como cima implícita. La inserción respeta el orden de
// readEntries (directorios primero, luego archivos, ambos alfabéticos) y
// reconstruye el aplanado manteniendo el cursor visible.
func (fb *FileBrowser) AddChild(parentPath string, e Entry) bool {
	if parentPath == "" {
		return false
	}
	child := &treeNode{name: e.Name, path: e.Path, isDir: e.IsDir}

	if parentPath == fb.root {
		// El root es la cima implícita: no hay un nodo que lo represente, así que
		// el hijo nuevo entra en el primer nivel, entre los nodos de profundidad 0
		// del aplanado.
		child.depth = 0
		var roots, rest []*treeNode
		for _, n := range fb.nodes {
			if n.depth == 0 {
				roots = append(roots, n)
			} else {
				rest = append(rest, n)
			}
		}
		fb.nodes = append(insertSorted(roots, child), rest...)
	} else {
		var parent *treeNode
		for _, n := range fb.nodes {
			if n.path == parentPath {
				parent = n
				break
			}
		}
		if parent == nil || !parent.isDir {
			return false
		}
		child.depth = parent.depth + 1
		parent.children = insertSorted(parent.children, child)
	}

	fb.flatten()
	fb.ensureCursorVisible()
	return true
}

// insertSorted devuelve list con child insertado en la posición que le
// corresponde en un listado de directorio: los directorios antes que los
// archivos, y dentro de cada grupo por orden alfabético —el mismo criterio de
// readEntries, para que un nodo creado no quede al final de su nivel hasta el
// próximo relist—.
func insertSorted(list []*treeNode, child *treeNode) []*treeNode {
	pos := len(list)
	for i, c := range list {
		if child.isDir && !c.isDir {
			pos = i
			break
		}
		if child.isDir == c.isDir && child.name < c.name {
			pos = i
			break
		}
	}
	list = append(list, nil)
	copy(list[pos+1:], list[pos:])
	list[pos] = child
	return list
}

// activateOrExpand decide la acción de Enter/→ sobre el nodo del cursor: un
// archivo se activa (ActionActivate), un dir colapsado pide sus hijos
// (ActionExpand) y un dir ya expandido se COLAPSA (toggle con la misma tecla
// con la que se abrió: Enter/→ alternan expandido ↔ colapsado, como en
// cualquier árbol; la acción es interna, sin E/S, y la selección queda en el
// dir). Sin nodos, Enter cae al flujo normal del controlador, como en U3.
func (fb *FileBrowser) activateOrExpand() (Action, bool) {
	if len(fb.nodes) == 0 {
		return ActionNone, false
	}
	n := fb.nodes[fb.cursor]
	if !n.isDir {
		return ActionActivate, true
	}
	if n.expanded {
		fb.collapseAtCursor()
		return ActionMove, true
	}
	return ActionExpand, true
}

// collapseAtCursor colapsa el dir expandido del cursor SIN E/S: el nodo
// conserva sus hijos (para re-expandir sin re-leer) y el aplanado se
// reconstruye sin ellos. Devuelve si colapsó algo.
func (fb *FileBrowser) collapseAtCursor() bool {
	if len(fb.nodes) == 0 {
		return false
	}
	n := fb.nodes[fb.cursor]
	if !n.isDir || !n.expanded {
		return false
	}
	n.expanded = false
	fb.flatten()
	fb.clamp()
	fb.ensureCursorVisible()
	return true
}

// selectParentAtCursor sube la selección al ancestro visible del nodo activo:
// en el aplanado (DFS), el dir expandido que contiene al nodo es el último
// nodo anterior con profundidad menor. Devuelve false si el nodo no tiene
// ancestro (nivel 0).
func (fb *FileBrowser) selectParentAtCursor() bool {
	if len(fb.nodes) == 0 || fb.cursor == 0 {
		return false
	}
	d := fb.nodes[fb.cursor].depth
	for j := fb.cursor - 1; j >= 0; j-- {
		if fb.nodes[j].depth < d {
			fb.cursor = j
			fb.ensureCursorVisible()
			return true
		}
	}
	return false
}

// HandleEvent procesa el teclado y el mouse del panel y devuelve (Action,
// handled): handled dice si el evento era del panel y Action qué quiere el
// panel del controlador (o nada).
//
// Up/Down/PageUp/PageDown/Home/End mueven el cursor y devuelven (ActionMove,
// true) AUNQUE el cursor no se mueva: con el foco en el explorador, esas
// teclas son del panel y no del documento. Enter/KeyLF y → sobre un archivo
// devuelven (ActionActivate, true) y sobre un dir colapsado (ActionExpand,
// true); ← colapsa el dir expandido del cursor internamente y devuelve
// (ActionMove, true). El mouse selecciona con Button1 (ActionMove) y scrollea
// con la rueda. Toda otra tecla o evento devuelve (ActionNone, false) y cae al
// flujo normal del controlador (atajos, documento, salida).
func (fb *FileBrowser) HandleEvent(ev tcell.Event) (Action, bool) {
	switch ev := ev.(type) {
	case *tcell.EventKey:
		switch ev.Key() {
		case tcell.KeyUp:
			fb.moveCursor(-1)
			return ActionMove, true
		case tcell.KeyDown:
			fb.moveCursor(1)
			return ActionMove, true
		case tcell.KeyPgUp:
			// Ctrl+PageUp/PageDown cambian de pestaña y son del controlador
			// (U2b), no scroll de página: el panel los deja caer, igual que el
			// editor los descarta con ModCtrl para que ninguna variante de
			// terminal escrolle por accidente.
			if ev.Modifiers()&tcell.ModCtrl != 0 {
				return ActionNone, false
			}
			fb.moveCursor(-fb.page())
			return ActionMove, true
		case tcell.KeyPgDn:
			if ev.Modifiers()&tcell.ModCtrl != 0 {
				return ActionNone, false
			}
			fb.moveCursor(fb.page())
			return ActionMove, true
		case tcell.KeyHome:
			fb.setCursor(0)
			return ActionMove, true
		case tcell.KeyEnd:
			fb.setCursor(len(fb.nodes) - 1)
			return ActionMove, true
		case tcell.KeyEnter, tcell.KeyLF:
			return fb.activateOrExpand()
		case tcell.KeyRight:
			return fb.activateOrExpand()
		case tcell.KeyLeft:
			// ← cierra la carpeta. Si el cursor está SOBRE el dir expandido, lo
			// colapsa y la selección queda EN ÉL —nunca salta al padre: al
			// colapsar una subcarpeta anidada, el colapso no se lleva el cursor
			// a otro nivel—. Si el cursor está en un hijo (o en un dir ya
			// colapsado), sube la selección al ancestro visible (el segundo ←
			// lo colapsa). Con el foco en el panel, ← es SIEMPRE del árbol
			// —también en el nivel raíz sin nada que colapsar ni subir—: la
			// flecha nunca se escapa al editor moviendo el cursor del documento
			// por sorpresa.
			if !fb.collapseAtCursor() {
				fb.selectParentAtCursor()
			}
			return ActionMove, true
		}

	case *tcell.EventMouse:
		x, y := ev.Position()
		switch {
		case ev.Buttons()&tcell.Button1 != 0:
			// El clic selecciona la fila del aplanado —cualquier nivel—; el
			// controlador enfoca el panel al enterarse de que el panel lo
			// manejó. Un clic fuera de las filas del panel no selecciona nada.
			if len(fb.nodes) == 0 || y < 0 || y >= fb.height {
				return ActionNone, false
			}
			idx := fb.top + y
			n := fb.nodes[idx]
			// Clic sobre la flecha de expansión (▸/▾) de un directorio:
			// alterna colapsado ↔ expandido, como en cualquier árbol GUI. La
			// flecha ocupa las celdas [depth*2, depth*2+2) de la fila; el clic
			// en el resto de la fila solo selecciona.
			if n.isDir && x >= n.depth*2 && x < n.depth*2+2 {
				fb.setCursor(idx)
				if n.expanded {
					fb.collapseAtCursor()
					return ActionMove, true
				}
				return ActionExpand, true
			}
			fb.setCursor(idx)
			return ActionMove, true
		case ev.Buttons()&tcell.WheelUp != 0:
			fb.moveCursor(-wheelScroll)
			return ActionMove, true
		case ev.Buttons()&tcell.WheelDown != 0:
			fb.moveCursor(wheelScroll)
			return ActionMove, true
		}
	}
	return ActionNone, false
}

// Draw pinta el árbol en coordenadas propias desde (0,0): el nodo activo va
// con la barra de selección (TreeCursor, fondo de acento) a todo el ancho del
// panel. Cada fila es indent + prefijo + nombre (+ "/" en los directorios),
// con la indentación de dos celdas por nivel y el prefijo como señal del tipo:
// "▸ " dir colapsado, "▾ " dir expandido, "  " archivo inactivo y "> "
// archivo ACTIVO —el marcador viaja con la selección y refuerza la barra sin
// salir de las dos celdas, así los nombres quedan alineados con los carets de
// los directorios—. Los nombres que no entran se recortan contra el ancho del
// panel (writeString avanza por grapheme cluster). Sin nodos o sin alto no hay
// nada que dibujar.
func (fb *FileBrowser) Draw(s Surface) {
	if len(fb.nodes) == 0 || fb.height <= 0 {
		return
	}
	for row := 0; row < fb.height; row++ {
		idx := fb.top + row
		if idx >= len(fb.nodes) {
			break
		}
		n := fb.nodes[idx]

		// La fila se pinta entera con el estilo del nodo: la barra de
		// selección de ancho completo hace legible la activa aunque el nombre
		// sea corto, y el repintado por fila limpia el resaltado viejo del
		// redibujo anterior.
		th := themeOr(fb.theme)
		style := th.Text
		if idx == fb.cursor {
			style = th.TreeCursor
		}
		for x := 0; x < fb.width; x++ {
			s.SetContent(x, row, ' ', nil, style)
		}

		prefix := "  "
		if n.isDir {
			// Los directorios conservan su caret también en la fila activa: el
			// caret es el indicador de expansión, reemplazarlo en la selección
			// perdería el estado expandido/colapsado.
			if n.expanded {
				prefix = "▾ "
			} else {
				prefix = "▸ "
			}
		} else if idx == fb.cursor {
			// El marcador de la fila activa en los archivos: además de la barra
			// de selección, "> " señala dónde está el cursor en el árbol.
			prefix = "> "
		}
		line := strings.Repeat(" ", n.depth*2) + prefix + n.name
		if n.isDir {
			line += "/"
		}
		writeString(s, 0, row, line, style, fb.width)
	}
}
