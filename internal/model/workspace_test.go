package model

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// wFile crea un archivo con contenido dentro de un directorio fijo y devuelve
// la ruta absoluta limpia. El callers usan el mismo dir para poder armar
// rutas no normalizadas.
func wFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}
	return path
}

func TestWorkspaceOpensTwoFilesInOrder(t *testing.T) {
	dir := t.TempDir()
	a := wFile(t, dir, "a.txt", "a")
	b := wFile(t, dir, "b.txt", "b")
	w := NewWorkspace()

	if err := w.SetRoot(dir); err != nil {
		t.Fatalf("SetRoot falló: %v", err)
	}
	if _, err := w.Open(a); err != nil {
		t.Fatalf("Open(a) falló: %v", err)
	}
	if _, err := w.Open(b); err != nil {
		t.Fatalf("Open(b) falló: %v", err)
	}

	if got, want := w.Len(), 2; got != want {
		t.Fatalf("Len() = %d, se esperaba %d", got, want)
	}
	// El orden es el de apertura: a primero, b segundo y activo.
	if got := w.Buffers()[0].Path(); got != a {
		t.Errorf("Buffers()[0].Path() = %q, se esperaba %q", got, a)
	}
	if got := w.Buffers()[1].Path(); got != b {
		t.Errorf("Buffers()[1].Path() = %q, se esperaba %q", got, b)
	}
	if w.Active() == nil || w.Active().Path() != b {
		t.Errorf("Active() = %v, se esperaba el buffer de %q", w.Active(), b)
	}
	if got, want := w.ActiveIndex(), 1; got != want {
		t.Errorf("ActiveIndex() = %d, se esperaba %d", got, want)
	}
	// La raíz quedó registrada para el explorador de archivos.
	if got := w.Root(); got != dir {
		t.Errorf("Root() = %q, se esperaba %q", got, dir)
	}
}

// openThree carga tres archivos en un workspace y devuelve la raíz y las rutas
// en orden de apertura: a[0] < a[1] < a[2].
func openThree(t *testing.T) (*Workspace, [3]string) {
	t.Helper()
	dir := t.TempDir()
	paths := [3]string{
		wFile(t, dir, "a.txt", "a"),
		wFile(t, dir, "b.txt", "b"),
		wFile(t, dir, "c.txt", "c"),
	}
	w := NewWorkspace()
	if err := w.SetRoot(dir); err != nil {
		t.Fatalf("SetRoot falló: %v", err)
	}
	for i, p := range paths {
		if _, err := w.Open(p); err != nil {
			t.Fatalf("Open(%q) falló: %v", p, err)
		}
		if got, want := w.ActiveIndex(), i; got != want {
			t.Fatalf("ActiveIndex() tras Open #%d = %d, se esperaba %d", i, got, want)
		}
	}
	return w, paths
}

// openFour abre cuatro archivos y devuelve el workspace con el último activo.
func openFour(t *testing.T) (*Workspace, []string) {
	t.Helper()
	dir := t.TempDir()
	paths := []string{
		wFile(t, dir, "a.txt", "a"),
		wFile(t, dir, "b.txt", "b"),
		wFile(t, dir, "c.txt", "c"),
		wFile(t, dir, "d.txt", "d"),
	}
	w := NewWorkspace()
	if err := w.SetRoot(dir); err != nil {
		t.Fatalf("SetRoot falló: %v", err)
	}
	for i, p := range paths {
		if _, err := w.Open(p); err != nil {
			t.Fatalf("Open(%q) falló: %v", p, err)
		}
		if got, want := w.ActiveIndex(), i; got != want {
			t.Fatalf("ActiveIndex() tras Open #%d = %d, se esperaba %d", i, got, want)
		}
	}
	return w, paths
}

func TestWorkspaceOpenIsIdempotent(t *testing.T) {
	w, paths := openThree(t)

	// La ruta la da el filesystem: la forma de escribir la ruta da igual.
	messy := filepath.Join(filepath.Dir(paths[1]), "sub", "..", filepath.Base(paths[1]))

	got, err := w.Open(messy)
	if err != nil {
		t.Fatalf("Open de la misma ruta falló: %v", err)
	}
	if want, _ := filepath.EvalSymlinks(paths[1]); got != w.Buffers()[1] || got.Path() != want {
		t.Fatalf("Open no devolvió el buffer existente; Path() = %q,ActiveIndex() = %d",
			got.Path(), w.ActiveIndex())
	}
	if got := w.Len(); got != 3 {
		t.Fatalf("Len() tras abrir dos veces = %d, se esperaba 3 (sin duplicados)", got)
	}
	// La dedupe activa el buffer existente: pasa a ser el activo (índice 1).
	if got, want := w.ActiveIndex(), 1; got != want {
		t.Fatalf("ActiveIndex() = %d, se esperaba %d", got, want)
	}
}

