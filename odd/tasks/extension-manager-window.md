# Feature: Ventana flotante de gestión de extensiones

## Description
Hoy la gestión de extensiones vive solo en la CLI (`--install-extension`, `--list-extensions`,
`--add-provider`, …). El usuario quiere hacerlo desde adentro del editor: una fila en la
ventana de configuración (`Ctrl+P`) que abra una **ventana flotante** con las extensiones
instaladas, las actualizables y las disponibles en los proveedores, más la opción de
agregar proveedores y extensiones.

## Decisiones de diseño (confirmadas con el usuario)

| # | Decisión |
| --- | --- |
| 1 | **Pestañas** por sección: `Instaladas` / `Actualizables` / `Disponibles` / `Proveedores`. Left/Right cambia de pestaña, Up/Down mueve el cursor. |
| 2 | Enter actúa **según la fila**: Disponible → instala; Actualizable → actualiza; Instalada → borra. |
| 3 | **Confirmación sí/no** antes de instalar, actualizar y borrar (los tres traen o borran código). |
| 4 | **Agregar proveedor** desde la ventana, con el prompt de texto (URL o ruta de carpeta). |

## Decisiones de arquitectura (del agente)

- `ConfigKind` gana `ConfigAction`: una fila que no muta un valor sino que dispara algo.
  `configItem` gana `action string`; `ConfigMenu` gana `Activated() string` (devuelve y
  limpia la acción disparada), porque `HandleEvent` devuelve `(handled, changed)` y no
  alcanza para señalar "abrí la ventana de extensiones".
- La fila nueva va al final: `Extensiones`. `ConfigMenuHeight()` pasa de 6 a 7.
- `view/ext_manager.go`: la ventana nueva. Un cursor por pestaña, `top` con scroll mínimo
  (la mecánica del menú de pestañas y de la config), frame con las pestañas en el borde
  superior y la del cursor marcada.
- Los datos se cargan **al abrir** la ventana (no en cada tecla): `ext.List` (instaladas),
  `ext.CheckUpdates` (actualizables, con la versión vieja→nueva), `ext.AvailableExtensions`
  (disponibles) y `ext.AllProviders` (proveedores).
- Las acciones reusan la maquinaria de la CLI: `ext.InstallByID`, `ext.UpdateAll` /
  `installSubdir`, `ext.RemoveNamespaced` / `RemoveRef`, `ext.CanonicalSource` +
  `DeriveName` + `SaveProviders`. **La ventana no reimplementa instalación**: llama a las
  mismas funciones.
- Confirmación con `openPrompt` (el prompt generalizado). El prompt se chequea ANTES que
  el manager en `handleEvent`, así que el flujo es: manager → Enter → prompt → respuesta
  → acción → el manager sigue abierto con los datos recargados.
- Los proveedores sin aprobar se marcan en la fila y su instalación no se ofrece sin
  aprobación (coherente con el modelo de confianza ya implementado).

## Tasks
- [ ] `internal/view/config_menu.go`: `ConfigAction`, campo `action`, fila `Extensiones`, `Activated()`; `configValueText` para la fila de acción; `ConfigMenuHeight` 6→7 <!-- id: 0 -->
- [ ] `internal/view/ext_manager.go`: la ventana con pestañas (`Instaladas`/`Actualizables`/`Disponibles`/`Proveedores`), cursor+top+scroll mínimo, `SetItems`, `HandleEvent` (devuelve la intención), `Draw` con frame y pestañas; `ExtManagerHeight()` para el dimensionado <!-- id: 1 -->
- [ ] `internal/controller/app.go`: abrir/cerrar la ventana desde `Activated()`; cargar datos al abrir; manejar eventos; `redraw`; las acciones (`install`/`update`/`remove`/`add-provider`) con confirmación y recarga <!-- id: 2 -->
- [ ] Tests: la fila de acción dispara `Activated`; `ExtManager` (pestañas, cursor, intención por Enter); el flujo de instalar/borrar con confirmación <!-- id: 3 -->
- [ ] Docs: README + docs/extension-system.md (la ventana y sus pestañas) <!-- id: 4 -->

## Evidence
- `configItem` + `configItems()` (4 filas fijas) y `ConfigMenu.HandleEvent` → `(handled, changed)`.
- El patrón de overlay: `configActive` + `HandleEvent` + `Draw(surf)` + `configRegion()`.
- Los datos: `ext.List`, `CheckUpdates`, `AvailableExtensions`, `AllProviders`, `InstallByID`, `RemoveNamespaced`.
- `openPrompt` (label + prefill + action) ya soporta el sí/no y el texto libre.
