# Protocolo de Integración de Agentes: tcode

Este archivo define cómo los agentes de IA externos deben interactuar con **tcode**, respetando la `constitution.md` y nuestra arquitectura MVC.

## 1. Principios de Interacción
- **Respeto a la Arquitectura:** Los agentes deben operar a través de las interfaces expuestas por el `Controller`, nunca manipulando el `View` directamente.
- **Acceso a Datos:** Toda lectura o modificación de archivos debe pasar por el `Model` (`PieceTable`), garantizando que la carga sea eficiente y vía `mmap`.
- **Asincronía:** Las solicitudes de los agentes deben ser asíncronas para no bloquear el bucle de renderizado de la terminal.

## 2. API de Agentes (Internal Agent API)
- **Contexto:** El agente puede solicitar el "buffer visible" del `Model`.
- **Edición:** Las ediciones deben enviarse como `Delta` (posición, qué se borra, qué se inserta) para mantener la integridad de la `Piece Table`.
- **Eventos:** El agente puede suscribirse al bus de eventos del `Controller` para reaccionar a cambios en tiempo real.

## 3. Limitaciones
- **Escalabilidad:** El agente no debe cargar el archivo completo en su propia memoria. Debe solicitar chunks específicos al `Model`.
- **Resource Aware:** Si un agente requiere realizar una operación de análisis costosa, debe notificar al usuario para ejecutarse en background, evitando picos de RAM en el editor.
