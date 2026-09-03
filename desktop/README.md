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

Build the web frontend directly into the desktop asset directory:

```powershell
cd ..\web
npm ci
npm run build:desktop
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

## Release assets

Tagged releases build and upload only the desktop application assets:

- `FunPDF-Installer-<tag>.exe`
- `FunPDF-Installer-<tag>.exe.sha256`
- `FunPDF-<tag>-windows.zip`

The GitHub-generated source archives may still appear separately.

## Development

Run from the `desktop` directory:

```powershell
wails dev
```

The desktop app serves embedded assets and proxies `/api/...` calls to the local Gin backend.

## Notes

- The desktop build does not require MySQL.
- The web/server development entry at the repository root still uses MySQL.
- If the app opens with a blank window, rebuild the embedded frontend with `npm run build:desktop`.
- If API calls fail, check whether port `38600` is already occupied.
