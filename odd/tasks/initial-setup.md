# Feature: Initial Project Setup

## Description
Establecer las bases del editor de código en Go (tcode), definiendo la arquitectura (MVC + Screen Architecture) y los requerimientos técnicos iniciales.

## Tasks
- [x] Definir la arquitectura en `docs/constitution.md` <!-- id: 0 -->
- [x] Inicializar `go.mod` <!-- id: 1 -->
- [x] Implementar el loop principal de la TUI con `tcell` y soporte de mouse <!-- id: 2 -->
- [x] Diseñar la estructura de la `Piece Table` básica <!-- id: 3 -->
- [x] Crear `agents.md` (raíz) y `docs/memory.md` <!-- id: 4 -->
- [x] Configurar repo: rama `main` + remoto `origin` <!-- id: 5 -->

## Evidence
- Commit raíz `fc07fa0` en `main`, pusheado a `git@github.com:leav-dev/tcode.git`.
- `agents.md` vive en la raíz por convención de estándares; la constitución y la
  memoria del proyecto quedan bajo `docs/`.
- `.gitignore` cubre estado local de Pi (`.atl/`), artefactos de build de Go,
  cobertura y basura de editor/OS.
- El trabajo posterior a este bootstrap se rastrea en
  `odd/tasks/efficient-viewport.md`.
