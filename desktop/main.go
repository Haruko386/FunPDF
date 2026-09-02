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
	funpdf "FunPDF/internal"
	"FunPDF/internal/common"
	"context"
	"embed"
	"errors"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

const backendAddr = "127.0.0.1:38600"

func newAPIProxy() http.Handler {
	target, err := url.Parse("http://" + backendAddr)
	if err != nil {
		log.Fatalf("parse backend: %v", err)
		return nil
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		proxy.ServeHTTP(w, r)
	})
}

func desktopCacheDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	appDir := filepath.Join(configDir, "FunPDF")
	cacheDir := filepath.Join(appDir, "cache")

	err = os.MkdirAll(cacheDir, 0755)
	if err != nil {
		return "", err
	}
	return cacheDir, nil
}

func startBackend(ctx context.Context) (*http.Server, error) {
	dbPath, err := desktopDatabasePath()
	if err != nil {
		return nil, err
	}
	err = funpdf.InitSqliteDatabase(dbPath)
	if err != nil {
		return nil, err
	}

	cacheDir, err := desktopCacheDir()
	if err != nil {
		return nil, err
	}
	r := funpdf.NewHTTPHandlerWithRuntime(funpdf.RuntimeInfo{
		Mode:         "desktop",
		Database:     "sqlite",
		DatabasePath: dbPath,
		CacheDir:     cacheDir,
	})

	common.Banner()
	log.Printf("FunPDF %s", common.GetVersion())

	funpdf.StartPDFTextCleaner(ctx)

	httpSvr := &http.Server{Addr: backendAddr, Handler: r}

	go func() {
		if err = httpSvr.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("start backend: %v", err)
		}
	}()

	return httpSvr, nil
}

func desktopDatabasePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	addDir := filepath.Join(configDir, "FunPDF")

	err = os.MkdirAll(addDir, 0755)
	if err != nil {
		return "", err
	}

	return filepath.Join(addDir, "FunPDF.db"), nil
}

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:  "FunPDF " + common.GetVersion(),
		Width:  1280,
		Height: 860,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: newAPIProxy(),
		},
		OnStartup:  app.Start,
		OnShutdown: app.Shutdown,
		//Bind:       []any{app},
	})
	if err != nil {
		log.Fatalf("start wails: %v", err)
		return
	}
}
