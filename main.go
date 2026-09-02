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
	"FunPDF/internal"
	"FunPDF/internal/common"
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	err := internal.InitDatabaseFromEnv()
	if err != nil {
		return
	}

	r := internal.NewHTTPHandler()

	common.Banner()
	log.Printf("FunPDF %s", common.GetVersion())

	cleanerCtx, cleanerCtxCancel := context.WithCancel(context.Background())
	defer cleanerCtxCancel()
	internal.StartPDFTextCleaner(cleanerCtx)

	addr := os.Getenv("FUNPDF_ADDR")
	if addr == "" {
		addr = ":9384"
	}

	srv := &http.Server{Addr: addr, Handler: r}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("start backend: %v", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	cleanerCtxCancel()
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
}