func TestWorkspaceOpenSymlinkResolvesToSameBuffer(t *testing.T) {
	dir := t.TempDir()
	a := wFile(t, dir, "origin.txt", "contenido original")
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(a, link); err != nil {
		t.Skip("no se pudo crear el symlink:", err)
	}
	w := NewWorkspace()
	if err := w.SetRoot(dir); err != nil {
		t.Fatalf("SetRoot falló: %v", err)
	}

	first, err := w.Open(a)
	if err != nil {
		t.Fatalf("Open(origen) falló: %v", err)
	}
	if got := w.Len(); got != 1 {
		t.Fatalf("Len() tras abrir el origen = %d, se esperaba 1", got)
	}

	second, err := w.Open(link)
	if err != nil {
		t.Fatalf("Open(symlink) falló: %v", err)
	}
	if second != first {
		t.Fatal("abrir el symlink devolvió un buffer nuevo en lugar del existente")
	}
	if got := w.Len(); got != 1 {
		t.Fatalf("Len() tras abrir el symlink = %d, se esperaba 1 (sin duplicados)", got)
	}
}

func TestWorkspaceOpenDirectoryFails(t *testing.T) {
	dir := t.TempDir() // El propio directorio temporal es un dir real en disco.
	w := NewWorkspace()

	if _, err := w.Open(dir); err != ErrIsDirectory {
		t.Fatalf("Open(directorio) = %v, se esperaba ErrIsDirectory", err)
	}
	if got := w.Len(); got != 0 {
		t.Fatalf("Len() = %d, se esperaba 0 tras el error", got)
	}
	if w.Active() != nil {
		t.Fatalf("Active() = %v, se esperaba nil", w.Active())
	}
}

func TestWorkspaceOpenEmptyPathFails(t *testing.T) {
	w := NewWorkspace()
	cases := []string{"", "   ", "\t"}
	for _, path := range cases {
		if _, err := w.Open(path); err == nil {
			t.Fatalf("Open(%q) debe devolver error", path)
		}
		if got := w.Len(); got != 0 {
			t.Fatalf("Len() tras Open(%q) = %d, se esperaba 0", path, got)
		}
	}
}

func TestWorkspaceOpenNonexistentDoesNotCorruptState(t *testing.T) {
	dir := t.TempDir()
	w := NewWorkspace()
	if err := w.SetRoot(dir); err != nil {
		t.Fatalf("SetRoot falló: %v", err)
	}

	if _, err := w.Open(filepath.Join(dir, "fantasma.txt")); err == nil {
		t.Fatal("Open de un archivo inexistente debe devolver error")
	}
	if got := w.Len(); got != 0 {
		t.Fatalf("Len() tras el error = %d, se esperaba 0", got)
	}
	if w.Active() != nil || w.ActiveIndex() != -1 {
		t.Fatalf("estado tras el error: Active()=%v ActiveIndex()=%d, se esperaba nil/-1",
			w.Active(), w.ActiveIndex())
	}

	// El workspace queda utilizable: abrir un archivo válido sigue funcionando.
	good := wFile(t, dir, "ok.txt", "ok")
	got, err := w.Open(good)
	if err != nil {
		t.Fatalf("Open tras el error falló: %v", err)
	}
	if got.Path() != good {
		t.Fatalf("Path() = %q, se esperaba %q", got.Path(), good)
	}
	if got, want := w.Len(), 1; got != want {
		t.Fatalf("Len() = %d, se esperaba %d", got, want)
	}
	if got, want := w.ActiveIndex(), 0; got != want {
		t.Fatalf("ActiveIndex() = %d, se esperaba %d", got, want)
	}
}

