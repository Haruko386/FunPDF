# FunPDF Desktop

FunPDF Desktop is the Windows desktop entry for FunPDF. It uses Wails to embed the Vue frontend and starts a local Gin backend for the existing `/api/...` routes.

## Runtime

- Frontend assets: `desktop/frontend/dist`
- Local backend: `127.0.0.1:38600`
- Desktop database: SQLite
- Default data directory: `<UserConfigDir>/FunPDF`
- Default database file: `<UserConfigDir>/FunPDF/FunPDF.db`
- Default cache directory: `<UserConfigDir>/FunPDF/cache`

The SQLite database stays in the user config directory. PDF source files, editor state, and runtime cache files are stored under the cache directory.

## Build

Build the web frontend first, then copy the generated `dist` directory into the desktop frontend folder:

```powershell
cd ..\web
npm ci
npm run build
cd ..
Copy-Item -Recurse -Force web\dist desktop\frontend\
```

Then build the desktop app:

```powershell
cd desktop
wails build
```

The executable is generated at:

```text
desktop/build/bin/FunPDF.exe
```

## Development

Run from the `desktop` directory:

```powershell
wails dev
```

The desktop app serves embedded assets and proxies `/api/...` calls to the local Gin backend.

## Notes

- The desktop build does not require MySQL.
- The web/server development entry at the repository root still uses MySQL.
- If the app opens with a blank window, rebuild `web/dist` and copy it to `desktop/frontend/dist`.
- If API calls fail, check whether port `38600` is already occupied.
