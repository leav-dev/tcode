# Follow-up: observaciones R3-001, R3-002, R4-001 del review

## Description
Tres advisory del último review aprobado, atacados por defecto concreto.

## Fixes
- **R4-001** (`uninstall.go:43`, resilience): el primer fallo abortaba con
  estado parcial. Ahora best-effort tras borrar el binario: dir no vacío se
  deja en silencio (no es nuestro), errores de rc se acumulan con
  `errors.Join` sin cortar a los demás.
- **R3-001** (`uninstall.go:55`, reliability): el progreso parcial se perdía
  (nil junto al error). Ahora `Uninstall` devuelve lo eliminado JUNTO al
  error y `main.go` lo imprime antes del mensaje (`desinstalación parcial`).
- **R3-002** (`seen.go:70`, reliability): la propiedad "los vistos de un
  proveedor caído no se pierden" se cumple en código; se pinea con
  `TestPruneConservaVistosDeProveedorCaido` (mark → prune → save → load).

## Tasks
- [x] `uninstall.go`: acumulación + tolerancia + doc best-effort
- [x] `main.go`: imprime progreso parcial ante error
- [x] Tests: dir no vacío, acumulación unix (chmod+tag), regresión de vistos
- [x] Suite completa verde

## Evidence
- `go test ./...` OK (incluye `uninstall_unix_test.go` con tag `unix`).
