# PixelTwin v1.0.0

Herramienta de escritorio para Windows que detecta y elimina imágenes duplicadas y similares, incluyendo soporte para carpetas de red (NAS) y archivos HEIC/HEIF de iPhone.

---

## Características

- Detección de **duplicados exactos** (SHA256) y **similares** (pHash con umbral ajustable)
- Soporte para **carpetas de red / NAS** — las miniaturas se sirven desde Go, sin restricciones del WebView
- Soporte para **HEIC/HEIF** (.heic, .heif, .hif) via ImageMagick embebido
- **Corrección automática de orientación EXIF** en las miniaturas
- **Caché de hashes persistente** — los reescaneos de carpetas ya procesadas son casi instantáneos
- **Historial de escaneos** recientes con relanzamiento automático
- **Lightbox** con navegación por teclado (←→) y tira de miniaturas del grupo
- **Borrado en lote** a la papelera con un único diálogo de confirmación nativo de Windows
- **Cancelación limpia** del escaneo en curso
- Un único `.exe` portable sin dependencias

## Formatos soportados

`.jpg` `.jpeg` `.png` `.gif` `.bmp` `.tiff` `.tif` `.webp` `.heic` `.heif` `.hif`

---

## Archivos

| Archivo | Qué hace |
|---|---|
| `app.go` | Puente entre Go y la UI — expone métodos a JavaScript via Wails (Scan, DeleteFiles, GetHistory, etc.) |
| `main.go` | Punto de entrada: configura y arranca la ventana Wails |
| `scanner.go` | Motor principal: escaneo de carpetas, hashing SHA256 + pHash, agrupación de duplicados |
| `heic.go` | Soporte HEIC/HEIF via `magick.exe` embebido — extrae a temp, convierte a JPEG en memoria |
| `thumbs.go` | Servidor de miniaturas HTTP interno — sirve imágenes redimensionadas con corrección de orientación EXIF |
| `hashcache.go` | Caché de hashes persistente en disco — evita recalcular SHA256/pHash en reescaneos |
| `history.go` | Historial de escaneos recientes — guarda ruta, fecha y nº de grupos por escaneo |
| `trash_windows.go` | Papelera nativa de Windows via `SHFileOperationW` y apertura de archivos con app predeterminada |
| `go.mod` / `go.sum` | Dependencias del módulo Go |
| `wails.json` | Configuración del proyecto Wails (nombre, versión, frontend, etc.) |
| `frontend/.env` | Variables de entorno para Vite — contiene `VITE_APP_VERSION` |
| `frontend/index.html` | Estructura HTML de la app |
| `frontend/src/main.js` | Lógica del frontend — llama a Go via bindings de Wails, gestiona UI y eventos |
| `frontend/src/style.css` | Estilos — tema oscuro completo |
| `build/appicon.png` | Icono de la app — Wails lo convierte a `.ico` al compilar |

---

## Requisitos

- [Go 1.22+](https://go.dev/dl/)
- [Wails v2](https://wails.io/docs/gettingstarted/installation) — `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- [Node.js 18+](https://nodejs.org/)
- Windows 10/11

---

## Instalación y desarrollo

### 1. Clonar el repositorio
```bash
git clone https://github.com/amjprogramacion/pixeltwin.git
cd pixeltwin
```

### 2. Añadir magick.exe (soporte HEIC, opcional)
Descarga [ImageMagick portable Q16 x64](https://imagemagick.org/script/download.php#windows), descomprime y copia `magick.exe` a:
```
assets/magick.exe
```
Sin este archivo la app funciona con normalidad pero ignora los archivos `.heic/.heif`.

### 3. Instalar dependencias y arrancar en modo dev
```bash
go mod tidy
wails dev
```

### 4. Compilar el ejecutable final
```bash
wails build
```
El resultado es `build/bin/pixeltwin.exe` — un único archivo portable que incluye todo.

---

## Datos del usuario

La app guarda estos archivos en `%APPDATA%\PixelTwin\`:

| Archivo | Contenido |
|---|---|
| `pixeltwin_cache.json` | Caché de hashes SHA256 y pHash |
| `pixeltwin_history.json` | Historial de escaneos recientes |

Se crean automáticamente en el primer uso y persisten entre sesiones.

---

## Licencia

MIT