func TestWorkspaceCloseRefusesModifiedBuffer(t *testing.T) {
	w, paths := openThree(t)

	// El activo es paths[2]. Se modifica un buffer no activo para probar que
	// el chequeo no depende de cuál esté activo.
	b := w.Buffers()[1]
	if err := b.Insert(0, "edit"); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}

	err := w.Close(1)
	if !errors.Is(err, ErrBufferModified) {
		t.Fatalf("Close(modificado) = %v, se esperaba ErrBufferModified", err)
	}
	if got, want := w.Len(), 3; got != want {
		t.Fatalf("Len() tras el rechazo = %d, se esperaba %d", got, want)
	}
	// El buffer sigue utilizable: el contenido incluye la edición y la ruta.
	if got, want := b.GetContent(), "editb"; got != want {
		t.Fatalf("contenido del buffer = %q, se esperaba %q", got, want)
	}
	if got := b.Path(); got != paths[1] {
		t.Fatalf("Path() = %q, se esperaba %q", got, paths[1])
	}
}

func TestWorkspaceCloseForceRemovesModifiedBuffer(t *testing.T) {
	w, _ := openThree(t)

	b := w.Buffers()[1]
	if err := b.Insert(0, "edit"); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}

	if err := w.CloseForce(1); err != nil {
		t.Fatalf("CloseForce(modificado) falló: %v", err)
	}
	if got, want := w.Len(), 2; got != want {
		t.Fatalf("Len() = %d, se esperaba %d", got, want)
	}
	if w.AnyModified() {
		// Tras el cierre forzado solo quedan buffers limpios.
		t.Fatal("CloseForce(1) no debió dejar buffers modificados")
	}
}

func TestWorkspaceCloseBeforeActiveKeepsSameActiveBuffer(t *testing.T) {
	w, paths := openThree(t)

	// El activo es paths[2]. Cerrar una pestaña ANTERIOR no puede cambiar de
	// archivo: el mismo documento tiene que seguir adelante, corrido una
	// posición a la izquierda.
	if err := w.CloseForce(0); err != nil {
		t.Fatalf("CloseForce(0) falló: %v", err)
	}

	if got, want := w.Len(), 2; got != want {
		t.Fatalf("Len() = %d, se esperaba %d", got, want)
	}
	if got, want := w.ActiveIndex(), 1; got != want {
		t.Fatalf("ActiveIndex() = %d, se esperaba %d", got, want)
	}
	if got := w.Active().Path(); got != paths[2] {
		t.Fatalf("Active().Path() = %q, se esperaba %q (el mismo archivo de antes)", got, paths[2])
	}
}

func TestWorkspaceCloseAfterActiveKeepsActive(t *testing.T) {
	w, paths := openThree(t)

	if err := w.SetActive(0); err != nil {
		t.Fatalf("SetActive(0) falló: %v", err)
	}

	// Cerrar una pestaña POSTERIOR no toca a la activa.
	if err := w.CloseForce(2); err != nil {
		t.Fatalf("CloseForce(2) falló: %v", err)
	}

	if got, want := w.ActiveIndex(), 0; got != want {
		t.Fatalf("ActiveIndex() = %d, se esperaba %d", got, want)
	}
	if got := w.Active().Path(); got != paths[0] {
		t.Fatalf("Active().Path() = %q, se esperaba %q", got, paths[0])
	}
}

func TestWorkspaceCloseReleasesResources(t *testing.T) {
	dir := t.TempDir()
	a := wFile(t, dir, "a.txt", "contenido para un mmap no vacío")
	w := NewWorkspace()
	if err := w.SetRoot(dir); err != nil {
		t.Fatalf("SetRoot falló: %v", err)
	}
	pt, err := w.Open(a)
	if err != nil {
		t.Fatalf("Open falló: %v", err)
	}
	// Con el buffer abierto el proceso tiene exactamente un descriptor para el
	// archivo: es la línea de base que le da sentido al 0 del final.
	if got := openFDsFor(t, a); got != 1 {
		t.Fatalf("descriptores abiertos para a.txt con el buffer abierto = %d, se esperaba 1", got)
	}

	idx := w.ActiveIndex()
	if err := w.CloseForce(idx); err != nil {
		t.Fatalf("CloseForce falló: %v", err)
	}
	// El buffer removido debe quedar sin descriptor ni mmap. Se comprueba sobre
	// las estructuras, no sobre el behavior público.
	if pt.file != nil {
		t.Fatal("CloseForce dejó pt.file != nil en el buffer cerrado")
	}
	if pt.originalBuffer != nil {
		t.Fatal("CloseForce dejó pt.originalBuffer != nil en el buffer cerrado")
	}
	// Y el descriptor tiene que estar cerrado de verdad, no solo con el campo en
	// nil: eso se observa contando los descriptores reales del proceso.
	if got := openFDsFor(t, a); got != 0 {
		t.Fatalf("descriptores abiertos para a.txt tras CloseForce = %d, se esperaba 0", got)
	}
}

