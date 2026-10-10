package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	defaultWindowWidth  = 1120
	defaultWindowHeight = 720
	minimumWindowWidth  = 900
	minimumWindowHeight = 560
)

func main() {
	app := newApplication()

	mygo.App.WhenReady(func() {
		app.loadSystemSettings()
		openMainWindow(app)
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func openMainWindow(app *application) {
	mygo.NewWindow(mygo.WindowOptions{
		Title:                "GoPaste",
		Width:                defaultWindowWidth,
		Height:               defaultWindowHeight,
		MinWidth:             minimumWindowWidth,
		MinHeight:            minimumWindowHeight,
		TitleBarStyle:        mygo.TitleBarHiddenInset,
		TrafficLightPosition: &mygo.Point{X: 18, Y: 11},
		TitleBarHeight:       windowTitleBarHeight,
		BackgroundColor:      "light-dark(#f5f5f7, #181a1f)",
		Vibrancy:             mygo.VibrancyNone,
		StateKey:             "main",
		Content:              ui.View(app.view),
	})
}
