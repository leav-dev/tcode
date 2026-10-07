# Feature: `tcode uninstall` en Go

## Description
Desinstalar exigía el script (`install.sh --uninstall`). Se suma el comando
al CLI para hacerlo sin salir del editor ni buscar el repo.

## Decisiones de arquitectura (del agente)
- Vive en `internal/update` (`Uninstall(home, exe)` testeable, espejo del
  script: **mismo alcance**). `main.go` solo resuelve home + ejecutable.
- Borra el binario en uso, `~/.tcode/bin` solo si lo contenía (un `~/go/bin`
  con más herramientas no se toca) y los bloques PATH marcados de los rc.
- No toca config, temas, extensiones ni providers: reinstalar encuentra todo.
- Sin confirmación (igual que el script); un typo como `uninstal` cae al
  editor como archivo, nunca desinstala por error.
- Windows devuelve error claro (el .exe en uso está lockeado).

## Tasks
- [x] `internal/update/uninstall.go`: `Uninstall` + `stripMarkedBlock`
- [x] `internal/update/uninstall_test.go`: flujo completo, dir ajeno, sin rcs,
  segunda vez falla, strip unitario
- [x] `main.go`: caso `uninstall` + línea en la guía
- [x] Docs: README (fila Desinstalar)

## Evidence
- `go test` OK; simulacro vivo con `HOME` falso: borra binario+dir, limpia
  el bloque del rc conservando lo propio, exit 0. Instalación real intacta.