// openFDsFor cuenta los descriptores abiertos del proceso que apuntan al
// archivo path, leyendo /proc/self/fd.
//
// Es la diferencia entre comprobar que un campo quedó en nil y comprobar que el
// sistema operativo ya no tiene el archivo abierto: si release() dejara de
// cerrar el descriptor pero igual limpiara el campo, este contador lo delata.
// Se compara por identidad de archivo (os.SameFile) y no por texto de ruta, para
// no depender de cómo se escribe el camino.
func openFDsFor(t *testing.T, path string) int {
	t.Helper()
	if _, err := os.Stat("/proc/self/fd"); err != nil {
		t.Skip("/proc/self/fd no está disponible, no se puede observar el descriptor")
	}

	want, err := os.Stat(path)
	if err != nil {
		t.Fatalf("no se pudo hacer stat de %q: %v", path, err)
	}
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatalf("no se pudo leer /proc/self/fd: %v", err)
	}

	count := 0
	for _, e := range entries {
		target, err := os.Readlink(filepath.Join("/proc/self/fd", e.Name()))
		if err != nil {
			// El descriptor puede haberse cerrado mientras leíamos, entre ellos el
			// propio directorio que usó ReadDir.
			continue
		}
		info, err := os.Stat(target)
		if err != nil {
			continue
		}
		if os.SameFile(info, want) {
			count++
		}
	}
	return count
}

// TestWorkspaceCloseWithMiddleBufferActive cubre los tres cierres cuando la
// pestaña activa es la del medio. Es el caso que el invariante del índice tiene
// que resolver en las dos direcciones y sobre sí mismo.
func TestWorkspaceCloseWithMiddleBufferActive(t *testing.T) {
	t.Run("cerrar una anterior deja la misma activa", func(t *testing.T) {
		w, paths := openThree(t)
		if err := w.SetActive(1); err != nil {
			t.Fatalf("SetActive(1) falló: %v", err)
		}
		if err := w.CloseForce(0); err != nil {
			t.Fatalf("CloseForce(0) falló: %v", err)
		}
		if got, want := w.ActiveIndex(), 0; got != want {
			t.Fatalf("ActiveIndex() = %d, se esperaba %d", got, want)
		}
		if got := w.Active().Path(); got != paths[1] {
			t.Fatalf("Active().Path() = %q, se esperaba %q", got, paths[1])
		}
	})

	t.Run("cerrar una posterior no mueve la activa", func(t *testing.T) {
		w, paths := openThree(t)
		if err := w.SetActive(1); err != nil {
			t.Fatalf("SetActive(1) falló: %v", err)
		}
		if err := w.CloseForce(2); err != nil {
			t.Fatalf("CloseForce(2) falló: %v", err)
		}
		if got, want := w.ActiveIndex(), 1; got != want {
			t.Fatalf("ActiveIndex() = %d, se esperaba %d", got, want)
		}
		if got := w.Active().Path(); got != paths[1] {
			t.Fatalf("Active().Path() = %q, se esperaba %q", got, paths[1])
		}
	})

	t.Run("cerrar la activa del medio adelanta la siguiente", func(t *testing.T) {
		w, paths := openThree(t)
		if err := w.SetActive(1); err != nil {
			t.Fatalf("SetActive(1) falló: %v", err)
		}
		if err := w.CloseForce(1); err != nil {
			t.Fatalf("CloseForce(1) falló: %v", err)
		}
		if got, want := w.ActiveIndex(), 1; got != want {
			t.Fatalf("ActiveIndex() = %d, se esperaba %d", got, want)
		}
		if got := w.Active().Path(); got != paths[2] {
			t.Fatalf("Active().Path() = %q, se esperaba %q", got, paths[2])
		}
	})
}

// TestWorkspaceCloseWithGapBeforeActive cierra una pestaña a DOS posiciones de
// distancia de la activa. Es el caso que distingue un corrimiento correcto de
// "activar el índice donde estaba la cerrada": a una sola posición de distancia
// los dos comportamientos coinciden por casualidad, así que ese test no alcanza
// como detector.
func TestWorkspaceCloseWithGapBeforeActive(t *testing.T) {
	w, paths := openFour(t)

	// La activa es la última (d). Cerrar la primera tiene que dejar activa a d,
	// corrida al índice 2, y no a b en el 0.
	if err := w.CloseForce(0); err != nil {
		t.Fatalf("CloseForce(0) falló: %v", err)
	}

	if got, want := w.ActiveIndex(), 2; got != want {
		t.Fatalf("ActiveIndex() = %d, se esperaba %d", got, want)
	}
	if got := w.Active().Path(); got != paths[3] {
		t.Fatalf("Active().Path() = %q, se esperaba %q", got, paths[3])
	}
}

