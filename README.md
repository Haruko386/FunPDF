<div align="center">
<img src="./external/FunPDF-Banner.png" width="420" alt="FunPDF logo">
</div>

<p align="center">
  <a href="./README.md" style="text-decoration: none;"><img alt="README in English" src="https://img.shields.io/badge/English-DBEDFA" style="display: inline-block; vertical-align: middle;"></a>
  <a href="./external/README_zh.md" style="text-decoration: none;"><img alt="README in Simplified Chinese" src="https://img.shields.io/badge/%E7%AE%80%E4%BD%93%E4%B8%AD%E6%96%87-DFE0E5" style="display: inline-block; vertical-align: middle;"></a>
</p>


<div align="center">

  [![Language](./external/badge/Vue-Golang-00ADD8.svg)](https://github.com/haruko386/funpdf)
  [![Release](./external/badge/Latest%20release-blue.svg)](https://github.com/haruko386/funpdf/releases/latest)
  [![Language](./external/badge/License-GNU%20General%20Public.svg)](https://github.com/haruko386/funpdf/blob/main/LICENSE)
</div>

<div align="center" style="margin-top:20px;margin-bottom:20px;">
  <img width="1200" alt="FunPDF-Banner" src="https://github.com/user-attachments/assets/bb015eea-50f6-4463-8e46-5dc9f82ce0e8" />
</div>

<details open>
<summary>📕 <strong>Table of Contents</strong></summary>

- [💡 What is FunPDF?](#what-is-funpdf)
- [🎮 Get Started](#get-started)
- [🌟 Key Features](#key-features)
- [🎬 Run with Docker](#run-with-docker)
- [🔧 Configurations](#configurations)
- [🔨 Build a Docker Image](#build-a-docker-image)
- [🛠️ Launch from Source for Development](#launch-from-source-for-development)
- [📖 Documentation](#documentation)
- [🙌 Contributing](#contributing)

</details>

<a id="what-is-funpdf"></a>
## 💡 What is FunPDF?

FunPDF is a lightweight, self-hosted PDF reader and annotation prototype. It combines a Vue 3 web interface with a Go API and stores document metadata in MySQL. Uploaded PDFs and their editor state are kept in a local `Cache` directory.

> [!important]
> The `master` branch contains only the front-end and back-end applications for the web architecture. For desktop applications, please refer to the `main` branch.

<a id="get-started"></a>
## 🎮 Get Started

The quickest way to run the current full development setup is to use Docker for MySQL and the Go API, then run the Vite frontend locally.

### Prerequisites

- Docker with Docker Compose
- Node.js 18+ and npm

### Start the application

```bash
docker compose up -d --build
cd web
npm ci
npm run dev
```

Open <http://localhost:5173>. The frontend proxies `/api` requests to the Go service at `http://localhost:9384`.

<a id="key-features"></a>
## 🌟 Key Features

- PDF reading
  - Open local `.pdf` files and editable `.funpdf` project files
  - Render PDF pages with PDF.js, including text layers and clickable PDF links
  - Navigate pages, zoom in/out, fit to width, rotate, drag-and-drop files, and keep multiple PDFs open in tabs
  - Copy selected text and use the selection popup for common actions
- Annotation and editing
  - Draw freehand strokes, highlight text, add underline and strikeout marks, and erase annotations
  - Add note annotations from the toolbar or selected text
  - View note lists and saved translation snippets in the sidebar
  - Undo, redo, clear annotations, and track unsaved editor state
- Saving and export
  - Save PDF source files, editor state, thumbnails, and revisions to the local library
  - Autosave cached editor state for saved documents
  - Export or print a flattened PDF with annotations applied
  - Store editable project data in `.funpdf` format for later reopening
- Library and album management
  - Browse the public file library, reopen saved PDFs, rename files, and delete stored files
  - Create, edit, and delete albums with generated or uploaded cover images
  - Add files to albums, remove files from albums without deleting the source file, and inspect file memberships
- Translation
  - Configure Baidu, DeepL, Google, and Azure translator credentials
  - Translate selected PDF text from the sidebar or selection popup
  - Persist source/target language and provider-specific translation options locally
  - Save translation results back onto note annotations
- AI-assisted reading
  - Configure AI providers and model lists from the sidebar
  - Create per-PDF chat sessions and stream model responses with optional reasoning content
  - Ask questions about the current PDF or a selected text quote
  - Tune chat parameters such as temperature, top-p, max tokens, thinking, and effort
  - Current backend chat/model-list implementation is wired for DeepSeek-compatible provider handling
- Runtime and deployment
  - Go API with Gin and GORM, backed by MySQL metadata storage
  - Local `Cache` storage for uploaded PDFs, editor state, thumbnails, and recoverable delete flow
  - Docker Compose setup for the backend API and MySQL
  - Feature toggles for AI Chat, translation, and notes in the UI settings panel

<a id="run-with-docker"></a>
## 🎬 Run with Docker

Start the Go API and MySQL:

```bash
docker compose up -d --build
```

Check the containers and follow the application logs:

```bash
docker compose ps
docker compose logs -f app
```

The API is available at <http://localhost:9384>. For example, `GET /api/files` lists the saved files.

The Compose setup currently builds the **backend only**. Run `npm run dev` inside `web` to use the browser interface. MySQL data and uploaded PDF data are persisted in the `mysql_data` and `app_cache` named volumes.

Stop the services without deleting their data:

```bash
docker compose down
```

To also remove the database and cached PDF volumes, run `docker compose down -v`. This permanently deletes the locally persisted FunPDF data.

<a id="configurations"></a>
## 🔧 Configurations

| Variable | Default | Description |
| --- | --- | --- |
| `FUNPDF_ADDR` | `:9384` | Address used by the Go HTTP server |
| `FUNPDF_MYSQL_DSN` | Local root connection | MySQL DSN used by the backend |

The Docker Compose defaults are intended for local development and use `root` / `password`. Change these credentials before exposing the service to any untrusted network, and update both `MYSQL_ROOT_PASSWORD` and `FUNPDF_MYSQL_DSN` together.

<a id="build-a-docker-image"></a>
## 🔨 Build a Docker Image

Build only the FunPDF backend image from the repository root:

```bash
docker build -f docker/Dockerfile -t funpdf:local .
```

The image listens on port `9384`, stores files under `/app/Cache`, and requires access to a MySQL server through `FUNPDF_MYSQL_DSN`. For most local installations, `docker compose up -d --build` is the recommended option because it creates the network, database, and persistent volumes automatically.

<a id="launch-from-source-for-development"></a>
## 🛠️ Launch from Source for Development

Requirements: Go 1.25+, Node.js 18+, npm, and MySQL 8.x.

1. Start MySQL. You can reuse the Compose service:

   ```bash
   docker compose up -d mysql
   ```

2. Set the backend configuration and run Go from the repository root.

   Bash:

   ```bash
   export FUNPDF_MYSQL_DSN='root:password@tcp(127.0.0.1:3306)/funpdf?charset=utf8mb4&parseTime=True&loc=Local'
   export FUNPDF_ADDR=':9384'
   go run .
   ```

   PowerShell:

   ```powershell
   $env:FUNPDF_MYSQL_DSN = 'root:password@tcp(127.0.0.1:3306)/funpdf?charset=utf8mb4&parseTime=True&loc=Local'
   $env:FUNPDF_ADDR = ':9384'
   go run .
   ```

3. In another terminal, start the frontend:

   ```bash
   cd web
   npm ci
   npm run dev
   ```

4. Open <http://localhost:5173>. Runtime PDF data is written to `./Cache` relative to the backend working directory.

Useful checks:

```bash
go test ./...
cd web && npm run build
```

<a id="documentation"></a>
## 📖 Documentation
- [Current file API notes](./internal/development.md)

<a id="contributing"></a>
## 🙌 Contributing

Issues and pull requests are welcome. Please keep changes focused and run the Go tests and frontend build before submitting a pull request.

## License

FunPDF is licensed under the [GNU General Public License v3.0](./LICENSE).
