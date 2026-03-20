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
	"syscall"
)

// magickSem limita el número de procesos magick.exe simultáneos.
// Con demasiados en paralelo Windows se queda sin recursos.
var magickSem = make(chan struct{}, 2)

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

// heicDimensions obtiene las dimensiones reales de un HEIC usando
// "magick identify" — lee solo los metadatos, sin decodificar píxeles.
// Mucho más rápido y ligero que hacer un decode completo.
func heicDimensions(path string) (int, int, error) {
	if err := initMagick(); err != nil {
		return 0, 0, err
	}

	magickSem <- struct{}{}
	defer func() { <-magickSem }()

	// identify -format "%w %h" devuelve "ancho alto" en texto
	cmd := exec.Command(magickPath, "identify", "-format", "%w %h", path)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return 0, 0, err
	}

	var w, h int
	if _, err := fmt.Sscanf(stdout.String(), "%d %d", &w, &h); err != nil {
		return 0, 0, err
	}
	return w, h, nil
}

// decodeHEIC convierte un archivo HEIC a imagen Go a resolución completa.
// Usado para generar miniaturas donde se necesita calidad.
func decodeHEIC(path string) (image.Image, error) {
	return decodeHEICAtSize(path, 0)
}

// decodeHEICSmall convierte un HEIC a una imagen pequeña (maxW px).
// Usado durante el escaneo para calcular pHash — mucho más rápido y ligero
// que decodificar la imagen completa de 12MP.
func decodeHEICSmall(path string, maxW int) (image.Image, error) {
	return decodeHEICAtSize(path, maxW)
}

// decodeHEICAtSize es la implementación común. Si maxW > 0, magick redimensiona
// antes de enviar los datos — ahorrando RAM y tiempo de decode.
func decodeHEICAtSize(path string, maxW int) (image.Image, error) {
	if err := initMagick(); err != nil {
		return nil, err
	}

	// Adquirir slot del semáforo — máximo 2 magick.exe simultáneos
	magickSem <- struct{}{}
	defer func() { <-magickSem }()

	var cmd *exec.Cmd
	if maxW > 0 {
		// Redimensionar dentro de magick antes de exportar:
		// "-thumbnail WxH>" solo escala si es más grande, muy eficiente
		resize := fmt.Sprintf("%dx%d>", maxW, maxW)
		cmd = exec.Command(magickPath, path, "-thumbnail", resize, "jpeg:-")
	} else {
		cmd = exec.Command(magickPath, path, "jpeg:-")
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
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