func TestWorkspaceCloseFirstOfThreeKeepsOrder(t *testing.T) {
	w, paths := openThree(t)

	if err := w.SetActive(0); err != nil {
		t.Fatalf("SetActive(0) falló: %v", err)
	}
	if err := w.CloseForce(0); err != nil {
		t.Fatalf("CloseForce(0) falló: %v", err)
	}

	if got, want := w.Len(), 2; got != want {
		t.Fatalf("Len() = %d, se esperaba %d", got, want)
	}
	// Los buffers conservan su orden relativo, corriendo a la izquierda.
	if got := w.Buffers()[0].Path(); got != paths[1] {
		t.Fatalf("Buffers()[0].Path() = %q, se esperaba %q", got, paths[1])
	}
	if got := w.Buffers()[1].Path(); got != paths[2] {
		t.Fatalf("Buffers()[1].Path() = %q, se esperaba %q", got, paths[2])
	}
	// El buffer que ocupó el lugar del cerrado queda activo.
	if got, want := w.ActiveIndex(), 0; got != want {
		t.Fatalf("ActiveIndex() = %d, se esperaba %d", got, want)
	}
	if w.Active().Path() != paths[1] {
		t.Fatalf("Active().Path() = %q, se esperaba %q", w.Active().Path(), paths[1])
	}
}

func TestWorkspaceCloseLastLeavesEmptyCleanState(t *testing.T) {
	dir := t.TempDir()
	a := wFile(t, dir, "a.txt", "a")
	w := NewWorkspace()
	if err := w.SetRoot(dir); err != nil {
		t.Fatalf("SetRoot falló: %v", err)
	}
	if _, err := w.Open(a); err != nil {
		t.Fatalf("Open falló: %v", err)
	}

	if err := w.CloseForce(0); err != nil {
		t.Fatalf("CloseForce falló: %v", err)
	}
	if w.Active() != nil {
		t.Fatalf("Active() = %v, se esperaba nil", w.Active())
	}
	if got, want := w.ActiveIndex(), -1; got != want {
		t.Fatalf("ActiveIndex() = %d, se esperaba %d", got, want)
	}
	if w.Next() != nil {
		t.Fatalf("Next() = %v, se esperaba nil sobre workspace vacío", w.Next())
	}
	if w.Prev() != nil {
		t.Fatalf("Prev() = %v, se esperaba nil sobre workspace vacío", w.Prev())
	}
	if w.AnyModified() {
		t.Fatal("un workspace vacío no debería reportar buffers modificados")
	}
}

func TestWorkspaceNextPrevWrapAround(t *testing.T) {
	w, paths := openThree(t)

	// Adelante circular desde el índice 2: 2 -> 0 -> 1 -> 2.
	w.SetActive(2)
	for _, want := range []int{0, 1, 2} {
		if got := w.Next(); got.Path() != paths[want] {
			t.Fatalf("Next() = %q (índice esperado %d)", got.Path(), want)
		}
		if got, wantIdx := w.ActiveIndex(), want; got != wantIdx {
			t.Fatalf("ActiveIndex() tras Next = %d, se esperaba %d", got, wantIdx)
		}
	}
	// Hacia atrás circular: 2 -> 1 -> 0 -> 2.
	for _, want := range []int{1, 0, 2} {
		if got := w.Prev(); got.Path() != paths[want] {
			t.Fatalf("Prev() = %q (índice esperado %d)", got.Path(), want)
		}
		if got, wantIdx := w.ActiveIndex(), want; got != wantIdx {
			t.Fatalf("ActiveIndex() tras Prev = %d, se esperaba %d", got, wantIdx)
		}
	}
}

func TestWorkspaceAnyModifiedCoversNonActiveBuffers(t *testing.T) {
	w, _ := openThree(t)

	// El workspace recién abierto no está modificado.
	if w.AnyModified() {
		t.Fatal("workspace recién cargado no debe reportar cambios")
	}

	// Editar un buffer NO activo (el 0; el activo es el 2).
	if err := w.Buffers()[0].Insert(0, "x"); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}
	if !w.AnyModified() {
		t.Fatal("AnyModified no detectó la edición en un buffer no activo")
	}
}

