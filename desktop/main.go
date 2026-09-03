// Last modified: 2026-08-16
//
// Copyright 2026 Haruko386, SJZU. All rights reserved.

//  Licensed under the GNU GENERAL PUBLIC LICENSE, Version 3.0 (the "License");
//  you may not use this file except in compliance with the License.
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.
//

package main

import (
	"FunPDF/internal/common"
	"context"
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	backendCtx, cancelBackend := context.WithCancel(context.Background())
	app.shutdown = cancelBackend

	backend, backendAddr, desktopToken, err := startBackend(backendCtx)
	if err != nil {
		cancelBackend()
		log.Fatalf("start desktop backend: %v", err)
	}

	app.backend = backend

	err = wails.Run(&options.App{
		Title:  "FunPDF " + common.GetVersion(),
		Width:  1280,
		Height: 860,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: newAPIProxy(backendAddr, desktopToken),
		},
		OnStartup:  app.Start,
		OnShutdown: app.Shutdown,
	})

	if err != nil {
		log.Fatalf("start wails: %v", err)
	}
}
