# Feature: tcode.read_file para scripts (validar imports en ambas direcciones)

## Description
La sesión del repo tcode-extention pide `tcode.read_file(relpath)` en el host
Lua para que la extensión unused-imports valide también "importado que no
existe en el módulo origen" (imports relativos a archivos). Condición local:
que no enlentezca el editor (sin proceso SO aparte —rompería el sandbox—;
lectura acotada, sincrónica y rápida + cacheo Lua).

## Tasks
- [x] `internal/ext/script.go`: método `ReadFile` en `ScriptAPI` + binding `tcode.read_file` (nil silencioso) <!-- id: 0 -->
- [x] `internal/controller/host_files.go`: `App.ReadFile` (contención, allowlist, cota 2 MiB, symlinks) <!-- id: 1 -->
- [x] Tests: host (tabla/nil) + controller (bordes) + fakes (`fakeAPI`, `forwardAPI`) <!-- id: 2 -->
- [x] Docs: fila en `docs/extension-system.md` + respuesta a la sesión tcode-extention <!-- id: 3 -->

## Evidence
- Patrón a reusar: `DirFiles`/`dir_files` (nil silencioso, cotas en el proveedor).
- `ScriptAPI` la implementan: `App`, `fakeAPI` (script_test), `forwardAPI` (manager_test).
