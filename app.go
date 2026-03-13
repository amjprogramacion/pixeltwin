package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App es la estructura principal expuesta a la UI via Wails.
// Todos sus metodos publicos son llamables desde JavaScript.
type App struct {
	ctx        context.Context
	scanner    *Scanner
	lastScan   *ScanResult
	cancelScan context.CancelFunc // cancela el escaneo en curso, nil si no hay ninguno
}

func NewApp() *App {
	return &App{
		scanner: NewScanner(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// Precargar caché de hashes para que el primer escaneo sea rápido
	go globalCache.Load()
}

// ── Tipos que viajan entre Go y JavaScript (JSON) ──────────────────────────

type ImageDTO struct {
	Path     string `json:"path"`
	Filename string `json:"filename"`
	SizeFmt  string `json:"sizeFmt"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	NameDate string `json:"nameDate"` // "" si no tiene fecha en el nombre
	IsOrigin bool   `json:"isOrigin"` // true = primer elemento del grupo (original sugerido)
}

type GroupDTO struct {
	ID         int        `json:"id"`
	GroupType  string     `json:"groupType"`  // "exact" | "similar"
	Similarity float64    `json:"similarity"` // 100.0 para exactos
	Images     []ImageDTO `json:"images"`
	TotalSize  string     `json:"totalSize"`  // tamaño total del grupo formateado
	WasteSize  string     `json:"wasteSize"`  // espacio recuperable (todos menos el primero)
}

type ScanResultDTO struct {
	TotalFound   int      `json:"totalFound"`
	TotalScanned int      `json:"totalScanned"`
	TotalErrors  int      `json:"totalErrors"`
	GroupCount   int      `json:"groupCount"`
	Reclaimable  string   `json:"reclaimable"`
	Duration     string   `json:"duration"`
	Groups       []GroupDTO `json:"groups"`
}

type ProgressDTO struct {
	Done    int    `json:"done"`
	Total   int    `json:"total"`
	Percent int    `json:"percent"`
	Current string `json:"current"` // nombre del archivo actual
}

type HistoryEntryDTO struct {
	Folder    string `json:"folder"`
	ScannedAt string `json:"scannedAt"` // formateado para mostrar
	Groups    int    `json:"groups"`
}

// ── Metodos expuestos a JavaScript ────────────────────────────────────────

// SelectFolder abre el dialogo nativo de Windows para elegir carpeta.
func (a *App) SelectFolder() string {
	folder, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Seleccionar carpeta de imágenes",
	})
	if err != nil {
		return ""
	}
	return folder
}

// Scan lanza el escaneo en la carpeta dada.
// Emite eventos "scan:progress" durante el proceso y devuelve el resultado final.
// Si se llama a CancelScan() mientras está en curso, devuelve error "cancelado".
func (a *App) Scan(rootDir string, similarityThreshold int) (*ScanResultDTO, error) {
	if rootDir == "" {
		return nil, fmt.Errorf("no se ha seleccionado ninguna carpeta")
	}

	// Cancelar cualquier escaneo previo que pudiera estar corriendo
	if a.cancelScan != nil {
		a.cancelScan()
	}

	scanCtx, cancel := context.WithCancel(a.ctx)
	a.cancelScan = cancel
	defer func() { a.cancelScan = nil }()

	a.scanner.SimilarityThreshold = similarityThreshold
	a.scanner.Workers = 8
	a.scanner.Ctx = scanCtx

	// Emitir progreso al frontend via eventos Wails
	a.scanner.OnProgress = func(done, total int) {
		// Si ya fue cancelado no emitir más eventos
		select {
		case <-scanCtx.Done():
			return
		default:
		}
		pct := 0
		if total > 0 {
			pct = done * 100 / total
		}
		runtime.EventsEmit(a.ctx, "scan:progress", ProgressDTO{
			Done:    done,
			Total:   total,
			Percent: pct,
		})
	}

	result, err := a.scanner.Scan(rootDir)
	if err != nil {
		return nil, err
	}

	a.lastScan = result
	dto := toDTO(result)

	// Guardar en historial (sin duplicar la misma carpeta)
	addToHistory(rootDir, dto.GroupCount)

	return dto, nil
}

// GetHistory devuelve el historial de escaneos recientes.
func (a *App) GetHistory() []HistoryEntryDTO {
	entries := loadHistory()
	result := make([]HistoryEntryDTO, len(entries))
	for i, e := range entries {
		result[i] = HistoryEntryDTO{
			Folder:    e.Folder,
			ScannedAt: e.ScannedAt.Format("02/01/2006 15:04"),
			Groups:    e.Groups,
		}
	}
	return result
}

// CancelScan detiene el escaneo en curso si lo hay.
func (a *App) CancelScan() {
	if a.cancelScan != nil {
		a.cancelScan()
	}
}

// DeleteFiles mueve todos los archivos indicados a la papelera en una
// sola operacion — el usuario ve un unico dialogo de confirmacion.
// Devuelve slice vacio si todo fue bien.
func (a *App) DeleteFiles(paths []string) []string {
	if err := moveToTrash(paths); err != nil {
		// Si falla el lote entero devolvemos todas las rutas como fallidas
		return paths
	}
	return []string{}
}

// OpenFile abre un archivo con el programa predeterminado de Windows.
func (a *App) OpenFile(path string) error {
	return openWithDefault(path)
}

// ── Helpers de conversion ─────────────────────────────────────────────────

func toDTO(r *ScanResult) *ScanResultDTO {
	var reclaimable int64
	groups := make([]GroupDTO, 0, len(r.Groups))

	for i, g := range r.Groups {
		var total, waste int64
		images := make([]ImageDTO, len(g.Images))

		for j, img := range g.Images {
			nd := ""
			if img.hasNameDate() {
				nd = img.NameDate.Format("02/01/2006 15:04:05")
			}
			images[j] = ImageDTO{
				Path:     img.Path,
				Filename: filepath.Base(img.Path),
				SizeFmt:  formatSize(img.Size),
				Width:    img.Width,
				Height:   img.Height,
				NameDate: nd,
				IsOrigin: j == 0,
			}
			total += img.Size
			if j > 0 {
				waste += img.Size
			}
		}
		reclaimable += waste

		groups = append(groups, GroupDTO{
			ID:         i + 1,
			GroupType:  g.GroupType,
			Similarity: g.Similarity,
			Images:     images,
			TotalSize:  formatSize(total),
			WasteSize:  formatSize(waste),
		})
	}

	// Ordenar: exactos primero, luego similares
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].GroupType != groups[j].GroupType {
			return groups[i].GroupType == "exact"
		}
		return groups[i].ID < groups[j].ID
	})

	return &ScanResultDTO{
		TotalFound:   r.TotalFound,
		TotalScanned: r.TotalScanned,
		TotalErrors:  r.TotalErrors,
		GroupCount:   len(r.Groups),
		Reclaimable:  formatSize(reclaimable),
		Duration:     r.Duration.Round(time.Millisecond).String(),
		Groups:       groups,
	}
}

func formatSize(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// moveToTrash y openWithDefault se implementan en trash_windows.go
// para usar las APIs nativas de Windows.
var _ = os.Remove // asegura que os esta importado aunque no se use directamente aqui
