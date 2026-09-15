# FireFly Rockola

> Rockola / Jukebox local para Windows con arquitectura desacoplada:
> **Go** como Core de negocio, **SQLite** como base local y la **interfaz
> Windows** consumiendo únicamente una API HTTP/JSON.

[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8.svg)](https://go.dev)
[![Estado](https://img.shields.io/badge/fases-1%20a%205%20implementadas-green.svg)](CHANGELOG.md)

## Características

- **Catálogo** en SQLite (los archivos de audio permanecen en el filesystem;
  la DB solo guarda rutas absolutas).
- **Búsqueda** por título, artista, álbum y género, con escaneo recursivo de
  carpetas y detección de duplicados.
- **Créditos simulados** con alta administrativa y auditoría de cada consumo.
- **Cola FIFO** persistente con estados `pending`, `playing`, `completed` y
  `cancelled`, y garantía de una sola canción reproducida.
- **Reproductor nativo** de Windows vía MCI (`winmm.dll`), sin dependencias
  externas ni procesos adicionales.
- **API HTTP/JSON local** y **UI responsive** embebida en el binario que se
  abre automáticamente en el navegador.

```text
                    ROCKOLA
                       │
             ┌─────────┴─────────┐
             │                   │
        WINDOWS UI            GO CORE
             │                   │
             │             ┌─────┼─────┐
             │             │     │     │
             │          Catalog Queue Credits
             │             │     │     │
             │             └─────┼─────┘
             │                   │
             │                SQLite
             │
             └──── API / IPC ────┘
                       │
                       ▼
                 MEDIA PLAYER
```

## Estado del proyecto

| Fase | Contenido | Estado |
|------|-----------|--------|
| 1 | SQLite, catálogo, scan de carpeta, búsqueda | Implementada |
| 2 | Queue FIFO, Credits | Implementada |
| 3 | MediaPlayer nativo de Windows | Implementada |
| 4 | API HTTP/JSON local | Implementada |
| 5 | Interfaz local para Windows | Implementada |
| 6 | Integración automática Player -> Queue | Pendiente |

## Estructura del proyecto

```text
rockola/
├── core/
│   ├── catalog/      # Dominio del catálogo: Song, Repository, Service, Scanner
│   ├── config/       # Configuración por variables de entorno
│   ├── credits/      # Dominio del saldo simulado
│   ├── playback/     # Interfaz MediaPlayer y coordinador de reproducción
│   ├── queue/        # Dominio y servicio FIFO
│   └── storage/      # Implementación SQLite (única capa que conoce SQL)
├── database/         # Esquema SQL embebido en el binario
├── player/           # Adaptador MCI para Windows
├── api/              # Handlers HTTP/JSON; no contiene SQL
├── ui/               # Interfaz web embebida y launcher de Windows
├── cmd/
│   └── rockola-core/ # Punto de entrada del Core
├── api/rockola.http  # Colección de pruebas para REST Client
├── LICENSE
├── THIRD_PARTY_LICENSES.md
├── CHANGELOG.md
└── README.md
```

## Requisitos

- Windows
- Go 1.22+ (probado con 1.26)
- Sin CGO: se usa el driver SQLite puro Go (`modernc.org/sqlite`); **no** se
  necesita GCC/MinGW.

## Compilar

```powershell
go build -o rockola-core.exe ./cmd/rockola-core
```

## Configuración

| Variable | Default | Descripción |
|----------|---------|-------------|
| `ROCKOLA_DB_PATH` | `rockola.db` | Ruta del archivo SQLite (se crea automáticamente) |
| `ROCKOLA_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `ROCKOLA_HTTP_ADDR` | `127.0.0.1:8080` | Dirección local de la API y la UI |

```powershell
$env:ROCKOLA_DB_PATH = "D:\Rockola\rockola.db"
$env:ROCKOLA_HTTP_ADDR = "127.0.0.1:8080"
```

## Uso

### Interfaz (recomendado)

```powershell
.\rockola-core.exe
```

Inicia la API y abre la interfaz automáticamente en
`http://127.0.0.1:8080`. Para iniciar sin abrir ventana:

```powershell
.\rockola-core.exe serve
```

> Si recompilas mientras el Core está abierto, detén primero el proceso
> anterior con `Ctrl+C`; de lo contrario el puerto 8080 seguirá atendido por
> el binario viejo y el nuevo proceso no podrá iniciarse.

### CLI del Core

```powershell
# Escanear una carpeta de música (recursivo, evita duplicados)
.\rockola-core.exe scan "D:\Rockola\Music"

# Catálogo y búsqueda
.\rockola-core.exe songs
.\rockola-core.exe search "Soda Stereo"
.\rockola-core.exe song 3
.\rockola-core.exe artists
.\rockola-core.exe albums
.\rockola-core.exe genres

# Créditos simulados
.\rockola-core.exe credits
.\rockola-core.exe credits-add 5
.\rockola-core.exe credits-consume

# Cola FIFO (cada queue-add consume 1 crédito)
.\rockola-core.exe queue-add 3
.\rockola-core.exe queue
.\rockola-core.exe queue-next
.\rockola-core.exe queue-remove 4
.\rockola-core.exe queue-clear

# Transiciones persistidas de reproducción
.\rockola-core.exe queue-playing 1
.\rockola-core.exe queue-current
.\rockola-core.exe queue-complete 1

# Reproducir una canción directamente (Ctrl+C para detener)
.\rockola-core.exe player-play 3
```

## API HTTP/JSON

Colección de pruebas: [`api/rockola.http`](api/rockola.http).

```text
GET    /api/health
GET    /api/songs
GET    /api/songs/search?q=Soda
GET    /api/songs/{id}
GET    /api/artists
GET    /api/albums
GET    /api/genres
POST   /api/catalog/scan       {"path":"D:\\Rockola\\Music"}

GET    /api/queue
POST   /api/queue              {"song_id":1}
DELETE /api/queue/{id}
DELETE /api/queue

GET    /api/credits
POST   /api/credits/add        {"amount":5}
POST   /api/credits/consume

GET    /api/player
POST   /api/player/play
POST   /api/player/pause
POST   /api/player/resume
POST   /api/player/stop
POST   /api/player/next
POST   /api/player/volume      {"volume":0.75}
```

Ejemplo de flujo completo:

```powershell
Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8080/api/catalog/scan `
  -ContentType application/json -Body '{"path":"D:\\Rockola\\Music"}'

Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8080/api/credits/add `
  -ContentType application/json -Body '{"amount":5}'

Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8080/api/queue `
  -ContentType application/json -Body '{"song_id":1}'

Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8080/api/player/play
Invoke-RestMethod -Uri http://127.0.0.1:8080/api/player
```

Los errores tienen formato estable:

```json
{
  "error": {
    "code": "insufficient_credits",
    "message": "agregando canción a la cola: créditos insuficientes"
  }
}
```

## Escaneo de música

```text
POST /api/catalog/scan  {"path":"D:\\Rockola\\Music"}
```

- Recorre la carpeta recursivamente.
- Detecta `mp3`, `wav`, `flac`, `ogg`, `m4a`, `wma`.
- Evita duplicados por ruta; guarda rutas absolutas.
- Reporta agregadas, existentes e incompatibles.

> MVP: todavía no se lee metadata ID3. El título se deriva del nombre del
> archivo; si se llama `Artista - Título.mp3`, se separan ambos campos.

## Notas de arquitectura

- **Dominio vs. persistencia**: los servicios de negocio definen interfaces
  como `catalog.Repository`; `core/storage` las implementa con SQL
  parametrizado. Los handlers HTTP nunca ejecutan SQL.
- **Búsqueda**: hoy usa `LIKE`. Migrar a SQLite FTS5 solo requiere cambiar
  `SongRepository.Search` en `core/storage/song_repository.go`.
- **Crédito + cola atómicos**: encolar una canción descuenta el crédito y
  crea la entrada de cola dentro de la misma transacción SQLite.
- **Player desacoplado**: `core/playback.MediaPlayer` no conoce Windows;
  `player.MCIPlayer` lo implementa sobre `winmm.dll` con todas las llamadas
  serializadas en un único hilo Windows.
- **FASE 6 pendiente**: el avance automático al terminar una canción aún no
  está integrado; `next` lo realiza manualmente desde la UI y la API.

## Pruebas

```powershell
go test ./...
go vet ./...
```

Las pruebas verifican saldo insuficiente, auditoría, consumo atómico, archivo
faltante, FIFO, estados persistidos, validación HTTP, controles del player y
una regresión nativa que ejecuta comandos MCI desde distintos goroutines.

## Licencia y dependencias

- El proyecto se distribuye bajo **Apache License 2.0**: consulta
  [`LICENSE`](LICENSE).
- Las dependencias de terceros se listan con sus licencias en
  [`THIRD_PARTY_LICENSES.md`](THIRD_PARTY_LICENSES.md).
- Historial de cambios en [`CHANGELOG.md`](CHANGELOG.md).