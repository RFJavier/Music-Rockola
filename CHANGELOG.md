# Changelog

Todos los cambios notables de este proyecto se documentan en este archivo.

El formato se basa en [Keep a Changelog](https://keepachangelog.com/es/1.1.0/),
y las versiones siguen [Semantic Versioning](https://semver.org/lang/es/).

## [Sin publicar]

### Pendiente

- FASE 6: integración automática Player -> Queue (avance al terminar la
  pista) y monitoreo activo de fin de reproducción.
- Lectura de metadata ID3 para completar título, artista, álbum y duración.
- Migración de la búsqueda a SQLite FTS5.

## [0.1.0] - 2026-09-14

MVP inicial del Core de la rockola con arquitectura desacoplada
(Go + SQLite + Windows) y primera versión lista para publicar en GitHub.

### Agregado

- **Catálogo**
  - Modelo de canción junto con interfaces y servicios del dominio.
  - Búsqueda por título, artista, álbum o género (`LIKE`).
  - Validación de rutas de archivos multimedia en el filesystem.
  - Escaneo recursivo de carpetas con detección de duplicados y conteo de
    formatos no compatibles.
- **Créditos simulados**
  - Saldo persistido, alta administrativa (`+1`, `+5`, `+10`).
  - Consumo de una unidad por canción encolada.
  - Auditoría de cada operación en `credit_transactions`.
- **Cola FIFO**
  - Alta de canciones con consumo del crédito en una transacción única.
  - Estados `pending`, `playing`, `completed` y `cancelled`.
  - Garantía de una sola canción `playing`; historial persistido.
- **Reproductor**
  - Abstracción `MediaPlayer` independiente de Windows.
  - Implementación MCI sobre `winmm.dll` con todas las llamadas serializadas
    en un único hilo Windows.
  - Controles: play, pause, resume, stop, next, volumen, posición y duración.
- **API HTTP/JSON local**
  - Endpoints de catálogo, scan, créditos, cola y player.
  - Formato de errores estable y apagado ordenado.
- **Interfaz local**
  - UI responsive embebida en el binario, abierta automáticamente en el
    navegador predeterminado de Windows.
  - Sin lógica de negocio: consume únicamente la API.
- **Documentación**
  - `README.md`, `LICENSE` (Apache 2.0), `THIRD_PARTY_LICENSES.md`,
    `CHANGELOG.md`, `.http` para pruebas de API y `.gitignore`.

### Corregido

- Errores MCI 263 en `pause`/`stop` al ejecutar operaciones desde hilos
  distintos: ahora todo el transporte se serializa en un hilo Windows
  dedicado.
- `pause` después del fin natural devolvía error interno: ahora devuelve un
  estado inválido controlado.
- `next` sin canciones pendientes devolvía error: ahora finaliza en estado
  `stopped`.

### Técnico

- Go 1.22+ sin CGO; SQLite embebido con `modernc.org/sqlite`.
- Separación estricta entre dominio, persistencia, API e interfaz.
- Pruebas de servicios, persistencia y regresión del player.