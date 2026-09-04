package main

import (
	funpdf "FunPDF/internal"
	"FunPDF/internal/common"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

const (
	backendHost      = "127.0.0.1"
	backendStartPort = 38600
	backendMaxPort   = 38800
)

// newAPIProxy creates a reverse proxy that forwards API requests to the backend server.
func newAPIProxy(backendAddr, desktopToken string) http.Handler {
	target, err := url.Parse("http://" + backendAddr)
	if err != nil {
		panic(fmt.Errorf("parse backend address %q: %w", backendAddr, err))
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director

	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Header.Set("X-FunPDF-Desktop-Token", desktopToken)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		proxy.ServeHTTP(w, r)
	})
}

// listenBackend opens a local listener for the embedded backend server.
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

// startBackend initializes and starts the embedded backend HTTP server.
func startBackend(ctx context.Context) (*http.Server, string, string, error) {
	dbPath, err := desktopDatabasePath()
	if err != nil {
		return nil, "", "", err
	}

	if err := funpdf.InitSqliteDatabase(dbPath); err != nil {
		return nil, "", "", err
	}

	defaultCacheDir, err := desktopCacheDir()
	if err != nil {
		return nil, "", "", err
	}

	cacheDir, err := funpdf.EnsureRuntimeInfo(dbPath, defaultCacheDir)
	if err != nil {
		return nil, "", "", err
	}

	desktopToken := common.GenerateUUID()
	r := funpdf.NewHTTPHandlerWithRuntime(funpdf.RuntimeInfo{
		Mode:         "desktop",
		Database:     "sqlite",
		DatabasePath: dbPath,
		CacheDir:     cacheDir,
		DesktopToken: desktopToken,
	})

	common.Banner()
	log.Printf("FunPDF %s", common.GetVersion())

	funpdf.StartPDFTextCleaner(ctx)

	listener, backendAddr, err := listenBackend()
	if err != nil {
		return nil, "", "", err
	}

	httpSvr := &http.Server{
		Addr:              backendAddr,
		Handler:           r,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       10 * time.Second,
	}

	go func() {
		if err := httpSvr.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("desktop backend error: %v", err)
		}
	}()

	log.Printf("desktop backend listening on http://%s", backendAddr)

	return httpSvr, backendAddr, desktopToken, nil
}
