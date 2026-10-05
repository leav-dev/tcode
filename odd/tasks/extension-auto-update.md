# Feature: Actualización automática de extensiones al arrancar

## Description
El repo de un proveedor se actualizó pero la extensión instalada en
`~/.tcode/extensions/<proveedor>/<id>/` quedó vieja: la instalación es un clon
de un solo sentido y no hay sincronización. El editor no detecta los cambios
remotos.

El usuario quiere que el editor actualice las extensiones instaladas
automáticamente al arrancar.

## Decisiones de diseño (confirmadas con el usuario)

| # | Decisión |
| --- | --- |
| 1 | **Auto al arrancar**: el editor verifica y actualiza solo al arrancar |
| 2 | **Todas**: actualiza todas las extensiones instaladas |
| 3 | **Solo si cambió la versión**: compara la versión del manifest instalado con la del proveedor; si es la misma, saltea |

## Decisiones de arquitectura (del agente)

- **La copia instalada no tiene `.git`** (`copyTree` lo excluye), así que es
  imposible hacer `git fetch` localmente. Para saber si el remoto cambió, el
  editor vuelve a leer el proveedor (liviano, solo manifests) y compara
  versiones.
- **El chequeo corre ANTES de `loadExtensions`**: así el editor arranca con la
  versión nueva ya en disco, sin tener que recargar extensiones a mitad del
  arranque.
- **La comparación es por versión** (campo `version` del manifest): si el
  autor no sube la versión, el cambio no se detecta. Es la limitación
  documentada de la decisión 3.
- **Tolerante**: un proveedor caído o inalcanzable no impide revisar los
  demás, y nunca impide el arranque. Los errores se reportan.
- **Sin confirmación**: actualizar desde un proveedor ya aprobado (de ahí salió
  la instalación) no vuelve a pedir confianza.

## Tasks
- [ ] `internal/ext/install.go`: `UpdateResult` + `UpdateAll(providers, userRoot, fetcher)` — compara versiones, re-clona y redeploya las que cambiaron, acumula errores <!-- id: 0 -->
- [ ] `main.go`: `updateExtensionsAtStartup()` — corre el chequeo antes de `NewApp`, reporta por stdout/stderr <!-- id: 1 -->
- [ ] Tests: misma versión (skip), versión distinta (update), proveedor ausente, extensión ausente, múltiples instaladas <!-- id: 2 -->
- [ ] Docs: README (comportamiento de auto-actualización) <!-- id: 3 -->

## Evidence
- `installSubdir` ya hace re-clonar + redeployar reemplazando (reinstalar
  actualiza): valida el manifest, borra el destino y copia con `skipGit`.
- `List(userRoot)` devuelve `Info{ID, Name, Version, Provider}` — la versión
  instalada y el proveedor de cada una.
- `ListExtensions(p, fetcher)` lee los manifests del proveedor (liviano).
- Hoy hay una instalada: `tcode-extention/tcode.errordetector v2.0.0`.
