package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/corona10/goimagehash"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// reNameDate busca el patron YYYYMMDD_HHMMSS en cualquier parte del nombre de archivo.
// Ejemplos que detecta:
//   20240105_112900.jpg
//   IMG_20240105_112900_edited.jpg
//   foto_20240105_112900_v2.heic
var reNameDate = regexp.MustCompile(`\b(\d{8})_(\d{6})\b`)

// parseNameDate extrae la fecha/hora del nombre de archivo.
// Devuelve time.Time zero si el nombre no contiene el patron.
func parseNameDate(path string) time.Time {
	base := filepath.Base(path)
	m := reNameDate.FindStringSubmatch(base)
	if m == nil {
		return time.Time{}
	}
	// m[1] = "20240105", m[2] = "112900"
	t, err := time.ParseInLocation("20060102_150405", m[1]+"_"+m[2], time.Local)
	if err != nil {
		return time.Time{}
	}
	return t
}

var imageExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".bmp":  true,
	".tiff": true,
	".tif":  true,
	".webp": true,
	".heic": true,
	".heif": true,
	".hif":  true,
}

type ImageInfo struct {
	Path     string
	Size     int64
	ModTime  time.Time
	SHA256   string
	PHash    uint64
	Width    int
	Height   int
	Error    string
	NameDate time.Time // fecha extraida del nombre de archivo (zero si no tiene)
}

// hasNameDate devuelve true si el archivo tiene fecha valida en el nombre
func (img *ImageInfo) hasNameDate() bool {
	return !img.NameDate.IsZero()
}

type DuplicateGroup struct {
	Images     []*ImageInfo
	GroupType  string
	Similarity float64
}

type ScanResult struct {
	TotalFound   int
	TotalScanned int
	TotalErrors  int
	Groups       []DuplicateGroup
	Duration     time.Duration
}

type Scanner struct {
	SimilarityThreshold int
	Workers             int
	OnProgress          func(done, total int)
	Ctx                 context.Context // cancelacion externa; nil = sin limite
}

func NewScanner() *Scanner {
	return &Scanner{
		SimilarityThreshold: 10,
		Workers:             8,
	}
}

func (s *Scanner) Scan(rootDirs []string) (*ScanResult, error) {
	start := time.Now()

	// Cargar caché de hashes al inicio (no-op si ya está cargada)
	globalCache.Load()

	// Recoger imágenes de todas las carpetas, deduplicando rutas
	seen := make(map[string]bool)
	var paths []string
	for _, rootDir := range rootDirs {
		fmt.Printf("Buscando imagenes en: %s\n", rootDir)
		dirPaths, err := collectImagePaths(rootDir)
		if err != nil {
			fmt.Printf("Error escaneando %s: %v\n", rootDir, err)
			continue
		}
		for _, p := range dirPaths {
			norm := normPath(p)
			if !seen[norm] {
				seen[norm] = true
				paths = append(paths, p)
			}
		}
	}
	fmt.Printf("Encontradas %d imagenes en total\n", len(paths))

	images := s.processImages(paths)

	var errors int
	var valid []*ImageInfo
	for _, img := range images {
		if img == nil {
			// Worker cancelado antes de procesar esta imagen
			continue
		}
		if img.Error != "" {
			errors++
		} else {
			valid = append(valid, img)
		}
	}
	fmt.Printf("Procesadas: %d OK, %d con error\n", len(valid), errors)

	// Limpiar entradas obsoletas y guardar caché actualizada
	scannedSet := make(map[string]bool, len(paths))
	for _, p := range paths {
		scannedSet[normPath(p)] = true
	}
	globalCache.Prune(scannedSet)
	globalCache.Save()

	// Si fue cancelado durante el procesamiento, abortar
	if s.Ctx != nil {
		select {
		case <-s.Ctx.Done():
			return nil, fmt.Errorf("escaneo cancelado")
		default:
		}
	}

	groups := s.findGroups(valid)

	return &ScanResult{
		TotalFound:   len(paths),
		TotalScanned: len(valid),
		TotalErrors:  errors,
		Groups:       groups,
		Duration:     time.Since(start),
	}, nil
}

func collectImagePaths(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if imageExtensions[ext] {
			paths = append(paths, path)
		}
		return nil
	})
	return paths, err
}

func (s *Scanner) processImages(paths []string) []*ImageInfo {
	total := len(paths)
	results := make([]*ImageInfo, total)
	jobs := make(chan int, total)

	var wg sync.WaitGroup
	var mu sync.Mutex
	done := 0

	for w := 0; w < s.Workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				// Comprobar cancelación antes de procesar cada imagen
				if s.Ctx != nil {
					select {
					case <-s.Ctx.Done():
						return
					default:
					}
				}

				img := processImage(paths[idx])
				results[idx] = img

				mu.Lock()
				done++
				if s.OnProgress != nil {
					s.OnProgress(done, total)
				}
				mu.Unlock()
			}
		}()
	}

	for i := range paths {
		jobs <- i
	}
	close(jobs)

	wg.Wait()
	return results
}

func isHEIC(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".heic" || ext == ".heif" || ext == ".hif"
}

