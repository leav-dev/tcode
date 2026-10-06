# Feature: Topes de geometría y scroll para la ventana flotante de configuración

## Description
La ventana flotante de configuración (`Ctrl+P`) crece verticalmente sin
control: `configRegion()` la dimensiona con `ConfigMenuHeight()`
(`filas + marco`), así que cada fila nueva la agranda y el scroll interno
(cursor/top, ya implementado) nunca entra en acción. Además su ancho es un
fijo de 34 que no crece nunca.

El usuario pidió: (1) que la ventana tenga **scroll** para evitar que crezca
verticalmente sin control y (2) definir un **ancho máximo** que **pueda
crecer**. Valores decididos con el usuario:

- **Alto máximo: 8 filas visibles** (alto 10 con marco). Con más filas, el
  scroll interno navega; con menos, la ventana mide lo que necesita.
- **Ancho máximo: 40 columnas**, con **crecimiento por contenido**: la
  ventana nace en la base actual (34) y se ensancha solo si una fila
  (etiqueta + valor) lo exige, hasta el tope de 40 y sin pasar del editor.

La ventana de extensiones no cambia: ya tiene alto fijo (14) con scroll por
pestaña.

## Tasks
- [ ] `view/config_menu.go`: constantes exportadas `ConfigMenuBaseWidth = 34`,
  `ConfigMenuMaxWidth = 40`, `ConfigMenuMaxHeight = 10` (8 filas visibles +
  marco); `ConfigMenuContentWidth()` (fila más ancha + marco, para el
  crecimiento por contenido); accessor `Top()` para los tests del reencuadre
  (patrón `ExtManager.topFor`) <!-- id: 1 -->
- [ ] Controller `configRegion`: `menuH = min(ConfigMenuHeight(),
  ConfigMenuMaxHeight)` recortado al alto del editor (el scroll entra al
  superar las 8 filas); `menuW = min(max(content, base), max)` recortado al
  ancho del editor; comentario actualizado <!-- id: 2 -->
- [ ] Tests view (`config_menu_test.go`): constantes de geometría y
  `ConfigMenuContentWidth` (fila más ancha + marco, menor a la base con las
  filas actuales); scroll con alto limitado (`Resize` chico + `End`/`Down`
  reencuadra `top` para que la fila del cursor quede visible) <!-- id: 3 -->
- [ ] Tests controller (`app_config_test.go`): `configRegion` con editor sano
  (34x7 con las filas actuales); recorte al editor angosto (ancho) y bajo
  (alto) <!-- id: 4 -->
- [ ] Docs: `docs/config.md` — scroll con más de 8 filas y ancho máximo de
  40 con crecimiento por contenido <!-- id: 5 -->
- [ ] Verificación (gofmt/vet/go test, paquetes view y controller) y commit de
  unidad del trabajo <!-- id: 6 -->

## Design decisions
- **El tope de alto vive en view, la combinación en el controlador.** Igual
  que `ExtManagerHeight()` + `extRegion()`: la vista expone su contenido y
  sus topes; el controlador recorta al editor. Toda la geometría de contenido
  (filas, valores) es de la vista.
- **La base de ancho es 34 (el ancho actual).** El crecimiento por contenido
  solo ensancha si una fila supera la base; con las filas actuales la ventana
  queda en 34, como hoy. El tope de 40 corta filas futuras largas.
- **El scroll no se toca: solo se habilita.** cursor/top/clamp/
  ensureCursorVisible/page ya existen y funcionan; al limitar el alto de la
  ventana dejan de ser letra muerta. `Top()` los hace testeables (patrón de
  `ExtManager.topFor`).
- **Recorte al editor intacto:** nunca más ancha que el editor ni más alta
  que su área, como hoy.
