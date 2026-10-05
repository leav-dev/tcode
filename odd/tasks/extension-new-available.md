# Feature: Detectar y auto-instalar extensiones nuevas de proveedores

## Description
El proveedor agrega extensiones nuevas al repo pero el editor no las detecta:
`UpdateAll` solo refresca las ya instaladas. El usuario quiere que el editor
detecte las novedades y las instale (o las reporte) al arrancar.

## Decisiones de diseño (confirmadas con el usuario)

| # | Decisión |
| --- | --- |
| 1 | Extensión nueva de un proveedor **aprobado** → se **auto-instala** al arrancar |
| 2 | Extensión nueva de un proveedor **no aprobado** → se **reporta** al arrancar, no se instala |

## Decisiones de arquitectura (del agente)

- **Detección**: `AvailableExtensions(providers, userRoot, fetcher)` enumera el
  catálogo de cada proveedor (`ListExtensions`, liviano) y diff con los
  instalados (`List`). Lo que ofrece pero no está instalado = novedad.
- **Auto-instalación**: `InstallAvailable(available, userRoot, fetcher)`
  instala las novedades de proveedores aprobados (reutiliza `withProviderRoot`
  + `installSubdir`). Las no aprobadas se saltan.
- **Orden**: primero `UpdateAll` (refresca las instaladas), después
  `AvailableExtensions` (diff contra el set actualizado), después
  `InstallAvailable` (auto-instala las aprobadas).
- **Reporte**: las actualizadas, las instaladas y las novedades sin aprobar
  se reportan por stdout; los errores por stderr. Nunca es fatal.

## Tasks
- [ ] `internal/ext/install.go`: `AvailableExt` + `AvailableExtensions` + `InstallAvailable` <!-- id: 0 -->
- [ ] `main.go`: extender `updateExtensionsAtStartup` — encontrar nuevas, auto-instalar aprobadas, reportar no aprobadas <!-- id: 1 -->
- [ ] Tests: `AvailableExtensions` (novedad detectada, ya instalada no) + `InstallAvailable` (aprobada instala, no aprobada salta) <!-- id: 2 -->
- [ ] Docs: README <!-- id: 3 -->

## Evidence
- `ListExtensions(p, fetcher)` devuelve todo lo que ofrece un proveedor (liviano).
- `List(userRoot)` devuelve las instaladas con su proveedor.
- `installSubdir` ya hace re-clonar + redeployar reemplazando.
- `UpdateAll` solo recorre las instaladas: una extensión nueva del proveedor no está en `List(userRoot)` y por eso no se detecta hoy.
