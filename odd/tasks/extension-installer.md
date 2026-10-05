# Feature: Instalador de extensiones desde repositorios git

## Description
El usuario quiere instalar **sus propias extensiones**, definidas en otros
repositorios suyos ("verificadas por mí, se consideran seguras"): no hay
marketplace, ni verificación de firma, ni anillos de confianza — el origen es
una **URL de repositorio git** y la validación es estructural del manifest.

Comandos nuevos de línea de comandos (headless, antes de abrir la UI):
- `tcode --install-extension <url-git>` → clona (`git clone --depth 1`),
  valida `extension.json` en la raíz del clon con `ext.Load`, y despliega TODO
  el clon (salvo `.git`) a `~/.tcode/extensions/<id>/` (raíz de usuario; si el
  id ya existe, se reemplaza). Rechaza manifiestos rotos sin tocar destino.
- `tcode --list-extensions` → lista id, nombre y versión de las extensiones de
  la raíz de usuario.
- `tcode --remove-extension <id>` → borra `~/.tcode/extensions/<id>` (el id se
  valida para descartar traversal).
- La extensión queda disponible en la **próxima sesión** (el arranque ya las
  descubre); no hay recarga en caliente.

## Tasks
- [x] `internal/ext/install.go`: `InstallFromGit(url, userRoot, cloner)`,
  `Remove(userRoot, id)`, `List(userRoot) ([]Info, []error)`; `Info{ID, Name,
  Version}`; `var clone` inyectable (tortuoso para tests sin git);
  sanitización de ids y mensajes claros de error (sin git, URL inválida,
  repo sin extension.json, manifest inválido) <!-- id: 0 -->
- [x] `main.go`: parseo de los tres flags antes del path; mode headless que
  ejecuta la operación, imprime resultado en consola y `os.Exit(0|1)`; la raíz
  de usuario se resuelve con un helper del controller <!-- id: 1 -->
- [x] Tests (`internal/ext/install_test.go`): cloner fake (copia un árbol
  local) → instala con id correcto, excluye `.git`, manifest inválido no toca
  destino, sin manifest → error, reemplazo sobre id existente, id con ".."
  rechazado; `List` ignora carpetas rotas; `Remove` acotado; test de
  integración con git real en `file://` con `t.Skip` si git no está
  <!-- id: 2 -->
- [x] Docs: sección "Instalar extensiones" en `docs/extension-system.md` (los
  tres comandos, estructura esperada del repo: `extension.json` en la raíz,
  nota de confianza del autor, disponible en la próxima sesión) y README
  <!-- id: 3 -->
- [x] Verificación y commit: `ad116df` <!-- id: 4 -->

## Design decisions
- **Repos del autor = confiados por definición**: sin checksums ni firmas; la
  barrera es estructural (manifest válido). Documentado con el modelo.
- **`clone` inyectable**: los tests no dependen de git/red; el integration
  test con `file://` corre solo si `git` existe (skip limpio).
- **El despliegue copia el clon completo** (sin `.git`): los futuros hooks con
  scripting y los assets auxiliares de la extensión viajan con ella.
- **Raíz de usuario**: instalada una vez, disponible en todos los proyectos
  (las del proyecto siguen igual en `.tcode/extensions` del workspace).
- **Sin recarga en caliente**: la instalación termina y la extensión se ve al
  reiniciar; simple y predecible (candidato de futuro, no ahora).