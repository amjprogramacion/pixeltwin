package main

import (
	"embed"
	"net/http"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	defer cleanupMagick()

	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "PixelTwin",
		Width:     1200,
		Height:    800,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
			// Middleware intercepta TODAS las peticiones antes que Wails.
			// Las que van a /thumb las resuelve ThumbHandler.
			// El resto las pasa al siguiente handler (assets estáticos).
			Middleware: func(next http.Handler) http.Handler {
				return NewThumbHandler(next)
			},
		},
		BackgroundColour: &options.RGBA{R: 18, G: 18, B: 18, A: 1},
		OnStartup:        app.startup,
		OnBeforeClose:    app.beforeClose,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			DisableWindowIcon: false,
		},

	})
	if err != nil {
		println("Error:", err.Error())
	}
}
