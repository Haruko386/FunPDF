# FunPDF Desktop Minimal Run Guide

This document is the step-by-step plan for making the first desktop version of FunPDF runnable.

The goal is not to build the final release package yet. The goal is:

```text
Double click / run one desktop command
  -> open a desktop window
  -> load the current Vue UI
  -> call the existing Gin APIs
  -> keep the current backend architecture mostly unchanged
```

> [!IMPORTANT]
> For the first desktop version, do not rewrite the frontend, do not replace the existing router, and do not migrate storage yet. The fastest safe path is Wails + embedded Vue assets + local Gin backend.

---

## 0. Current project shape

The current project already has these useful parts:

```text
FunPDF/
  main.go                  # current web/server entry
  internal/router.go       # existing Gin API router
  internal/handler/        # existing handlers
  internal/service/        # existing services
  web/                     # Vue frontend
  desktop/app.go           # currently only contains: package desktop
```

The desktop version should reuse:

- the existing Vue frontend;
- the existing Gin router;
- the existing handlers/services/entities;
- the existing version source in `internal/common/version.go`.

The desktop version should add:

- a Wails desktop entry;
- a tiny local backend server;
- an embedded copy of the built Vue frontend;
- a small bridge/proxy so frontend `/api/...` requests still reach Gin.

---

## 1. Install desktop tooling

Install Wails CLI:

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

Then verify the environment:

```powershell
wails doctor
```

You want the important items to be OK:

```text
Go
Node.js
npm
WebView2
```

> [!NOTE]
> On Windows, Wails uses Microsoft WebView2. If `wails doctor` reports WebView2 missing, install the WebView2 Runtime first.

---

## 2. Keep the first desktop build dependent on the current database

Right now `main.go` initializes MySQL through:

```go
dao.InitMysql(dsn)
```

For the first runnable desktop version, keep this requirement.

That means the minimum desktop version still needs MySQL running, just like the current backend.

> [!WARNING]
> Do not start by migrating the desktop version to SQLite. The project already imports SQLite, but changing storage at the same time as adding Wails creates two large variables. First make the desktop shell run. Then migrate storage later if needed.

Minimum expectation:

```powershell
$env:FUNPDF_MYSQL_DSN="root:password@(127.0.0.1:3306)/funpdf?charset=utf8mb4&parseTime=True&loc=Local"
```

If `FUNPDF_MYSQL_DSN` is not set, continue using the existing default from `main.go`.

---

## 3. Create a reusable backend bootstrap

The current `main.go` mixes these jobs together:

1. create handlers;
2. create Gin router;
3. initialize database;
4. run database migration;
5. start the PDF text cleanup ticker;
6. start HTTP server;
7. wait for OS interrupt.

The desktop entry needs jobs 1 to 6, but not job 7.

So the first real code task should be extracting the reusable parts from `main.go`.

Create a new file:

```text
internal/server.go
```

Use package:

```go
package internal
```

Suggested functions:

```go
func NewHTTPHandler() *gin.Engine
```

This function should:

- create all handlers;
- create `gin.Default()`;
- create `internal.NewRouter(...)`;
- call `router.Setup(r)`;
- return `r`.

Then add:

```go
func InitDatabaseFromEnv() error
```

This function should:

- read `FUNPDF_MYSQL_DSN`;
- fall back to the existing default DSN;
- call `dao.InitMysql(dsn)`;
- run the same `AutoMigrate(...)` currently in `main.go`;
- return errors instead of calling `log.Fatalf`.

Then add:

```go
func StartPDFTextCleaner(ctx context.Context)
```

This function should:

- create the current 30-minute ticker;
- clear `engine.PDFText`;
- stop when `ctx.Done()` is closed.

> [!TIP]
> This is not a new architecture. It is just moving existing startup code into reusable functions so both `main.go` and the desktop entry can call the same backend setup.

After this extraction, update root `main.go` so it still behaves the same:

```text
main.go
  -> internal.InitDatabaseFromEnv()
  -> internal.NewHTTPHandler()
  -> internal.StartPDFTextCleaner(ctx)
  -> http.Server.ListenAndServe()
  -> graceful shutdown
```

Verify the normal server mode still works before touching Wails:

```powershell
go test ./...
go run .
```

---

## 4. Fix the desktop package layout

Current file:

```text
desktop/app.go
```

