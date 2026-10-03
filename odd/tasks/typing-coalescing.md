# Feature: Agrupación de Tipeo en el Historial

## Description
Escribir una palabra dejaba **una entrada de historial por carácter**, así que
deshacer `"hola"` costaba cuatro `Ctrl+Z`. Las inserciones contiguas de una runa se
fusionan ahora en un solo `Change`.

Commit: `2ce807c`.

## Tasks
- [x] Fusionar inserciones contiguas en `record` <!-- id: 0 -->
- [x] Cortar el grupo en el salto de línea <!-- id: 1 -->
- [x] Cortar el grupo en el movimiento del cursor <!-- id: 2 -->
- [x] Cortar el grupo en deshacer y rehacer <!-- id: 3 -->
- [x] Tests de fusión, corte y reversibilidad <!-- id: 4 -->
- [x] Verificación: `go vet`, `gofmt -l`, `go test -race` <!-- id: 5 -->
- [x] Commit de unidad de trabajo <!-- id: 6 -->

## Design decisions

### La regla de fusión es deliberadamente angosta
Se fusionan dos cambios solo si **los dos** son inserciones puras, son **contiguos**
(lo nuevo empieza exactamente donde terminó lo anterior) y **ninguno contiene un
salto de línea**. Así el Enter sigue siendo un paso propio y nada se fusiona por
encima de un salto.

### La contigüidad sola no alcanza
Moverse con el cursor y **volver a la misma posición** produce inserciones
contiguas. Sin un corte explícito, lo escrito antes y después del movimiento se
fusionaría. Por eso el grupo también se corta:

- **en el movimiento del cursor**, llamado desde cada manejador de movimiento de la
  vista;
- **en deshacer y rehacer**, marcado dentro del modelo. Si no, lo que se escribe
  después de deshacer se fusionaría con un cambio que ya quedó atrás en el
  historial, y el paso de deshacer siguiente borraría texto de dos momentos
  distintos.

### Se fusiona en `record`, no en `insertRaw`
`insertRaw` sigue manteniendo el índice de líneas en cada inserción, sin importar si
después se fusiona o no. La fusión es una decisión de historial, no de documento.

## Evidence

### Cuatro tests necesitaron cortar el grupo explícitamente
`TestUndoAndRedoAreLIFO`, `TestBranchingAfterUndoKeepsTheDocumentDirty` y los dos
tests diferenciales alimentaban inserciones **contiguas** asumiendo una entrada de
historial por llamada. Al fusionarse, las pilas de deshacer y de estados de
referencia quedaron desfasadas.

No era un bug del modelo sino de los tests. Los dos diferenciales ahora cortan el
grupo **por edición a propósito**: verificar que las inversas son exactas es su
trabajo, no la política de fusión, que tiene sus propios tests.

### Un test mío tenía las expectativas corridas
`TestDeletingBreaksTheTypingGroup` esperaba `"ab"` tras un deshacer, pero un
deshacer quita la `"c"` insertada después: el contenido correcto era `"a"`. La
corrección dejó el test **mejor**, porque ahora muestra los tres pasos separados:
la ráfaga `"ab"`, el borrado y la inserción posterior.

### Verificación
```
go vet ./...              → limpio
gofmt -l .                → limpio
go test -race -count=1    → pasa (el total subió a 164 tras esta unidad)
```

## Known Limitations
- Un `Insert` programático de varias runas (por ejemplo de un agente) seguido de
  tipeo contiguo se fusiona con él. No afecta al uso por teclado, donde cada
  inserción es de una runa.
- El historial sigue sin tope.
- El borrado va por runa en los tests sintéticos, aunque la edición real use
  *grapheme clusters*.
