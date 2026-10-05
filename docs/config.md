# Configuración del editor

tcode maneja sus **configuraciones básicas** desde una **ventana flotante**
(overlay sobre el editor, como el menú de pestañas): se abre y cierra con
**`Ctrl+,`** y los cambios se aplican **en vivo** y se persisten.

## La ventana flotante

```
┌─ Configuración ────────┐
│ Tab size           4   │
│ Word wrap        on    │
│ Panel width       24   │
└────────────────────────┘
```

- `Up` / `Down` mueven la fila activa.
- `Left` / `Right` cambian el valor de la fila (los booleanos alternan; los
  números se mueven con paso y límites).
- `Enter` alterna los booleanos; en los números no hace nada.
- `Escape` (o cualquier otra tecla) cierra la ventana.

## El archivo

`~/.tcode/config.json` se lee al arrancar (un JSON roto o ausente conserva los
defaults: el archivo del usuario jamás rompe el editor) y se reescribe cada
vez que cambiás un ajuste.

```json
{
  "IndentUnit": 4,
  "WordWrap": true,
  "ExplorerWidth": 24
}
```

| Clave | Significado | Default | Rango en la ventana |
| --- | --- | --- | --- |
| `IndentUnit` | Espacios de la unidad de indentación (Tab y auto-indent) | `4` | 1–8 |
| `WordWrap` | Salto de palabra visual (también alternable con `Ctrl+Shift+W`) | `true` | on/off |
| `ExplorerWidth` | Ancho máximo del panel lateral | `24` | 16–48 (paso 2) |

## Notas de terminal

- `Ctrl+,` funciona en Windows Terminal. En algunos terminales Unix la
  combinación con signos puede no llegar a la aplicación (como el form feed de
  `Ctrl+L`).
- Los tests del controlador aíslan el archivo redirigiendo `configFilePath` a
  un directorio temporal (patrón de `themeFilePath`).

En el código: `view/settings.go` (estado de la config), `view/config_menu.go`
(el overlay), `internal/controller/app.go` (`loadConfig`/`saveConfig`/
`toggleConfig`).