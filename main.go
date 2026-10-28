// Команда os_model — Wails-приложение «Модель ОС» (вариант 11).
package main

import (
	"embed"
	"flag"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	seed := flag.Uint64("seed", 0, "начальное значение генератора случайных чисел")
	flag.Parse()

	app := NewApp(*seed)

	err := wails.Run(&options.App{
		Title:     "Модель ОС — вариант 11",
		Width:     1500,
		Height:    1000,
		MinWidth:  1180,
		MinHeight: 700,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 243, G: 244, B: 246, A: 1},
		OnStartup:        app.startup,
		Bind:             []interface{}{app},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
