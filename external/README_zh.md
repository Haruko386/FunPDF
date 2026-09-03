<div align="center">
  <img src="./FunPDF-Banner.png" width="420" alt="FunPDF 标志">
</div>

<p align="center">
  <a href="../README.md" style="text-decoration: none;"><img alt="English README" src="https://img.shields.io/badge/English-DFE0E5" style="display: inline-block; vertical-align: middle;"></a>
  <a href="./README_zh.md" style="text-decoration: none;"><img alt="简体中文 README" src="https://img.shields.io/badge/%E7%AE%80%E4%BD%93%E4%B8%AD%E6%96%87-DBEDFA" style="display: inline-block; vertical-align: middle;"></a>
</p>

<div align="center">

  [![Language](../external/badge/Vue-Golang-00ADD8.svg)](https://github.com/haruko386/funpdf)
  [![Release](../external/badge/Latest%20release-blue.svg)](https://github.com/haruko386/funpdf/releases/latest)
  [![Language](../external/badge/License-GNU%20General%20Public.svg)](https://github.com/haruko386/funpdf/blob/main/LICENSE)
</div>

<div align="center" style="margin-top:20px;margin-bottom:20px;">
  <img width="1200" alt="FunPDF-Banner" src="https://github.com/user-attachments/assets/bb015eea-50f6-4463-8e46-5dc9f82ce0e8" />
</div>

<details open>
<summary>📕 <strong>目录</strong></summary>

- [💡 FunPDF 是什么？](#what-is-funpdf)
- [🎮 快速开始](#get-started)
  - [环境要求](#环境要求)
  - [启动网页开发环境](#启动网页开发环境)
- [🌟 主要功能](#key-features)
- [🖥️ 桌面端](#desktop-app)
- [🎬 使用 Docker 运行](#run-with-docker)
- [🔧 配置](#configurations)
- [🔨 构建 Docker 镜像](#build-a-docker-image)
- [🛠️ 从源码启动开发环境](#launch-from-source-for-development)
- [📖 文档](#documentation)
- [🙌 参与贡献](#contributing)
- [开源许可](#开源许可)

</details>

<a id="what-is-funpdf"></a>
## 💡 FunPDF 是什么？

FunPDF 是一个轻量的 PDF 阅读与标注原型。项目使用 Vue 3 构建界面，使用 Go 提供 API。桌面端通过 Wails 嵌入 Vue UI，并使用 SQLite 保存元数据；网页/服务端开发模式使用 MySQL。上传的 PDF 及其编辑状态保存在本地缓存目录中。

<a id="get-started"></a>
## 🎮 快速开始

普通用户建议使用桌面端。开发网页/服务端模式时，可以使用 Docker 运行 MySQL 和 Go API，再在本地运行 Vite 前端。

### 环境要求

- Go 1.25+
- Node.js 18+ 与 npm
- 如需运行网页/服务端开发环境，需要 Docker 与 Docker Compose
- 如需从源码构建桌面端，需要 Wails CLI 和 WebView2

### 启动网页开发环境

```bash
docker compose up -d --build
cd web
npm ci
npm run dev
```

打开 <http://localhost:5173>。前端会把 `/api` 请求代理到 `http://localhost:9384` 的 Go 服务。

<a id="key-features"></a>
## 🌟 主要功能

- PDF 阅读
  - 打开本地 `.pdf` 文件和可编辑的 `.funpdf` 工程文件
  - 使用 PDF.js 渲染页面、文本层和 PDF 内部/外部链接
  - 支持翻页、缩放、适应宽度、旋转、拖拽打开文件，以及多 PDF 标签页
  - 支持复制选中文字，并通过选区浮层执行常用操作
- 标注与编辑
  - 支持画笔、高亮、下划线、删除线和橡皮擦
  - 支持通过工具栏或选中文字创建便签
  - 侧栏可查看便签列表和已保存的翻译片段
  - 支持撤销、重做、清空标注，并跟踪未保存编辑状态
- 保存与导出
  - 将 PDF 原文件、编辑状态、缩略图和修订号保存到本地文件库
  - 对已保存文档自动保存编辑状态
  - 导出或打印已扁平化的 PDF，标注会写入导出文件
  - 使用 `.funpdf` 格式保存可重新打开的编辑工程
- 文件库与合集管理
  - 浏览公共文件区，重新打开已保存 PDF，重命名文件并删除已存文件
  - 创建、编辑和删除合集，支持自动生成或上传合集封面
  - 将公共文件加入合集，从合集中移除文件但不删除源文件，并查看文件所属合集
- 翻译
  - 配置百度、DeepL、Google 和 Azure 翻译器凭据
  - 从侧栏或选区浮层翻译 PDF 选中文字
  - 在本地持久化源语言、目标语言和各翻译器的运行参数
  - 可将翻译结果保存回便签标注
- AI 辅助阅读
  - 在侧栏配置 AI 服务商和模型列表
  - 为每个 PDF 创建独立对话会话，并以流式方式返回模型回答和可选思考内容
  - 支持围绕当前 PDF 或选中文字引用进行提问
  - 支持设置 temperature、top-p、max tokens、thinking 和 effort 等对话参数
  - 当前后端的聊天和云端模型列表实现主要面向 DeepSeek 兼容服务商
- 运行与部署
  - 桌面端使用 Wails 嵌入 Vue UI，通过本地 Gin API 桥接，并使用 SQLite 保存元数据
  - 网页/服务端开发模式使用 Gin、GORM 和 MySQL 保存元数据
  - 使用本地缓存目录保存上传的 PDF、编辑状态、缩略图，并提供可恢复的删除流程
  - 使用 Docker Compose 启动后端 API 与 MySQL
  - UI 设置面板支持开关 AI Chat、翻译和便签功能

<a id="desktop-app"></a>
## 🖥️ 桌面端

桌面端是当前主要分发目标。它会嵌入 `desktop/frontend/dist` 中的 Vue 构建产物，在本机启动 `127.0.0.1:38600` 的 Gin 后端，并使用 SQLite 保存桌面端元数据。

默认桌面端数据目录位于当前用户配置目录下：

```text
<UserConfigDir>/FunPDF/FunPDF.db
<UserConfigDir>/FunPDF/cache
```

从源码构建桌面端：

```powershell
cd web
npm ci
npm run build
cd ..
Copy-Item -Recurse -Force web\dist desktop\frontend\
cd desktop
wails build
```

构建产物位于：

```text
desktop/build/bin/FunPDF.exe
```

桌面端说明见 [desktop/README.md](../desktop/README.md)。

<a id="run-with-docker"></a>
## 🎬 使用 Docker 运行

启动 Go API 和 MySQL：

```bash
docker compose up -d --build
```

查看容器状态和应用日志：

```bash
docker compose ps
docker compose logs -f app
```

API 位于 <http://localhost:9384>，例如 `GET /api/files` 可以列出已保存的文件。

目前 Compose 配置只构建**后端**。如需使用浏览器界面，请进入 `web` 目录运行 `npm run dev`。MySQL 数据和上传的 PDF 分别持久化到 `mysql_data`、`app_cache` 命名卷。

停止服务但保留数据：

```bash
docker compose down
```

如需同时删除数据库和 PDF 缓存卷，可运行 `docker compose down -v`。该命令会永久删除本地持久化的 FunPDF 数据。

<a id="configurations"></a>
## 🔧 配置

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `FUNPDF_ADDR` | `:9384` | Go HTTP 服务监听地址 |
| `FUNPDF_MYSQL_DSN` | 本地 root 连接 | 后端使用的 MySQL DSN |

Docker Compose 中的 `root` / `password` 仅适合本地开发。将服务暴露到不可信网络前，请修改凭据，并确保 `MYSQL_ROOT_PASSWORD` 与 `FUNPDF_MYSQL_DSN` 同步更新。

<a id="build-a-docker-image"></a>
## 🔨 构建 Docker 镜像

在仓库根目录单独构建 FunPDF 后端镜像：

```bash
docker build -f docker/Dockerfile -t funpdf:local .
```

镜像监听 `9384` 端口，将文件保存到 `/app/Cache`，并需要通过 `FUNPDF_MYSQL_DSN` 连接 MySQL。对于大多数本地部署，推荐使用 `docker compose up -d --build`，它会自动创建网络、数据库和持久化卷。

<a id="launch-from-source-for-development"></a>
## 🛠️ 从源码启动开发环境

环境要求：Go 1.25+、Node.js 18+、npm 和 MySQL 8.x。

1. 启动 MySQL，也可以直接复用 Compose 中的服务：

   ```bash
   docker compose up -d mysql
   ```

2. 设置后端配置，并在仓库根目录启动 Go。

   Bash：

   ```bash
   export FUNPDF_MYSQL_DSN='root:password@tcp(127.0.0.1:3306)/funpdf?charset=utf8mb4&parseTime=True&loc=Local'
   export FUNPDF_ADDR=':9384'
   go run .
   ```

   PowerShell：

   ```powershell
   $env:FUNPDF_MYSQL_DSN = 'root:password@tcp(127.0.0.1:3306)/funpdf?charset=utf8mb4&parseTime=True&loc=Local'
   $env:FUNPDF_ADDR = ':9384'
   go run .
   ```

3. 在另一个终端启动前端：

   ```bash
   cd web
   npm ci
   npm run dev
   ```

4. 打开 <http://localhost:5173>。运行时 PDF 数据会写入后端工作目录下的 `./Cache`。

常用检查命令：

```bash
go test ./...
cd web && npm run build
```

<a id="documentation"></a>
## 📖 文档

- [当前文件 API 说明](../internal/development.md)

<a id="contributing"></a>
## 🙌 参与贡献

欢迎提交 Issue 和 Pull Request。提交前请尽量保持改动聚焦，并运行 Go 测试与前端构建。

## 开源许可

FunPDF 使用 [GNU General Public License v3.0](../LICENSE) 许可证。
