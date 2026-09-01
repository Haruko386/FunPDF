package main

import (
	"context"
	"log"
	"net/http"
	"time"
)

type App struct {
	ctx      context.Context
	shutdown context.CancelFunc
	backend  *http.Server
}

func NewApp() *App {
	return &App{}
}

func (a *App) Start(ctx context.Context) {
	a.ctx = ctx
	backendCtx, cancelBackend := context.WithCancel(ctx)

	a.shutdown = cancelBackend
	backend, err := startBackend(backendCtx)
	if err != nil {
		log.Fatal(err)
	}
	a.backend = backend
}

func (a *App) Shutdown(ctx context.Context) {
	if a.backend != nil {
		toCtx, cancelBackend := context.WithTimeout(context.Background(), time.Second*5)
		defer cancelBackend()
		err := a.backend.Shutdown(toCtx)
		if err != nil {
			log.Printf("desktop backend shutdown error: %v", err)
		}
	}
	if a.shutdown != nil {
		a.shutdown()
	}
}
