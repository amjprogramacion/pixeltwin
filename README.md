# DupeFinder — Escáner de imágenes duplicadas

Motor en Go puro para encontrar imágenes duplicadas y similares.
Paso previo a integrar con Wails para la interfaz gráfica.

## Requisitos

- Go 1.22 o superior → https://go.dev/dl/

## Instalación

```bash
# 1. Entrar a la carpeta del proyecto
cd dupefinder

# 2. Descargar dependencias
go mod tidy

# 3. Compilar
go build -o dupefinder.exe   # Windows
go build -o dupefinder       # Linux/Mac
```

## Uso

```bash
# Escanear una carpeta
./dupefinder.exe "C:\Users\Juan\Fotos"

# Escanear la carpeta actual
./dupefinder.exe .
```

## Ejemplo de salida

```
╔══════════════════════════════════════╗
║       DupeFinder - Escáner Go        ║
╚══════════════════════════════════════╝

Buscando imágenes en: C:\Users\Juan\Fotos
Encontradas 1284 imágenes
  [████████████████████] 1284/1284 (100%)
Procesadas: 1281 OK, 3 con error

══════════════ RESUMEN ══════════════
  Imágenes encontradas : 1284
  Procesadas con éxito : 1281
  Con errores          : 3
  Grupos de duplicados : 47
  Tiempo total         : 4.2s
  Espacio recuperable  : 2.3 GB

══════════════ GRUPOS ════════════════

  Grupo 1 [EXACTO 100%] — 3 archivos
  ✓ IMG_4821.jpg
      Ruta   : C:\Fotos\Verano\IMG_4821.jpg
      Tamaño : 6.1 MB   4032x3024 px  ← conservar
    IMG_4821 copia.jpg
      Ruta   : C:\Fotos\Backup\IMG_4821 copia.jpg
      Tamaño : 6.1 MB   4032x3024 px
    Playa agosto.jpg
      Ruta   : C:\Desktop\Playa agosto.jpg
      Tamaño : 6.1 MB   4032x3024 px
```

## Ajustar sensibilidad de similitud

En `main.go`, cambia el valor de `SimilarityThreshold`:

```go
scanner.SimilarityThreshold = 10  // recomendado (por defecto)
//                            0   → solo duplicados visualmente idénticos
//                            10  → misma foto con distinta compresión/recorte leve
//                            20  → fotos bastante parecidas (puede dar falsos positivos)
```

## Archivos

| Archivo      | Qué hace                                              |
|--------------|-------------------------------------------------------|
| `scanner.go` | Motor principal: escaneo, hashing SHA256 + pHash      |
| `main.go`    | Punto de entrada para pruebas en terminal             |
| `go.mod`     | Dependencias del módulo                               |

## Siguiente paso

Integrar `scanner.go` en un proyecto Wails:
- Copiar `scanner.go` al proyecto Wails
- Exponer `Scanner.Scan()` en `app.go` con `wails.Bind()`
- Llamar desde el frontend con `window.go.main.App.Scan(folder)`

## Dependencias

- `github.com/corona10/goimagehash` — perceptual hashing (pHash)
- `github.com/disintegration/imaging` — decodificación de imágenes
- Formatos soportados: JPG, PNG, GIF, BMP, TIFF, WebP
