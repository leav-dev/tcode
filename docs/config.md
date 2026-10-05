# Configuración del editor

tcode maneja sus **configuraciones básicas** desde una **ventana flotante**
(overlay sobre el editor, como el menú de pestañas): se abre y cierra con
**`Ctrl+P`** y los cambios se aplican **en vivo** y se persisten.

## La ventana flotante

```
┌─ Configuración ────────┐
│ Tab size           4   │
│ Word wrap        on    │
│ Panel width       24   │
│ Theme          Custom  │
│ Extensiones      abrir │
└────────────────────────┘
```

- `Up` / `Down` mueven la fila activa.
- `Left` / `Right` cambian el valor de la fila (los booleanos alternan; los
  números se mueven con paso y límites).
- `Enter` alterna los booleanos; en los números no hace nada.
- `Escape` (o cualquier otra tecla) cierra la ventana.

La ventana tiene **topes de geometría**: muestra hasta **8 filas** y, si hay
más, scrollea con `Up`/`Down` (y `PageUp`/`PageDown`) en vez de crecer. Su
ancho arranca en **34** y puede ensancharse con el contenido (una fila más
ancha) hasta un **máximo de 40**; nunca pasa del tamaño del editor.

## El archivo

`~/.tcode/config.json` se lee al arrancar (un JSON roto o ausente conserva los
defaults: el archivo del usuario jamás rompe el editor) y se reescribe cada
vez que cambiás un ajuste.

```json
{
  "IndentUnit": 4,
  "WordWrap": true,
  "ExplorerWidth": 24,
  "Theme": "dracula"
}
```

| Clave | Significado | Default | Rango en la ventana |
| --- | --- | --- | --- |
| `IndentUnit` | Espacios de la unidad de indentación (Tab y auto-indent) | `4` | 1–8 |
| `WordWrap` | Salto de palabra visual (también alternable con `Ctrl+Shift+W`) | `true` | on/off |
| `ExplorerWidth` | Ancho máximo del panel lateral | `24` | 16–48 (paso 2) |
| `Theme` | Paleta activa: `light`, `dark`, `light-hc`, `dark-hc`, `tokyo-night`, `dracula`; `""` / ausente = Custom (`theme.json`) | `""` | las 6 + Custom |

El `Theme` es un id estable de las paletas integradas (ver `docs/editor-theme.md`);
un valor desconocido se ignora al cargar (queda String vacío → Custom/default).

## Notas de terminal

- `Ctrl+P` (el byte 0x10) pasa limpio en Windows Terminal y en casi todo
  terminal estándar. Arrancó como `Ctrl+,`, pero el terminal del usuario
  interceptaba la coma; `Ctrl+Shift+P` queda fuera a propósito.
- Los tests del controlador aíslan el archivo redirigiendo `configFilePath` a
  un directorio temporal (patrón de `themeFilePath`).

En el código: `view/settings.go` (estado de la config), `view/config_menu.go`
(el overlay y sus topes de geometría), `internal/controller/app.go`
(`loadConfig`/`saveConfig`/`toggleConfig`/`configRegion`).