func TestWorkspaceBuffersReturnsCopy(t *testing.T) {
	w, _ := openThree(t)

	bs := w.Buffers()
	bs[0] = nil
	if got, want := w.Len(), 3; got != want {
		t.Fatalf("Len() = %d tras mutar la copia, se esperaba %d", got, want)
	}
	// Y el primer buffer sigue vivo dentro del workspace.
	if got := w.Buffers()[0]; got == nil {
		t.Fatal("Buffers()[0] quedó nil dentro del workspace por mutar la copia")
	}
}

func TestWorkspaceOutOfRangeIndexes(t *testing.T) {
	w, _ := openThree(t)

	if err := w.SetActive(3); err != ErrBufferOutOfRange {
		t.Fatalf("SetActive(3) = %v, se esperaba ErrBufferOutOfRange", err)
	}
	if err := w.SetActive(-1); err != ErrBufferOutOfRange {
		t.Fatalf("SetActive(-1) = %v, se esperaba ErrBufferOutOfRange", err)
	}
	if err := w.Close(3); err != ErrBufferOutOfRange {
		t.Fatalf("Close(3) = %v, se esperaba ErrBufferOutOfRange", err)
	}
	if err := w.CloseForce(7); err != ErrBufferOutOfRange {
		t.Fatalf("CloseForce(7) = %v, se esperaba ErrBufferOutOfRange", err)
	}
}

func TestWorkspaceCloseAllReleasesEverything(t *testing.T) {
	w, _ := openThree(t)

	// Dejar todo sucio: CloseAll no debe mirar el flag de modificación.
	for _, b := range w.Buffers() {
		if err := b.Insert(0, "sucio"); err != nil {
			t.Fatalf("Insert falló: %v", err)
		}
	}

	w.CloseAll()
	if got, want := w.Len(), 0; got != want {
		t.Fatalf("Len() = %d tras CloseAll, se esperaba %d", got, want)
	}
	if w.Active() != nil || w.ActiveIndex() != -1 {
		t.Fatalf("estado tras CloseAll: Active()=%v ActiveIndex()=%d, se esperaba nil/-1",
			w.Active(), w.ActiveIndex())
	}
	if w.AnyModified() {
		t.Fatal("un workspace cerrado no debe reportar buffers modificados")
	}
}

func TestWorkspaceCloseLastBufferMakesPreviousActive(t *testing.T) {
	w, paths := openThree(t)

	if err := w.CloseForce(2); err != nil {
		t.Fatalf("CloseForce(último) falló: %v", err)
	}
	if got, want := w.ActiveIndex(), 1; got != want {
		t.Fatalf("ActiveIndex() = %d, se esperaba %d (el nuevo último)", got, want)
	}
	if w.Active().Path() != paths[1] {
		t.Fatalf("Active().Path() = %q, se esperaba %q", w.Active().Path(), paths[1])
	}
}

func TestWorkspaceSetActiveErrorsOnEmpty(t *testing.T) {
	w := NewWorkspace()
	if w.Len() != 0 {
		t.Fatalf("Len() = %d, se esperaba 0", w.Len())
	}
	if w.Active() != nil || w.ActiveIndex() != -1 {
		t.Fatalf("workspace vacío activo: Active()=%v ActiveIndex()=%d", w.Active(), w.ActiveIndex())
	}
	if err := w.SetActive(0); err != ErrBufferOutOfRange {
		t.Fatalf("SetActive sobre vacío = %v, se esperaba ErrBufferOutOfRange", err)
	}
}

func TestWorkspaceNextPrevSingleBufferStaysPut(t *testing.T) {
	dir := t.TempDir()
	a := wFile(t, dir, "a.txt", "a")
	w := NewWorkspace()
	if err := w.SetRoot(dir); err != nil {
		t.Fatalf("SetRoot falló: %v", err)
	}
	if _, err := w.Open(a); err != nil {
		t.Fatalf("Open falló: %v", err)
	}

	if got := w.Next(); got.Path() != a {
		t.Fatalf("Next() con un solo buffer = %q, se esperaba quedarse en %q", got.Path(), a)
	}
	if got := w.Prev(); got.Path() != a {
		t.Fatalf("Prev() con un solo buffer = %q, se esperaba quedarse en %q", got.Path(), a)
	}
}
