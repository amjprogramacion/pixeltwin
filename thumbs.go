package main

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/rwcarlsen/goexif/exif"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

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

	// Tamaño: 320px para miniaturas, 1200px para el lightbox
	size := 320
	if r.URL.Query().Get("size") == "1200" {
		size = 1200
	}

	fmt.Printf("[thumb] solicitada: %s (size=%d)\n", decoded, size)

	thumb, err := generateThumb(decoded, size)
	if err != nil {
		fmt.Printf("[thumb] ERROR: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Printf("[thumb] OK %d bytes\n", len(thumb))
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(thumb)))
	w.WriteHeader(http.StatusOK)
	w.Write(thumb)
}

// exifOrientation lee el tag Orientation del EXIF de un archivo.
// Devuelve 1 (sin rotación) si no hay EXIF o si falla la lectura.
// Valores posibles:
//   1 = normal
//   3 = rotada 180°
//   6 = rotada 90° CW  (lo más común en fotos de móvil en vertical)
//   8 = rotada 90° CCW
//   2,4,5,7 = variantes con espejo (raro en cámaras normales)
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

// applyOrientation rota/espeja la imagen según el valor EXIF Orientation.
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
		return img // 1 = normal, sin cambios
	}
}

// generateThumb decodifica, corrige orientación EXIF y redimensiona la imagen.
func generateThumb(path string, maxW int) ([]byte, error) {
	var src image.Image
	var err error

	if isHEIC(path) {
		// HEIC: ImageMagick ya aplica la orientación al convertir
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

		// Leer y aplicar orientación EXIF
		// (solo para JPEG/TIFF que pueden tener EXIF — para el resto devuelve 1)
		orientation := exifOrientation(path)
		src = applyOrientation(src, orientation)
	}

	// Redimensionar a maxW px de ancho manteniendo proporción
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
