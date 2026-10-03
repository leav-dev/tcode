# Feature: Auto-indentación

## Description
Deuda grande de `docs/memory.md` ("sin auto-indentación"). Al presionar
`Enter`, el editor inserta una línea nueva con la **indentación heredada** de la
línea de origen y, si esa línea abre un bloque, un nivel más.

Decisiones de producto (usuario):

- **Nivel extra si la línea de origen termina en `{`, `[` o `:`** (el caso
  C-like `{`/`[` y el de Python `:`). Es una regla **mecánica**: se mira el
  último carácter no-espacio de la línea de origen, sin análisis sintáctico (un
  `:` en un slice o diccionario también indenta; se documenta como límite de
  esta versión).
- **Unidad estándar del editor**: `var indentUnit` — modificable (la futura
  configuración del editor la podrá exponer), por defecto **4 espacios** ("el
  tamaño del tab"). Coherente con el `tabWidth = 4` del proyecto.
- **`Tab` inserta `indentUnit`** en lugar de `\t`: el "tab del editor" es la
  unidad (4 espacios por defecto). Es la consecuencia natural de la decisión y
  hace que el nivel extra y la tecla Tab coincidan.
- El prefijo heredado se copia EXACTO de la línea de origen (tabs o espacios
  como estén); el nivel extra usa siempre `indentUnit`. Si la línea usa tabs y
  `indentUnit` es espacios, el nivel extra mezcla — comportamiento documentado;
  quien quiera tabs cambia `indentUnit` a `"\t"`.

Qué NO cambia: Backspace por cluster (borra la indentación heredada como
cualquier texto), el movimiento, el borrado de líneas. `Enter` al final del
documento o en una línea vacía: hereda "" (una línea vacía no indenta nada).

## Tasks
- [ ] `view`: `var indentUnit` (default `"    "`) + `autoIndent()`: prefijo de
  whitespace de la línea de origen; si su cola termina en `{`/`[`/`:` suma
  `indentUnit`; `handleKey` Enter/LF inserta `"\n"+indent` y Tab inserta
  `indentUnit` <!-- id: 0 -->
- [ ] Tests de vista: hereda el prefijo exacto; nivel extra tras `{`, `[`, `:`;
  línea plana sin indent; línea vacía; Enter al final; Tab inserta `indentUnit`
  y respeta el cambio de la variable <!-- id: 1 -->
- [ ] Verificación: suite completa sin fallos nuevos vs base + `go vet`/`gofmt`
  <!-- id: 2 -->

## Design decisions

### Regla mecánica, sin parser
Detectar bloques "de verdad" exige saber si el `{` está en un string, un
comentario o un slice — eso es un lexer por lenguaje. Esta versión usa la regla
observable que da el 90% del valor: la línea de origen termina (sin whitespace
de cola) en `{`, `[` o `:` → un nivel más. El exceso (indentar tras un `:` de
slice o un `{` en cadena) se corrige con Backspace y se anota como límite.

### La línea de origen es la que se rompe, no la anterior
`Enter` rompe la línea actual: el prefijo heredado y la regla de bloque miran
ESA línea (donde está el cursor), no la de arriba. Es lo que hace que Enter en
el medio de una línea indentada mantenga el nivel.

### `indentUnit` como var, no const
La futura configuración del editor (comando, settings) la podrá cambiar; los
tests la bajan/alteran para fijar el comportamiento.

## Falsificación (tests que escriben primero contra el código roto)
1. Sin `autoIndent` → `TestEnterInheritsIndent` falla (la línea nueva queda sin
   el prefijo).
2. Sin nivel extra → `TestEnterAfterOpenBraceAddsIndent` falla.
3. Sin regla `:` → `TestEnterAfterColonAddsIndent` falla.
4. Con Tab insertando `\t` → `TestTabInsertsIndentUnit` (espacios) falla.

## Evidence
(Rellenar por unidad.)