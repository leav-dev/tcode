# Constitución del Proyecto: tcode

Este documento establece las reglas fundamentales, la arquitectura y los principios de diseño para el desarrollo de **tcode**, un editor de código eficiente basado en terminal escrito en Go.

## 1. Principios Core

- **Eficiencia Extrema:** El uso de RAM debe ser mínimo. Si algo no está en pantalla, no debe estar cargado de forma activa en estructuras pesadas.
- **Lazy Loading:** Los archivos se manejan mediante `mmap` y `Piece Table`.
- **Amigabilidad (Mouse & Agentes):** Soporte de mouse completo (clic, scroll, selección) y una arquitectura que permita a agentes de IA interactuar con el estado del editor de forma asincrónica.
- **Portabilidad:** Debe funcionar en cualquier terminal moderna (xterm, kitty, alacritty, etc.).

## 2. Arquitectura de Alto Nivel: MVC + Screen Architecture

Para mantener la modularidad y evitar el "código espagueti", usamos una combinación de Model-View-Controller (MVC) con un enfoque de Pantallas (Screens).

### A. Model (Capa de Datos)
- Responsable de la gestión del texto (Piece Table).
- Maneja el undo/redo y la persistencia (mmap).
- **Aislamiento:** No sabe nada de la terminal ni del renderizado.

### B. View (Capa de Presentación)
- Responsable de dibujar en la terminal usando `tcell`.
- Implementa el **Virtual Viewport**: solo procesa y dibuja el rango de líneas visibles.
- **Screen Architecture:** La View se divide en componentes modulares (FileBrowser, EditorPane, StatusBar, CommandPanel). Cada componente es una "pantalla" que solo se carga y renderiza si es necesaria.

### C. Controller (Capa de Lógica)
- El cerebro que une el Model y la View.
- Captura los eventos de `tcell` (teclado y mouse) y decide qué acción tomar.
- Orquesta la comunicación con los agentes de IA mediante canales asincrónicos.

## 3. Screen Architecture (Detalle)

Dentro de cada capa (especialmente en View y Controller), el código se organiza por "Escenas" o "Pantallas":

- **Modularidad Total:** Cada funcionalidad (ej. el explorador de archivos) vive en su propio paquete.
- **Carga Bajo Demanda:** No se inicializan estructuras de datos de una pantalla si el usuario no la invoca.
- **Comunicación por Eventos:** Los componentes se comunican a través de un bus de eventos interno para evitar acoplamiento fuerte.

## 4. Estándares de Código

- **Lenguaje:** Go 1.21+ (aprovechando genéricos para la Piece Table).
- **Gestión de Memoria:** Prohibido cargar archivos enteros en slices de bytes; usar `mmap`.
- **Concurrencia:** Los agentes y procesos pesados corren en Goroutines separadas.
- **Testing:** Cada componente del Model debe tener tests unitarios (TDD).

### 4.1 Layout de los tests: van junto al código

**Los archivos `_test.go` viven en el mismo directorio que el código que prueban.** No hay una carpeta `tests/` en la raíz. Esto es una decisión deliberada, no un descuido, y conviene que un revisor lo sepa de antemano:

```
internal/model/piece_table.go
internal/model/piece_table_test.go   <- mismo directorio, a propósito
internal/view/editor_view.go
internal/view/editor_view_test.go    <- mismo directorio, a propósito
```

**Por qué:** Go compila los `_test.go` únicamente junto al paquete de su mismo directorio. Un archivo de test en otra carpeta pertenece a **otro paquete** y solo puede usar la API **exportada**. Moverlos obligaría a una de dos cosas, ambas malas: exportar estado interno solo para poder testearlo, o resignar la verificación de invariantes que no son observables desde afuera.

**Qué se perdería concretamente** si se movieran (210 referencias a estado interno en total):

- `TestGetRangeIsZeroCopyViewIntoMmap` comprueba que el slice devuelto **apunte dentro del `mmap`**. Es la garantía cero-copia, la premisa central del proyecto, y desde afuera no es demostrable.
- Los invariantes de la Piece Table: que la suma de las piezas sea `docLen` y que `lineOffsets` esté ordenado, sin duplicados y siempre precedido por `\n`. Ese test encontró un bug real en la edición.
- El acceso a `v.cursor`, `v.viewport`, `app.confirmQuit` para verificar movimiento, columna deseada y confirmación de salida.

**El marcador que evita la confusión es el sufijo `_test.go`.** El toolchain de Go lo excluye del build normal, `go test ./...` los descubre solo, y cualquier lector de Go los reconoce como tests al instante. Que un archivo tenga 500 líneas de test no lo hace ambiguo: lo hace un test largo.

**Lo que sí corresponde hacer para reducir ruido:** usar `testdata/` para fixtures —el toolchain lo ignora por convención— y extraer andamiaje repetido (pantallas simuladas, helpers de teclado, carga de archivos temporales) a un paquete interno compartido, en lugar de duplicarlo en cada archivo.

## 5. Integración de Agentes

El editor debe exponer una **Internal Agent API** (vía canales o RPC local) que permita:
1. Leer el contexto actual (ventana visible).
2. Solicitar ediciones atómicas en la Piece Table.
3. Suscribirse a eventos de cambio de archivo.
