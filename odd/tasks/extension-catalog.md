# Feature: Catálogo de extensiones + panel de instalación en la ventana de ajustes

## Description
El usuario quiere instalar extensiones desde su monorepo
`https://github.com/leav-dev/tcode-extention` (5 extensiones en subcarpetas,
sin índice). En la ventana de ajustes (`Ctrl+P`) habrá una opción **Extensions**
que abre un panel con:
- la lista de extensiones disponibles (id, nombre, versión, si ya está instalada);
- **selección múltiple** (Espacio marca/desmarca);
- un **item al final** "Install N selected" que instala todas las marcadas;
- **scroll en ambos sentidos**: `Up` mueve el cursor arriba y desplaza la lista,
  `End` salta al final (la lista es más larga que la ventana).

El catálogo se deriva de la API de GitHub (carpetas con `extension.json`), sin
que el repo remoto mantenga un índice. La instalación clona el repo y copia la
subcarpeta de la extensión (extensión nueva: instalar por subdirectorio).

## Hitos
- **A (backend):** `ext.FetchCatalog` (descubre carpetas vía la API de GitHub y
  lee cada `extension.json` desde raw, con timeouts y errores claros) y
  `ext.InstallFromGitSubdir(url, subdir, …)` reutilizando el núcleo de
  `InstallFromGit`.
- **B (UI):** item **Extensions** en el `ConfigMenu` (acción que abre el panel),
  `ExtensionsPanel` (lista con cursor/scroll, multiselección, item final de
  instalación), y el controlador: descarga del catálogo en goroutine (la UI no se
  congela), instalación de las marcadas con aviso en la barra, refresco de
  instaladas y vuelta a la ventana de ajustes al cerrar.

## Tasks
- [ ] `internal/ext/catalog.go`: `CatalogEntry{ID, Name, Version, Subdir,
  Installed}` + `FetchCatalog` (base API y base raw inyectables para tests,
  `http.Client` con timeout, filtro de directorios y manifests inválidos) <!-- id: 0 -->
- [ ] `internal/ext/install.go`: `InstallFromGitSubdir` (clona, valida
  `extension.json` de la subcarpeta, copia esa subcarpeta, reemplazo) <!-- id: 1 -->
- [ ] Tests del backend: catálogo con servidor HTTP falso (entradas válidas,
  carpetas sin manifest, manifest roto), instalación por subdir con cloner fake
  (id correcto, sin `.git`, manifest inválido no toca destino) <!-- id: 2 -->
- [ ] `internal/view/extensions_panel.go`: panel con cursor, scroll (Up/Down/
  Home/End/PgUp/PgDn), multiselección (Espacio), item final "Install N
  selected", marca de instalada, carga/error/caché vacío <!-- id: 3 -->
- [ ] `internal/view/config_menu.go`: `ConfigKind` de acción + item
  "Extensions" al final (Enter lo activa; Left/Right no aplican) <!-- id: 4 -->
- [ ] `internal/controller/app.go`: apertura del panel desde la ventana de
  ajustes, catálogo en goroutine con wake-up por evento, instalación de las
  marcadas (resultados en la barra), refresco y cierre; test de integración con
  catálogo falso e instalación por HTTP/git inyectados <!-- id: 5 -->
- [ ] Verificación, docs (`docs/extension-system.md` + `docs/memory.md`) y commit <!-- id: 6 -->

## Design decisions
- **El catálogo se deriva, no se declara:** API de GitHub (tree) + raw por
  manifest; el repo remoto no necesita índice. Se acepta un campo opcional
  `description` en el manifest para la lista; si no está, se muestra el nombre.
- **Sin bloqueo de UI:** la consulta va en goroutine con wake-up por evento
  (`PostEvent`), como los chequeos externos ya periúdicos; el panel muestra
  "Consultando catálogo…" mientras tanto.
- **Selección múltiple explícita:** Espacio marca; el item final instala lo
  marcado (o el del cursor si no hay nada marcado) — nunca instala de un
  tirón una extensión sin confirmar.
- **Ya instaladas:** se muestran como tal y reinstalar es *replace* (mismo
  contrato del CLI).