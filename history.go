package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const historyFile = "pixeltwin_history.json"
const historyMax  = 10

type HistoryEntry struct {
	Folders   []string  `json:"folders"`
	ScannedAt time.Time `json:"scannedAt"`
	Groups    int       `json:"groups"`
}

func historyPath() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = "."
	}
	dir := filepath.Join(appData, "PixelTwin")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, historyFile)
}

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

func saveHistory(entries []HistoryEntry) {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(historyPath(), data, 0644)
}

// foldersKey genera una clave normalizada y ordenada para comparar conjuntos de carpetas
func foldersKey(folders []string) string {
	norm := make([]string, len(folders))
	for i, f := range folders {
		norm[i] = strings.ToLower(filepath.Clean(filepath.ToSlash(f)))
	}
	sort.Strings(norm)
	return strings.Join(norm, "|")
}

func addToHistory(folders []string, groups int) []HistoryEntry {
	entries := loadHistory()
	key := foldersKey(folders)

	// Eliminar entrada previa con el mismo conjunto de carpetas
	filtered := entries[:0]
	for _, e := range entries {
		if foldersKey(e.Folders) != key {
			filtered = append(filtered, e)
		}
	}

	entry := HistoryEntry{
		Folders:   folders,
		ScannedAt: time.Now(),
		Groups:    groups,
	}
	entries = append([]HistoryEntry{entry}, filtered...)

	if len(entries) > historyMax {
		entries = entries[:historyMax]
	}

	saveHistory(entries)
	return entries
}
