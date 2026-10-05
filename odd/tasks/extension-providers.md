# Feature: Proveedores de extensiones

## Description
Hoy `tcode --install-extension <url>` clona **un** repo que tiene `extension.json`
en su raíz y lo despliega en `~/.tcode/extensions/<id>/`. No hay forma de
resolver extensiones por id ni de registrar fuentes: el autor del editor quiere
un sistema de **proveedores** (fuentes de extensiones) con un proveedor por
defecto y la posibilidad de agregar otros.

El proveedor por defecto es `https://github.com/leav-dev/tcode-extention` (un
monorepo: cada subcarpeta con `extension.json` es una extensión). El editor lo
registra pero **no instala nada**; el usuario elige qué instalar.

## Decisiones de diseño (confirmadas con el usuario)

| # | Decisión |
| --- | --- |
| 1 | Agregar proveedor: `tcode --add-provider <url\|carpeta>` → valida y persiste en `~/.tcode/providers.json` |
| 2 | Instalar por id: `tcode --install-extension <id>` → resuelve en todos los proveedores; ante colisión de ids gana el proveedor por defecto |
| 3 | El proveedor por defecto **solo se registra**: el editor lo conoce pero no instala nada |
| 4 | Destino namespaced: `~/.tcode/extensions/<proveedor>/<id>/` |
| 5 | Proveedor de archivo local = **carpeta local con subcarpetas** (como el monorepo pero en disco) |
| 6 | Confianza: **aprobación explícita por proveedor** — al agregar, `approved: false`; instalar desde un proveedor no aprobado pide confirmación |
| 7 | **Listado liviano**: listar/buscar extensiones de un proveedor NO baja los `.lua`, solo los `extension.json`. Clone parcial (`--filter=blob:none` + sparse checkout de `*/extension.json`). La instalación sí baja los archivos de la extensión elegida |

### Decisiones de arquitectura (del agente)

- **Proveedor por defecto built-in**: constante en `internal/ext`, siempre
  primero en la resolución, siempre aprobado, **no** está en el config.
- **Identidad del proveedor** (para el namespacing): URL git → último
  componente de la ruta (`tcode-extention`); carpeta local → nombre de la
  carpeta (`mis-extensions`). Riesgo documentado: dos proveedores con el mismo
  nombre colisionan; gana el primero en la resolución.
- **Config `~/.tcode/providers.json`**:
  ```json
  { "providers": [
    { "name": "tcode-extentions", "source": "https://github.com/...", "approved": false },
    { "name": "mis-extensions", "source": "/home/sistmas/mis-extensions", "approved": true }
  ] }
  ```
- **Listar extensiones de un proveedor**: git → clone **parcial**
  (`--depth 1 --filter=blob:none --no-checkout` + `sparse-checkout set
  '*/extension.json'`) a temp y escanear los manifests; local → escanear
  directo. Verificado empíricamente: 0 archivos `.lua` descargados, los 5
  manifests presentes. La API de GitHub (árbol + contents) es una optimización
  futura para URLs de GitHub (no clona nada, pero es GitHub-specific y tiene
  rate limit).
- **`--list-extensions`**: muestra `proveedor/id (nombre) vversión`.
- **`--remove-extension`**: acepta `<proveedor>:<id>` o busca en todos.

## Tasks
- [ ] `internal/ext/provider.go`: `Provider` struct, constante del proveedor por
  defecto, `LoadProviders`/`SaveProviders` sobre `~/.tcode/providers.json`,
  derivación del nombre desde la fuente <!-- id: 0 -->
- [ ] `internal/ext/provider.go`: `ListExtensions` — git (clone **parcial** + scan
  de manifests, sin `.lua`) y local (scan directo); devuelve
  `[]ProviderExt{id, name, version, subdir}` <!-- id: 1 -->
- [ ] `main.go`: flag `--add-provider <url|carpeta>` — valida la fuente, deriva
  el nombre, persiste con `approved: false` <!-- id: 2 -->
- [ ] `internal/ext/install.go`: instalación namespaced
  (`~/.tcode/extensions/<proveedor>/<id>/`) + resolución por id a través de
  `ListExtensions` de cada proveedor <!-- id: 3 -->
- [ ] Confianza: campo `approved`, prompt de aprobación al instalar desde un
  proveedor no aprobado, flag `--approve-provider <name>` <!-- id: 4 -->
- [ ] Adaptar `--list-extensions` (muestra proveedor) y `--remove-extension`
  (acepta `<proveedor>:<id>` o busca en todos) <!-- id: 5 -->
- [ ] Tests para provider.go, install.go y los flags nuevos <!-- id: 6 -->
- [ ] Docs: `docs/extension-system.md` (sección de proveedores) + README <!-- id: 7 -->

## Evidence
- `tcode --install-extension https://github.com/leav-dev/tcode-extention`
  falla hoy con `el repositorio no tiene extension.json en su raíz` (verificado
  empíricamente): el repo es un monorepo, no una extensión.
- Los 5 manifests del repo remoto validan contra `internal/ext/manifest.go`.
- El README del repo remoto anuncia el milestone: "A future catalog/install-by-id
  milestone may let the CLI resolve extensions from this repo directly."
