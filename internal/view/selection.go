package view

// Selection es un rango [Start, End) de offsets de documento marcado por el
// usuario. El archivo es texto; la selección es estado VISUAL de la vista,
// como el cursor, y el modelo no la conoce (solo se le pide el texto del rango
// al copiar).
type Selection struct {
	Start int
	End   int
}

// SelectionActive dice si hay un rango marcado. Lo usa el controller para
// decidir si Ctrl+C copia (en vez de salir del editor).
func (v *EditorView) SelectionActive() bool {
	return v.sel.Start != v.sel.End
}

// SelectionText devuelve el texto marcado, o "" sin selección.
func (v *EditorView) SelectionText() string {
	if !v.SelectionActive() {
		return ""
	}
	return v.model.TextRange(v.sel.Start, v.sel.End)
}

// SelectionRange devuelve el rango marcado (start ≤ end), y false sin marca.
func (v *EditorView) SelectionRange() (int, int, bool) {
	if !v.SelectionActive() {
		return 0, 0, false
	}
	return v.sel.Start, v.sel.End, true
}

// beginExtend prepara la extensión al mover con Shift: sin ancla previa, queda
// en la posición actual del cursor (la marca arranca ahí y el rango empieza
// vacío en ese punto).
func (v *EditorView) beginExtend() {
	if v.selAnchor == nil {
		a := v.cursorOffset()
		v.selAnchor = &a
	}
}

// updateSelection extiende el rango desde el ancla hasta el cursor (tras
// mover): Start/End siempre en orden.
func (v *EditorView) updateSelection() {
	if v.selAnchor == nil {
		return
	}
	a, c := *v.selAnchor, v.cursorOffset()
	if a <= c {
		v.sel.Start, v.sel.End = a, c
	} else {
		v.sel.Start, v.sel.End = c, a
	}
}

// clearSelection descarta el rango y el ancla (movimiento sin Shift).
func (v *EditorView) clearSelection() {
	v.sel = Selection{}
	v.selAnchor = nil
}

// selectAll marca el documento completo y deja el cursor al final.
func (v *EditorView) selectAll() bool {
	docLen := v.model.Len()
	v.setCursorAt(docLen)
	v.sel.Start, v.sel.End = 0, docLen
	v.selAnchor = nil
	return true
}

// moveWithShift ejecuta move y maneja la selección según el shift: con shift
// extiende desde el ancla; sin shift limpia. Devuelve si el movimiento cambió.
func (v *EditorView) moveWithShift(shift bool, move func() bool) bool {
	if shift {
		v.beginExtend()
	}
	changed := move()
	if shift {
		v.updateSelection()
	} else {
		v.clearSelection()
	}
	return changed
}