currently contains:

```go
package desktop
```

For a simple Wails app inside `desktop/`, use:

```go
package main
```

So `desktop/app.go` and `desktop/main.go` can belong to the same executable package.

The minimal `App` should only hold lifecycle state:

```go
type App struct {
    ctx context.Context
    shutdown context.CancelFunc
}
```

Suggested methods:

```go
func NewApp() *App
func (a *App) startup(ctx context.Context)
func (a *App) shutdownApp(ctx context.Context)
```

For the first version:

- `startup` only stores Wails context;
- `shutdownApp` cancels the backend context if it exists.

> [!IMPORTANT]
> Do not add business methods to Wails bindings yet. Keep all business behavior going through existing HTTP APIs. This keeps the desktop port small and testable.

---

## 5. Decide how Vue assets get embedded

Go `embed` cannot embed files from a parent directory.

That means this will not work inside `desktop/main.go`:

```go
//go:embed all:../web/dist
```

Instead, for the first version, copy the built frontend into the desktop folder:

```text
desktop/
  frontend/
    dist/
      index.html
      assets/
```

Manual build steps:

```powershell
cd web
npm run build
cd ..
```

Then copy:

```powershell
New-Item -ItemType Directory -Force desktop/frontend | Out-Null
Copy-Item -Recurse -Force web/dist desktop/frontend/
```

After copying, this should exist:

```text
desktop/frontend/dist/index.html
```

Then `desktop/main.go` can use:

```go
//go:embed all:frontend/dist
var assets embed.FS
```

> [!NOTE]
> Later we can automate this copy with a script or Wails build command. For the first learning pass, do it manually so every moving part is visible.

---

## 6. Create the desktop Wails entry

Create:

```text
desktop/main.go
```

This file should do four jobs:

1. initialize database;
2. start the Gin backend on `127.0.0.1:38600`;
3. start Wails;
4. stop the backend when the Wails window exits.

High-level structure:

```go
package main

import (
    "context"
    "embed"
    "errors"
    "log"
    "net/http"
    "net/http/httputil"
    "net/url"
    "strings"
    "time"

    funpdf "FunPDF/internal"
    "FunPDF/internal/common"

    "github.com/wailsapp/wails/v2"
    "github.com/wailsapp/wails/v2/pkg/options"
    "github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS
```

Use a fixed local backend address first:

```go
const backendAddr = "127.0.0.1:38600"
```

Then create a small API proxy handler.

Purpose:

```text
Vue requests /api/files
  -> Wails asset server receives /api/files
  -> proxy forwards it to http://127.0.0.1:38600/api/files
```

The proxy should only forward `/api/`.

Pseudo-structure:

```go
func newAPIProxy() http.Handler {
    target, err := url.Parse("http://" + backendAddr)
    if err != nil {
        panic(err)
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
```

Start backend before `wails.Run(...)`:

```go
func startBackend(ctx context.Context) (*http.Server, error) {
    if err := funpdf.InitDatabaseFromEnv(); err != nil {
        return nil, err
    }

    handler := funpdf.NewHTTPHandler()
    funpdf.StartPDFTextCleaner(ctx)

    srv := &http.Server{
        Addr:    backendAddr,
        Handler: handler,
    }

    go func() {
        err := srv.ListenAndServe()
        if err != nil && !errors.Is(err, http.ErrServerClosed) {
            log.Printf("desktop backend error: %v", err)
        }
    }()

    return srv, nil
}
```

Then in `main()`:

```go
func main() {
    app := NewApp()

    backendCtx, cancelBackend := context.WithCancel(context.Background())
    app.shutdown = cancelBackend

    backend, err := startBackend(backendCtx)
    if err != nil {
        log.Fatalf("start desktop backend: %v", err)
    }

    defer func() {
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        _ = backend.Shutdown(shutdownCtx)
        cancelBackend()
    }()

    err = wails.Run(&options.App{
        Title:  "FunPDF " + common.GetVersion(),
        Width:  1280,
        Height: 860,
        AssetServer: &assetserver.Options{
            Assets:  assets,
            Handler: newAPIProxy(),
        },
        OnStartup:  app.startup,
        OnShutdown: app.shutdownApp,
        Bind: []interface{}{
            app,
        },
    })
    if err != nil {
        log.Fatal(err)
    }
}
```

