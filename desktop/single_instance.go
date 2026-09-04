package main

import (
	"log"

	"github.com/wailsapp/wails/v2/pkg/options"
)

const singleInstanceID = "com.haruko386.funpdf.single-instance"

// newSingleInstanceLock prevents multiple FunPDF desktop windows.
func newSingleInstanceLock(app *App) *options.SingleInstanceLock {
	return &options.SingleInstanceLock{
		UniqueId: singleInstanceID,
		OnSecondInstanceLaunch: func(secondInstanceData options.SecondInstanceData) {
			log.Printf("second FunPDF instance ignored: args=%v", secondInstanceData.Args)
			app.Focus()
		},
	}
}
