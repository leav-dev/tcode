# Feature: Ancho de Caracteres Correcto

## Description
Corregir el renderizado de columnas usando *grapheme clusters* y ancho real de
caracteres. Hoy `Draw` asume 1 celda por runa, lo que desalinea las columnas con
caracteres anchos (CJK, emoji) y rompe el hit testing del mouse — un requisito
explícito del proyecto ("amigable con el mouse").

## Tasks
- [x] Renderizar por *grapheme cluster* con `uniseg`, usando `Width()` como avance de columna <!-- id: 0 -->
- [x] Pasar el cluster completo a `tcell.Screen.Put` <!-- id: 1 -->
- [x] Expandir tabulaciones al siguiente tab stop <!-- id: 2 -->
- [x] Manejar CRLF y CR como saltos de línea <!-- id: 3 -->
- [x] No dibujar clusters anchos que no entren en el borde derecho <!-- id: 4 -->
- [x] Tests de ancho: CJK, combinantes, emoji ZWJ, tabs, borde y scroll horizontal <!-- id: 5 -->
- [x] Verificación: `go vet`, `gofmt -l`, `go test -race` <!-- id: 6 -->
- [x] Commit de unidad de trabajo — el código quedó integrado en `a33c866` (la
  anotación de identidad quedó pendiente en su momento y se cierra en el commit de
  cierre de U2b) <!-- id: 7 -->

## Evidence

### `internal/view/editor_view.go`
- `Draw` ahora itera con `uniseg.NewGraphemes` y avanza la columna lógica por
  `Width()`, no por runa. La columna pasa a estar medida en celdas de terminal.
- Se reemplazó `s.SetContent(x, row, runes[0], runes[1:], style)` por
  `s.Put(x, row, cluster, style)`.
  - **Hallazgo:** `tcell.Screen.SetContent` está **deprecado** y delega en `Put`,
    pero además hace `string(append([]rune{mainc}, combc...))`: dos allocations
    por celda. `Put` recibe el cluster como `string` y es *grapheme-aware* por
    dentro (usa `uniseg.FirstGraphemeClusterInString`), así que evita ambas.
- El texto se expone con `unsafe.String(&content[0], len(content))`: vista sin
  copia sobre los bytes del `mmap`. Es seguro porque el mapeo es de solo lectura
  y ni `uniseg` ni `tcell` retienen la vista más allá de `Draw`.
- Se usa `uniseg.Graphemes.Str()` y no `.Bytes()`: `Str()` devuelve un substring
  sin alocar, mientras que `Bytes()` hace `[]byte(g.cluster)` y sí aloca.

### Casos borde cubiertos
- **Tab:** ancho = `tabWidth - col%tabWidth`; no se dibuja, la pantalla ya viene limpia.
- **CRLF:** tratado como un único salto de línea (GB3 de UAX #29 une CR y LF).
- **CR aislado:** reinicia la columna sin saltar de fila.
- **Combinante huérfano:** `Width() == 0` y no tiene dónde anclarse, así que se saltea.
- **Borde derecho:** un cluster se dibuja solo si `x+width <= Width`; escribir uno
  ancho en la última celda pisaría la celda de continuación de tcell.

### Falsificación de los tests
Se revirtió temporalmente `Draw` a la versión por runas con `git stash` y se
corrieron los tests nuevos contra la implementación vieja: **9 de 10 fallan**, con
los síntomas exactos del defecto.

| Test | Contra la impl. vieja | Síntoma observado |
|---|---|---|
| `TestWideCJKCharacterAdvancesTwoColumns` | FALLA | `celda (2,0) = ' '` — el ancho consumió 1 columna |
| `TestCombiningMarkSharesTheBaseCell` | FALLA | `combinantes = []` — el acento se llevó celda propia |
| `TestEmojiZWJSequenceCountsAsOneCluster` | FALLA | `celda (2,0) = '👩'` — el cluster ZWJ se partió runa por runa |
| `TestTabExpandsToNextTabStop` | FALLA | `celda (4,0) = 'c'` — los tabs avanzaban 1 columna |
| `TestWideCharacterThatDoesNotFitIsNotDrawn` | FALLA | el ancho se dibujó desbordando el borde |
| `TestHorizontalScrollSkipsByDisplayWidth` | FALLA | `celda (0,0) = 'a'` — scrolleaba por runas |
| `TestLoneCarriageReturnResetsColumnOnSameRow` | FALLA | el CR no reiniciaba la columna |
| `TestStandaloneCombiningMarkDoesNotAdvanceColumn` | FALLA | `celda (0,0) = ' '` |
| `TestMixedWidthLineKeepsColumnsAligned` | FALLA | `celda (3,0) = '☕'` — columnas desalineadas |
| `TestCRLFIsASingleLineBreak` | pasa | **No discrimina:** `Put` ya ignoraba el CR de
ancho cero, así que el código viejo no lo dibujaba por accidente, no por diseño. |

### Verificación
```
go vet ./...              → limpio
gofmt -l .                → limpio
go test -race -count=1    → 30 tests OK (20 previos + 10 nuevos)
```

## Known Limitations
- Sin wrapping de líneas largas: el scroll horizontal es el único recurso.
- El ancho se delega a `uniseg`; las tablas East Asian Ambiguous se resuelven con
  su criterio por defecto, que puede diferir de la fuente configurada en el terminal.
- La columna lógica ya se mide en celdas, pero todavía no existe un mapeo inverso
  (celda → offset del documento) que el hit testing del mouse va a necesitar.
