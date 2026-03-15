package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const cacheFile = "pixeltwin_cache.json"

// CacheEntry almacena los hashes y metadatos de una imagen ya procesada.
// La clave de validez es ruta + tamaño + fecha de modificación.
// Si cualquiera de los tres cambia, la entrada se considera obsoleta.
type CacheEntry struct {
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
	SHA256  string    `json:"sha256"`
	PHash   uint64    `json:"phash"`
	Width   int       `json:"width"`
	Height  int       `json:"height"`
}

// HashCache es una caché en memoria respaldada en disco.
type HashCache struct {
	mu      sync.RWMutex
	entries map[string]CacheEntry // clave: path normalizado
	dirty   bool                  // true si hay cambios pendientes de guardar
}

var globalCache = &HashCache{
	entries: make(map[string]CacheEntry),
}

// cachePath devuelve la ruta al archivo JSON en %APPDATA%\PixelTwin\
func cachePath() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = "."
	}
	dir := filepath.Join(appData, "PixelTwin")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, cacheFile)
}

// LoadCache carga la caché desde disco al arrancar.
func (c *HashCache) Load() {
	data, err := os.ReadFile(cachePath())
	if err != nil {
		return // primera vez, no hay caché aún
	}

	var entries []CacheEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		fmt.Printf("[cache] error cargando caché: %v\n", err)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]CacheEntry, len(entries))
	for _, e := range entries {
		c.entries[normPath(e.Path)] = e
	}
	fmt.Printf("[cache] cargadas %d entradas desde disco\n", len(c.entries))
}

// Save escribe la caché a disco si hubo cambios.
func (c *HashCache) Save() {
	c.mu.RLock()
	if !c.dirty {
		c.mu.RUnlock()
		return
	}
	entries := make([]CacheEntry, 0, len(c.entries))
	for _, e := range c.entries {
		entries = append(entries, e)
	}
	c.mu.RUnlock()

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(cachePath(), data, 0644); err != nil {
		fmt.Printf("[cache] error guardando caché: %v\n", err)
		return
	}

	c.mu.Lock()
	c.dirty = false
	c.mu.Unlock()
	fmt.Printf("[cache] guardadas %d entradas en disco\n", len(entries))
}

// Get devuelve una entrada válida si existe y los metadatos del archivo coinciden.
func (c *HashCache) Get(path string, size int64, modTime time.Time) (CacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	e, ok := c.entries[normPath(path)]
	if !ok {
		return CacheEntry{}, false
	}
	// Validar que el archivo no ha cambiado desde que se cacheó
	if e.Size != size || !e.ModTime.Equal(modTime) {
		return CacheEntry{}, false
	}
	return e, true
}

// Set almacena o actualiza una entrada en la caché.
func (c *HashCache) Set(e CacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[normPath(e.Path)] = e
	c.dirty = true
}

// Prune elimina entradas de archivos que ya no existen en disco.
// Llamar después de un escaneo para mantener la caché limpia.
func (c *HashCache) Prune(scannedPaths map[string]bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	before := len(c.entries)
	for k := range c.entries {
		if !scannedPaths[k] {
			delete(c.entries, k)
			c.dirty = true
		}
	}
	pruned := before - len(c.entries)
	if pruned > 0 {
		fmt.Printf("[cache] eliminadas %d entradas obsoletas\n", pruned)
	}
}

// Clear vacía la caché en memoria y borra el archivo del disco.
func (c *HashCache) Clear() {
	c.mu.Lock()
	c.entries = make(map[string]CacheEntry)
	c.dirty = false
	c.mu.Unlock()
	os.Remove(cachePath())
	fmt.Println("[cache] caché eliminada")
}

func normPath(p string) string {
	return filepath.Clean(filepath.ToSlash(p))
}
