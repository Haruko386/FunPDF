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
	"fmt"
	"log"
	"net"
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

const (
	backendHost      = "127.0.0.1"
	backendStartPort = 38600
	backendMaxPort   = 38800
)

func newAPIProxy(backendAddr string) http.Handler {
	target, err := url.Parse("http://" + backendAddr)
	if err != nil {
		panic(fmt.Errorf("parse backend address %q: %w", backendAddr, err))
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

func listenBackend() (net.Listener, string, error) {
	var lastErr error

	for p := backendStartPort; p <= backendMaxPort; p++ {
		addr := fmt.Sprintf("%s:%d", backendHost, p)

		listener, err := net.Listen("tcp", addr)
		if err == nil {
			return listener, addr, nil
		}
		lastErr = err
		log.Printf("desktop backend port unavailable: %s: %v", addr, err)
	}

	return nil, "", fmt.Errorf("no available desktop backend port in %s:%d-%d: last error: %w",
		backendHost,
		backendStartPort,
		backendMaxPort,
		lastErr,
	)
}

func startBackend(ctx context.Context) (*http.Server, string, error) {
	dbPath, err := desktopDatabasePath()
	if err != nil {
		return nil, "", err
	}

	if err := funpdf.InitSqliteDatabase(dbPath); err != nil {
		return nil, "", err
	}

	defaultCacheDir, err := desktopCacheDir()
	if err != nil {
		return nil, "", err
	}

	cacheDir, err := funpdf.EnsureRuntimeInfo(dbPath, defaultCacheDir)
	if err != nil {
		return nil, "", err
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

	listener, backendAddr, err := listenBackend()
	if err != nil {
		return nil, "", err
	}

	httpSvr := &http.Server{
		Addr:    backendAddr,
		Handler: r,
	}

	go func() {
		if err := httpSvr.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("desktop backend error: %v", err)
		}
	}()

	log.Printf("desktop backend listening on http://%s", backendAddr)

	return httpSvr, backendAddr, nil
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

	backendCtx, cancelBackend := context.WithCancel(context.Background())
	app.shutdown = cancelBackend

	backend, backendAddr, err := startBackend(backendCtx)
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
			Handler: newAPIProxy(backendAddr),
		},
		OnStartup:  app.Start,
		OnShutdown: app.Shutdown,
	})

	if err != nil {
		log.Fatalf("start wails: %v", err)
	}
}
