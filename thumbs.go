package main

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/disintegration/imaging"
	"github.com/rwcarlsen/goexif/exif"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// ── Caché de miniaturas en disco ──────────────────────────────────────────

var thumbCacheDir string
var thumbCacheOnce sync.Once

// initThumbCache inicializa la carpeta de caché de miniaturas en %APPDATA%\PixelTwin\thumbs\
func initThumbCache() string {
	thumbCacheOnce.Do(func() {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = "."
		}
		thumbCacheDir = filepath.Join(appData, "PixelTwin", "thumbs")
		os.MkdirAll(thumbCacheDir, 0755)
	})
	return thumbCacheDir
}

// thumbCachePath devuelve la ruta en disco para una miniatura dada.
// La clave es un MD5 de "ruta|tamaño|modtime" para invalidar automáticamente
// si el archivo original cambia.
func thumbCachePath(path string, size int) (string, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	key := fmt.Sprintf("%s|%d|%d", path, size, stat.ModTime().UnixNano())
	hash := fmt.Sprintf("%x", md5.Sum([]byte(key)))
	return filepath.Join(initThumbCache(), hash+".jpg"), nil
}

// ── Handler HTTP ──────────────────────────────────────────────────────────

type ThumbHandler struct {
	next http.Handler
}

func NewThumbHandler(next http.Handler) *ThumbHandler {
	return &ThumbHandler{next: next}
}

func (h *ThumbHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/thumb") {
		if h.next != nil {
			h.next.ServeHTTP(w, r)
		}
		return
	}

	rawPath := r.URL.Query().Get("path")
	if rawPath == "" {
		http.Error(w, "path requerido", http.StatusBadRequest)
		return
	}

	decoded, err := url.QueryUnescape(rawPath)
	if err != nil {
		decoded = rawPath
	}

	size := 320
	if r.URL.Query().Get("size") == "1200" {
		size = 1200
	}

	// ── Intentar servir desde caché en disco ──
	cachePath, err := thumbCachePath(decoded, size)
	if err == nil {
		if data, err := os.ReadFile(cachePath); err == nil {
			// Cache hit — servir directamente sin procesar nada
			w.Header().Set("Content-Type", "image/jpeg")
			w.Header().Set("Cache-Control", "private, max-age=86400")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
			w.WriteHeader(http.StatusOK)
			w.Write(data)
			return
		}
	}

	// ── Cache miss — generar miniatura ────────
	fmt.Printf("[thumb] generando: %s (size=%d)\n", decoded, size)
	thumb, err := generateThumb(decoded, size)
	if err != nil {
		fmt.Printf("[thumb] ERROR: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Guardar en caché para próximas peticiones (async para no bloquear la respuesta)
	if cachePath != "" {
		go os.WriteFile(cachePath, thumb, 0644)
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(thumb)))
	w.WriteHeader(http.StatusOK)
	w.Write(thumb)
}

// ── Generación de miniaturas ──────────────────────────────────────────────

func exifOrientation(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 1
	}
	defer f.Close()

	x, err := exif.Decode(f)
	if err != nil {
		return 1
	}
	tag, err := x.Get(exif.Orientation)
	if err != nil {
		return 1
	}
	val, err := tag.Int(0)
	if err != nil {
		return 1
	}
	return val
}

func applyOrientation(img image.Image, orientation int) image.Image {
	switch orientation {
	case 2:
		return imaging.FlipH(img)
	case 3:
		return imaging.Rotate180(img)
	case 4:
		return imaging.FlipV(img)
	case 5:
		return imaging.Transpose(img)
	case 6:
		return imaging.Rotate270(img)
	case 7:
		return imaging.Transverse(img)
	case 8:
		return imaging.Rotate90(img)
	default:
		return img
	}
}

func generateThumb(path string, maxW int) ([]byte, error) {
	var src image.Image
	var err error

	if isHEIC(path) {
		src, err = decodeHEIC(path)
		if err != nil {
			return nil, fmt.Errorf("decode heic: %w", err)
		}
	} else {
		f, ferr := os.Open(path)
		if ferr != nil {
			return nil, ferr
		}
		defer f.Close()

		src, _, err = image.Decode(f)
		if err != nil {
			return nil, fmt.Errorf("decode: %w", err)
		}

		orientation := exifOrientation(path)
		src = applyOrientation(src, orientation)
	}

	bounds := src.Bounds()
	if bounds.Dx() > maxW {
		src = imaging.Resize(src, maxW, 0, imaging.Lanczos)
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: 82}); err != nil {
		return nil, fmt.Errorf("jpeg encode: %w", err)
	}
	return buf.Bytes(), nil
}
