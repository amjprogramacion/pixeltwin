package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const historyFile = "pixeltwin_history.json"
const historyMax  = 10 // máximo de entradas únicas a guardar

// HistoryEntry representa un escaneo pasado.
type HistoryEntry struct {
	Folder    string    `json:"folder"`
	ScannedAt time.Time `json:"scannedAt"`
	Groups    int       `json:"groups"` // nº de grupos de duplicados encontrados
}

// historyPath devuelve la ruta al archivo JSON en %APPDATA%\PixelTwin\
func historyPath() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = "."
	}
	dir := filepath.Join(appData, "PixelTwin")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, historyFile)
}

// loadHistory lee el historial desde disco. Devuelve slice vacío si no existe.
func loadHistory() []HistoryEntry {
	data, err := os.ReadFile(historyPath())
	if err != nil {
		return []HistoryEntry{}
	}
	var entries []HistoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return []HistoryEntry{}
	}
	return entries
}

// saveHistory escribe el historial a disco.
func saveHistory(entries []HistoryEntry) {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(historyPath(), data, 0644)
}

// addToHistory añade una entrada al historial.
// Si ya existe la misma carpeta, actualiza esa entrada en lugar de duplicarla.
// Mantiene las entradas ordenadas de más reciente a más antigua.
func addToHistory(folder string, groups int) []HistoryEntry {
	entries := loadHistory()

	// Eliminar entrada previa de la misma carpeta si existe
	filtered := entries[:0]
	for _, e := range entries {
		if !pathEqual(e.Folder, folder) {
			filtered = append(filtered, e)
		}
	}

	// Insertar al principio (más reciente primero)
	entry := HistoryEntry{
		Folder:    folder,
		ScannedAt: time.Now(),
		Groups:    groups,
	}
	entries = append([]HistoryEntry{entry}, filtered...)

	// Recortar al máximo
	if len(entries) > historyMax {
		entries = entries[:historyMax]
	}

	saveHistory(entries)
	return entries
}

// pathEqual compara rutas ignorando mayúsculas/minúsculas (Windows es case-insensitive)
func pathEqual(a, b string) bool {
	return filepath.Clean(filepath.ToSlash(a)) == filepath.Clean(filepath.ToSlash(b))
}
