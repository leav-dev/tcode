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

## 5. Integración de Agentes

El editor debe exponer una **Internal Agent API** (vía canales o RPC local) que permita:
1. Leer el contexto actual (ventana visible).
2. Solicitar ediciones atómicas en la Piece Table.
3. Suscribirse a eventos de cambio de archivo.
