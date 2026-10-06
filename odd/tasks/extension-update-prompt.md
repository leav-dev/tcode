# Feature: Prompt de novedades y actualizaciones al arrancar

## Description
El chequeo de extensiones al arrancar imprime por stdout (invisible) y auto-instala
en silencio. El usuario quiere que las novedades y actualizaciones sean visibles y
pueda aceptar o denegar con un prompt de sí/no al arrancar.

## Decisiones de diseño (confirmadas con el usuario)

| # | Decisión |
| --- | --- |
| 1 | El prompt cubre NOVEDADES + ACTUALIZACIONES: el usuario revisa todo antes de aplicar |
| 2 | La decisión se presenta como un PROMPT DE SÍ/NO al arrancar: "Aplicar N actualizaciones y M novedades? [s/N]". Aceptar aplica, denegar omite |
| 3 | Denegar solo omite esta vez; el próximo arranque vuelve a preguntar |

## Decisiones de arquitectura (del agente)

- El chequeo se mueve del main.go al controller (App), para que sea visible.
- El prompt se muestra ANTES de loadExtensions: así, al aceptar, las actualizaciones y
  novedades se aplican antes de cargar, sin recargar extensiones (AddExtensions no es
  idempotente: appendea a `m.states` y el registry rechaza el segundo registro).
- Se usa un loop anidado de eventos para el prompt de arranque (como el config menu,
  pero antes del loop principal).
- `CheckUpdates` (nuevo) detecta actualizaciones SIN aplicar; `UpdateAll` se refactoriza
  para usar la misma detección y aplicar. `AvailableExtensions` ya detecta novedades.
- `updateExtensionsAtStartup` de main.go se elimina (el chequeo vive en el controller).

## Tasks
- [ ] `internal/ext/install.go`: refactor `UpdateAll` para usar `detectUpdates` compartido; agregar `CheckUpdates` (detección sin aplicar) <!-- id: 0 -->
- [ ] `internal/controller/app.go`: `checkExtensionsAndPrompt()` — detecta actualizaciones + novedades, muestra prompt sí/no, aplica al aceptar; loop anidado; llamado antes de `loadExtensions` <!-- id: 1 -->
- [ ] `main.go`: eliminar `updateExtensionsAtStartup` (el chequeo vive en el controller) <!-- id: 2 -->
- [ ] Tests: `CheckUpdates` (detecta sin aplicar), flujo del prompt (aceptar aplica, denegar omite) <!-- id: 3 -->
- [ ] Docs: README + docs/extension-system.md <!-- id: 4 -->

## Evidence
- `updateExtensionsAtStartup` en main.go imprime por stdout antes de la TUI: por eso
  las notificaciones no se ven.
- `Manager.AddExtensions` no es idempotente: appendea a `m.states` y registra comandos;
  el registry rechaza el segundo registro. Por eso el prompt va ANTES de loadExtensions.
- El prompt generalizado (openPrompt) y el config menu son los patrones a seguir.