> [!CAUTION]
> The backend must listen on `127.0.0.1`, not `0.0.0.0`. A desktop app should not expose its local API to the LAN by default.

---

## 7. Add Wails dependency

After writing the Wails imports, run:

```powershell
go mod tidy
```

This should add Wails dependencies to `go.mod` and `go.sum`.

Then verify the desktop package compiles:

```powershell
go test ./...
go build ./desktop
```

> [!NOTE]
> If `go build ./desktop` fails because `desktop/frontend/dist` does not exist, build and copy the frontend first.

---

## 8. Add `desktop/wails.json`

Create:

```text
desktop/wails.json
```

Minimum content:

```json
{
  "$schema": "https://wails.io/schemas/config.v2.json",
  "name": "FunPDF",
  "outputfilename": "FunPDF",
  "frontend:dir": "frontend",
  "frontend:install": "",
  "frontend:build": "",
  "frontend:dev:watcher": "",
  "frontend:dev:serverUrl": "",
  "author": {
    "name": "Haruko386",
    "email": ""
  }
}
```

For this first pass, keep frontend commands empty because you manually build `web/dist` and copy it into `desktop/frontend/dist`.

Run Wails from the desktop directory:

```powershell
cd desktop
wails dev
```

If that works, build:

```powershell
wails build
```

Expected output will be under a Wails build folder, usually similar to:

```text
desktop/build/bin/FunPDF.exe
```

---

## 9. First-run checklist

Before running the desktop app:

1. MySQL is running.
2. Database `funpdf` exists.
3. Frontend was built:

```powershell
cd web
npm run build
cd ..
```

4. Frontend was copied:

```powershell
New-Item -ItemType Directory -Force desktop/frontend | Out-Null
Copy-Item -Recurse -Force web/dist desktop/frontend/
```

5. Desktop package builds:

```powershell
go build ./desktop
```

6. Wails can run:

```powershell
cd desktop
wails dev
```

---

## 10. Expected minimum result

When the first desktop version runs correctly:

- a native desktop window opens;
- the current Vue UI is visible;
- requests to `/api/...` are handled by the local Gin backend;
- existing file, album, translation, provider, and AI APIs continue using the same router;
- closing the window shuts down the backend server.

This is the minimum acceptable desktop version.

---

## 11. Common errors and how to read them

### Blank white window

Most likely:

- `desktop/frontend/dist/index.html` does not exist;
- `//go:embed all:frontend/dist` points to the wrong directory;
- frontend assets were not copied after `npm run build`.

Check:

```powershell
Get-ChildItem desktop/frontend/dist
```

### API requests fail

Most likely:

- Gin backend did not start;
- port `38600` is occupied;
- proxy does not forward `/api/...`;
- database initialization failed.

Check whether the backend port is listening:

```powershell
netstat -ano | findstr 38600
```

### Desktop starts, but library data is missing

Most likely:

- desktop process is using a different MySQL DSN;
- `FUNPDF_MYSQL_DSN` differs between terminal sessions.

Print or log the DSN source while debugging, but do not log passwords in a release build.

### `wails` command not found

Most likely:

- Go bin directory is not in `PATH`;
- terminal was not restarted after installing Wails.

Check:

```powershell
go env GOPATH
```

The Wails binary is usually under:

```text
%GOPATH%\bin
```

---

## 12. What not to do in this first pass

Do not implement these yet:

- automatic updates;
- installer;
- code signing;
- file association;
- system tray;
- SQLite migration;
- secure keychain storage;
- Wails native business bindings;
- frontend rewrite.

> [!IMPORTANT]
> The first desktop milestone is successful when the current web app runs inside a native window. Everything else is release hardening.

---

## 13. After the minimum version runs

After this works, the next practical improvements are:

1. replace MySQL with SQLite for desktop mode;
2. move runtime data to user app data directory;
3. add app icon and Windows metadata;
4. add an installer;
5. add file association for `.pdf` or `.funpdf`;
6. store provider API keys in the OS credential store.

Do these only after the basic Wails shell is stable.

---

## References

- Wails application development guide: https://wails.io/docs/guides/application-development/
- Wails dynamic assets / asset handler guide: https://wails.io/docs/guides/dynamic-assets/
- Wails troubleshooting guide: https://wails.io/docs/guides/troubleshooting/
