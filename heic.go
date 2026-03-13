package main

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// magickBin contiene el ejecutable magick.exe embebido en el binario.
// El archivo debe existir en assets/magick.exe antes de compilar.
//
//go:embed assets/magick.exe
var magickBin embed.FS

var (
	magickPath     string // ruta al magick.exe extraido en disco temporal
	magickInitOnce sync.Once
	magickInitErr  error
)

// initMagick extrae magick.exe del binario a una carpeta temporal la primera
// vez que se necesita. Las llamadas siguientes reutilizan la misma ruta.
func initMagick() error {
	magickInitOnce.Do(func() {
		data, err := magickBin.ReadFile("assets/magick.exe")
		if err != nil {
			magickInitErr = fmt.Errorf("no se pudo leer magick.exe embebido: %w", err)
			return
		}

		// Crear carpeta temporal unica para esta ejecucion
		tmpDir, err := os.MkdirTemp("", "pixeltwin-*")
		if err != nil {
			magickInitErr = fmt.Errorf("no se pudo crear carpeta temporal: %w", err)
			return
		}

		magickPath = filepath.Join(tmpDir, "magick.exe")
		if err := os.WriteFile(magickPath, data, 0700); err != nil {
			magickInitErr = fmt.Errorf("no se pudo escribir magick.exe: %w", err)
			return
		}
	})
	return magickInitErr
}

// cleanupMagick borra el magick.exe temporal. Llamar con defer en main().
func cleanupMagick() {
	if magickPath != "" {
		os.RemoveAll(filepath.Dir(magickPath))
	}
}

// decodeHEIC convierte un archivo HEIC a imagen Go usando el magick embebido.
// No escribe ningun archivo intermedio: el JPEG va de magick directamente a RAM.
func decodeHEIC(path string) (image.Image, error) {
	if err := initMagick(); err != nil {
		return nil, err
	}

	// "jpeg:-" vuelca el resultado a stdout en lugar de a un archivo
	cmd := exec.Command(magickPath, path, "jpeg:-")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("magick fallo (%w): %s", err, stderr.String())
	}

	img, _, err := image.Decode(&stdout)
	if err != nil {
		return nil, fmt.Errorf("decode del jpeg en memoria: %w", err)
	}
	return img, nil
}
