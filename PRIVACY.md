# Política de privacidad de tcode

`tcode` es un editor local: tus archivos se editan en tu máquina y el programa
no tiene cuentas, telemetría ni rastreo de ningún tipo.

## Lo que nunca sale de tu máquina

- El contenido de tus archivos y buffers.
- Tu configuración, temas y lista de extensiones.
- Los scripts Lua de tus extensiones corren embebidos sin acceso a red
  (solo librerías base/tabla/string/math): una extensión no puede exfiltrar
  nada por sí misma.

Todo vive localmente bajo `~/.tcode/` (`config.json`, `theme.json`,
`providers.json`, `extensions/`, `extensions-seen.json`).

## Las únicas salidas a red (todas visibles y a pedido o avisadas)

| Cuándo | A dónde | Para qué |
|---|---|---|
| Al arrancar, en segundo plano | `api.github.com/repos/leav-dev/tcode` | Chequear si hay actualización (muestra un aviso en la barra, nunca descarga solo) |
| `tcode update` o instaladores | `github.com/leav-dev/tcode/releases` | Descargar el binario verificado |
| Al arrancar, en segundo plano | Tus proveedores de extensiones aprobados | Leer catálogos para avisarte novedades |
| `tcode --install-extension` o la ventana de extensiones | El proveedor elegido (previa aprobación explícita si no es de confianza) | Clonar solo esa extensión |

Agregar un proveedor no es confiar en él: la primera instalación desde una
fuente no aprobada pide confirmación explícita.

## Contacto

¿Dudas o reportes sobre privacidad? Abrí un issue en
<https://github.com/leav-dev/tcode/issues>.