func processImage(path string) *ImageInfo {
	info := &ImageInfo{Path: path}

	stat, err := os.Stat(path)
	if err != nil {
		info.Error = err.Error()
		return info
	}
	info.Size    = stat.Size()
	info.ModTime = stat.ModTime()
	info.NameDate = parseNameDate(path)

	// ── Comprobar caché ───────────────────────────────
	if cached, ok := globalCache.Get(path, info.Size, info.ModTime); ok {
		info.SHA256 = cached.SHA256
		info.PHash  = cached.PHash
		info.Width  = cached.Width
		info.Height = cached.Height
		return info // cache hit: nos saltamos sha256 + decode de imagen
	}

	// ── Cache miss: calcular hashes ───────────────────
	// SHA256 desde el archivo original
	f, err := os.Open(path)
	if err != nil {
		info.Error = err.Error()
		return info
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		info.Error = err.Error()
		return info
	}
	info.SHA256 = hex.EncodeToString(hasher.Sum(nil))

	// Decodificar para pHash y dimensiones
	var img image.Image
	if isHEIC(path) {
		// decodeHEIC vive en heic.go y usa el magick.exe embebido
		img, err = decodeHEIC(path)
		if err != nil {
			info.Error = fmt.Sprintf("heic: %s", err)
			return info
		}
	} else {
		f.Seek(0, io.SeekStart)
		img, _, err = image.Decode(f)
		if err != nil {
			info.Error = fmt.Sprintf("no se pudo decodificar: %s", err)
			return info
		}
	}

	bounds := img.Bounds()
	info.Width = bounds.Dx()
	info.Height = bounds.Dy()

	hash, err := goimagehash.PerceptionHash(img)
	if err != nil {
		info.Error = err.Error()
		return info
	}
	info.PHash = hash.GetHash()

	// Guardar en caché para futuros escaneos
	globalCache.Set(CacheEntry{
		Path:    path,
		Size:    info.Size,
		ModTime: info.ModTime,
		SHA256:  info.SHA256,
		PHash:   info.PHash,
		Width:   info.Width,
		Height:  info.Height,
	})

	return info
}

func (s *Scanner) findGroups(images []*ImageInfo) []DuplicateGroup {
	var groups []DuplicateGroup

	exactMap := make(map[string][]*ImageInfo)
	for _, img := range images {
		exactMap[img.SHA256] = append(exactMap[img.SHA256], img)
	}

	inExactGroup := make(map[string]bool)
	for _, group := range exactMap {
		if len(group) < 2 {
			continue
		}
		for _, img := range group {
			inExactGroup[img.Path] = true
		}
		sortByOriginal(group)
		groups = append(groups, DuplicateGroup{
			Images:     group,
			GroupType:  "exact",
			Similarity: 100.0,
		})
	}

	var candidates []*ImageInfo
	for _, img := range images {
		if !inExactGroup[img.Path] {
			candidates = append(candidates, img)
		}
	}

	visited := make(map[string]bool)
	for i := 0; i < len(candidates); i++ {
		a := candidates[i]
		if visited[a.Path] {
			continue
		}

		var group []*ImageInfo
		group = append(group, a)

		for j := i + 1; j < len(candidates); j++ {
			b := candidates[j]
			if visited[b.Path] {
				continue
			}
			if hammingDistance(a.PHash, b.PHash) <= s.SimilarityThreshold {
				group = append(group, b)
				visited[b.Path] = true
			}
		}

		if len(group) >= 2 {
			visited[a.Path] = true
			sortByOriginal(group)
			groups = append(groups, DuplicateGroup{
				Images:     group,
				GroupType:  "similar",
				Similarity: similarityPercent(s.SimilarityThreshold),
			})
		}
	}

	return groups
}

// sortByOriginal ordena un grupo poniendo el "original mas probable" primero.
// Jerarquia de criterios:
//  1. Tiene fecha/hora en el nombre  →  prioritario
//  2. Fecha mas antigua entre los que tienen nombre con fecha
//  3. Tamaño mayor como desempate final
func sortByOriginal(group []*ImageInfo) {
	// sort.Slice es estable para criterios multiples aplicados en orden
	// Go no garantiza estabilidad, asi que codificamos todos los criterios
	// en una sola funcion de comparacion.
	//
	// Retorna true si a debe ir ANTES que b (a es "mas original")
	less := func(a, b *ImageInfo) bool {
		aHas := a.hasNameDate()
		bHas := b.hasNameDate()

		// Criterio 1: quien tiene fecha en el nombre gana
		if aHas != bHas {
			return aHas // a tiene fecha, b no → a va primero
		}

		// Criterio 2: ambos tienen fecha → la mas antigua es el original
		if aHas && bHas {
			if !a.NameDate.Equal(b.NameDate) {
				return a.NameDate.Before(b.NameDate)
			}
		}

		// Criterio 3: desempate por tamaño (mayor = mas probable original)
		return a.Size > b.Size
	}

	// Bubble sort simple: los grupos suelen tener 2-5 elementos
	n := len(group)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-1-i; j++ {
			if less(group[j+1], group[j]) {
				group[j], group[j+1] = group[j+1], group[j]
			}
		}
	}
}

func hammingDistance(a, b uint64) int {
	xor := a ^ b
	count := 0
	for xor != 0 {
		count += int(xor & 1)
		xor >>= 1
	}
	return count
}

func similarityPercent(distanceThreshold int) float64 {
	return float64(64-distanceThreshold) / 64.0 * 100.0
}
