# FunPDF Backend API Reference

This document describes all HTTP APIs currently registered in `internal/router.go`. Every business API is mounted under the `/api` prefix.

> [!IMPORTANT]
> This document follows `internal/router.go`. It only describes routes that are currently registered. If handlers, DTOs, or frontend API calls change, update this document in the same change set.

> [!NOTE]
> Most JSON endpoints use the following response envelope:
>
> ```json
> {
>   "code": 200,
>   "data": {},
>   "msg": "success"
> }
> ```
>
> Translator handlers currently use `message` instead of `msg`. Some delete endpoints return only an HTTP status code with no JSON body.

## Common conventions

- `:file_id`: PDF file ID in the local library.
- `:album_id`: Album ID.
- `:translator_name`: Translator name, such as `baidu`, `deepl`, `google`, or `azure`.
- `:provider_id`: AI provider ID.
- `:session_id`: PDF AI chat session ID.

> [!TIP]
> The frontend usually calls these APIs through `web/src/api/*.ts`. When debugging an API issue, first confirm the route exists in `internal/router.go`, then check the matching handler for request fields and status codes.

## API overview

| Method | Path | Meaning |
| --- | --- | --- |
| `GET` | `/api/files` | List files in the local library |
| `POST` | `/api/files` | Upload a PDF and save its initial editor state |
| `PUT` | `/api/files/:file_id` | Update file metadata |
| `DELETE` | `/api/files/:file_id` | Delete a file record and move its cache directory |
| `GET` | `/api/files/:file_id/content` | Read the original PDF content |
| `GET` | `/api/files/:file_id/state` | Read the editor-state JSON |
| `GET` | `/api/files/:file_id/thumbnail` | Read the file thumbnail |
| `PATCH` | `/api/files/:file_id/state` | Save editor state and an optional thumbnail |
| `GET` | `/api/files/:file_id/album` | List albums that contain the file |
| `DELETE` | `/api/files/:file_id/cache` | Clear the in-memory PDF text cache |
| `GET` | `/api/albums` | List albums |
| `POST` | `/api/albums` | Create an album |
| `GET` | `/api/albums/:album_id` | List files in an album |
| `PUT` | `/api/albums/:album_id` | Update album metadata |
| `DELETE` | `/api/albums/:album_id` | Delete an album |
| `POST` | `/api/albums/:album_id/files` | Add files to an album in batch |
| `DELETE` | `/api/albums/:album_id/files` | Remove files from an album in batch |
| `GET` | `/api/translators` | List configured translators |
| `POST` | `/api/translators` | Create a translator configuration |
| `POST` | `/api/translators/:translator_name` | Translate text with the selected translator |
| `GET` | `/api/providers` | List configured AI providers |
| `POST` | `/api/providers` | Create an AI provider |
| `PATCH` | `/api/providers/:provider_id` | Update an AI provider |
| `DELETE` | `/api/providers/:provider_id` | Delete an AI provider |
| `POST` | `/api/providers/:provider_id/chat` | Call a provider model directly |
| `GET` | `/api/providers/:provider_id/list` | Fetch supported models from the provider |
| `GET` | `/api/providers/:provider_id/models` | List locally saved provider models |
| `POST` | `/api/providers/:provider_id/models` | Save provider models locally |
| `DELETE` | `/api/providers/:provider_id/models` | Delete locally saved provider models |
| `POST` | `/api/providers/:provider_id/sessions` | Create a PDF AI chat session |
| `DELETE` | `/api/providers/:provider_id/sessions/:session_id` | Delete a PDF AI chat session |
| `POST` | `/api/providers/:provider_id/sessions/:session_id/messages` | Send a message to a PDF AI chat session |

## Files API

The Files API manages PDF storage, editor state, thumbnails, revisions, and runtime cache for the public file library.

> [!IMPORTANT]
> Original PDF files and editor state are stored in the local `Cache` directory. MySQL stores file metadata. When a file is deleted, the current service logic moves its cache directory to `Cache/.trash` and deletes the database record.

### `GET /api/files`

Lists all files in the file library.

The response `data` is an array of file records. A single file record roughly looks like this:

```json
{
  "id": "file-id",
  "name": "paper.pdf",
  "thumbnail": "data:image/png;base64,...",
  "mime_type": "application/pdf",
  "size": 123456,
  "sha256": "hex",
  "revision": 1,
  "status": "active"
}
```

### `POST /api/files`

Uploads a PDF on the first save. The request type is `multipart/form-data`.

Fields:

- `file`: The PDF file.
- `editor_state`: Initial editor state as a JSON string.

> [!WARNING]
> The handler uses `http.MaxBytesReader` to cap the request body at about `200 MiB`. Requests over the limit, or requests with invalid `editor_state` JSON, return `400`.

On success, the endpoint returns `201` and the new file record in `data`.

### `PUT /api/files/:file_id`

Updates file metadata, mainly for renaming.

Request:

```json
{
  "name": "new-name.pdf",
  "mime_type": "application/pdf"
}
```

On success, the endpoint returns the updated file record.

### `DELETE /api/files/:file_id`

Deletes a file. On success, the endpoint returns `204 No Content`.

> [!CAUTION]
> This endpoint deletes the database file record and moves the local cache directory. From the user's perspective, this deletes the public file itself; it is not just an album unlink operation.

### `GET /api/files/:file_id/content`

Returns the original PDF file. The response `Content-Type` is `application/pdf`.

> [!NOTE]
> This endpoint is not a JSON response. Frontend clients should read it as binary data or `arraybuffer`.

### `GET /api/files/:file_id/state`

Reads the file's `editor-state.json`, which is used to restore annotations, notes, translation snippets, and other editor state.

### `GET /api/files/:file_id/thumbnail`

Reads the file thumbnail. The response `data` is usually a data URL or thumbnail string.

### `PATCH /api/files/:file_id/state`

Saves editor state and optionally updates the thumbnail.

Request:

```json
{
  "expected_revision": 1,
  "thumbnail": "data:image/png;base64,...",
  "editor_state": {
    "version": 1,
    "document": {},
    "editor": {}
  }
}
```

> [!IMPORTANT]
> `expected_revision` is used for optimistic locking. If the submitted revision does not match the current database revision, the handler returns `409 Conflict` to avoid overwriting another edit.

### `GET /api/files/:file_id/album`

Lists all albums that contain the file.

### `DELETE /api/files/:file_id/cache`

Clears the server-side in-memory PDF text cache. On success, the endpoint returns:

```json
{
  "code": 200,
  "msg": "success"
}
```

> [!NOTE]
> This endpoint does not delete the PDF file, editor state, or database record. It only affects runtime text cache.

## Albums API

The Albums API organizes public-library files into groups. Albums store relationships to files; the file itself still belongs to the public file library.

> [!TIP]
> Deleting an album does not delete public files. Removing a file from an album also does not delete the file itself. The endpoint that deletes a file is `DELETE /api/files/:file_id`.

### `GET /api/albums`

Lists all albums.

### `POST /api/albums`

Creates an album.

Request:

```json
{
  "name": "Paper Collection",
  "thumbnail": "data:image/png;base64,...",
  "description": "PDFs for a thesis project"
}
```

On success, the endpoint returns `201` and the created album in `data`.

### `GET /api/albums/:album_id`

Lists files in the selected album.

### `PUT /api/albums/:album_id`

Updates album name, cover image, and description.

Request:

```json
{
  "name": "Updated Album Name",
  "thumbnail": "data:image/png;base64,...",
  "description": "Updated description"
}
```

### `DELETE /api/albums/:album_id`

Deletes an album. On success, the endpoint returns `204 No Content`.

### `POST /api/albums/:album_id/files`

Adds public files to an album in batch.

Request:

```json
{
  "ids": ["file-id-1", "file-id-2"]
}
```

On success, the endpoint returns `200`. If some files fail to be added, `data` contains a map from failed file IDs to error reasons.

> [!NOTE]
> This endpoint creates `album_files` relationships. It does not copy PDF files.

### `DELETE /api/albums/:album_id/files`

Removes files from an album in batch.

Request:

```json
{
  "ids": ["file-id-1", "file-id-2"]
}
```

On success, the endpoint returns `204 No Content`.

## Translators API

The Translators API stores translation service credentials and calls translation services.

> [!IMPORTANT]
> Current translator implementations live under `internal/engine/translator`, and provider config files live under `conf/translators`. The frontend normalizes translator names; for example, `baidu-translators` becomes `baidu`.

### `GET /api/translators`

Lists configured translators.

The response `data` is an array of translator records. A single translator record looks like this:

```json
{
  "id": "translator-id",
  "name": "baidu",
  "params": {
    "api_key": "***",
    "app_id": "***"
  }
}
```

### `POST /api/translators`

Creates a translator configuration.

Example request:

```json
{
  "name": "baidu",
  "params": {
    "api_key": "your-api-key",
    "app_id": "your-app-id"
  }
}
```

Common `params`:

- `baidu`: `api_key`, `app_id`
- `deepl`: `api_key`
- `google`: `api_key`
- `azure`: `api_key`, `region`

> [!WARNING]
> Credentials are persisted by the backend. Do not commit real keys to the repository, logs, or test fixtures.

### `POST /api/translators/:translator_name`

Translates text with the selected translator.

Request:

```json
{
  "q": "Text to translate",
  "from": "auto",
  "to": "zh",
  "region": "free",
  "params": {
    "model_type": "prefer_quality_optimized"
  }
}
```

Field meanings:

- `q`: Text to translate.
- `from`: Source language. It can be omitted or set to auto-detection depending on the translator.
- `to`: Target language.
- `region`: Region information required by some translators, such as DeepL Free/Pro or Azure region.
- `params`: Translator-specific options, such as Baidu `model_type`, DeepL `formality`, Google `format`, or Azure `textType`.

On success, `data` contains the translated text string.

## Providers API

The Providers API stores AI provider configuration. Model chat and model-list endpoints depend on `base_url`, `url_suffix`, and `api_key` from the provider configuration.

> [!NOTE]
> The frontend ships presets for DeepSeek, OpenAI, SILICONFLOW, Moonshot, and Aliyun. The current backend `ModelService` chat and cloud model-list dispatch mainly implements the DeepSeek branch. This document describes endpoint meaning; it does not imply that every frontend preset has complete backend support.

### `GET /api/providers`

Lists configured AI providers.

### `POST /api/providers`

Creates an AI provider.

Request:

```json
{
  "name": "DeepSeek",
  "api_key": "your-api-key",
  "base_url": "https://api.deepseek.com",
  "url_suffix": {
    "chat": "chat/completions",
    "models": "models"
  }
}
```

On success, the endpoint returns the provider record.

### `PATCH /api/providers/:provider_id`

Updates the provider API key, base URL, and URL suffix.

Request:

```json
{
  "api_key": "new-api-key",
  "base_url": "https://api.deepseek.com",
  "url_suffix": {
    "chat": "chat/completions",
    "models": "models"
  }
}
```

### `DELETE /api/providers/:provider_id`

Deletes a provider configuration. On success, the endpoint returns:

```json
{
  "code": 200,
  "msg": "success"
}
```

## Provider Chat and Models API

These APIs are mounted under `/api/providers/:provider_id`. They handle direct model calls, provider-side model listing, and local model-list management.

> [!TIP]
> The provider-side model list and the locally saved model list are separate concepts: `/list` asks the provider for models, while `/models` manages the user-selected models stored in the local database.

### `POST /api/providers/:provider_id/chat`

Calls a provider model directly without creating a PDF chat session.

Request:

```json
{
  "model_name": "deepseek-chat",
  "model_id": "model-db-id",
  "messages": [
    { "role": "user", "content": "Hello" }
  ],
  "stream": false,
  "temperature": 0.7,
  "top_p": 1,
  "max_tokens": 2048,
  "thinking": false,
  "effort": "default",
  "verbosity": "medium"
}
```

Non-streaming response `data`:

```json
{
  "answer": "model answer",
  "reason_content": ""
}
```

Streaming responses use SSE:

```http
event: message
data: [MESSAGE]answer delta

event: message
data: [REASONING]reasoning delta

event: done
data: [DONE]
```

> [!NOTE]
> The direct chat endpoint uses a different SSE data format from the PDF session message endpoint. The frontend supports both JSON deltas and `[MESSAGE]` / `[REASONING]` prefixes.

### `GET /api/providers/:provider_id/list`

Fetches supported models from the provider.

Response `data`:

```json
[
  { "name": "deepseek-chat" }
]
```

### `GET /api/providers/:provider_id/models`

Lists locally saved models for the provider.

Response `data`:

```json
[
  { "id": "model-id", "name": "deepseek-chat" }
]
```

### `POST /api/providers/:provider_id/models`

Saves model names to the local database.

Request:

```json
{
  "names": ["deepseek-chat", "deepseek-reasoner"]
}
```

On success, the endpoint returns the saved model records.

### `DELETE /api/providers/:provider_id/models`

Deletes locally saved provider models.

Request:

```json
{
  "ids": ["model-id-1", "model-id-2"]
}
```

On success, the endpoint returns:

```json
{
  "code": 200,
  "msg": "success"
}
```

## Chat Sessions API

The PDF AI chat session API binds a file, provider, model, and system prompt into an independent session, then sends follow-up messages inside that session.

> [!IMPORTANT]
> Session creation currently uses `file_id`. The server can read or cache PDF text from the saved file. The frontend requires the PDF to be saved to the file library before the first AI question.

### `POST /api/providers/:provider_id/sessions`

Creates a PDF AI chat session.

Request:

```json
{
  "file_id": "file-id",
  "model_id": "model-db-id",
  "model_name": "deepseek-chat",
  "system_prompt": "You are a rigorous academic paper reading assistant."
}
```

Response:

```json
{
  "code": 200,
  "data": {
    "id": "session-id"
  },
  "msg": "success"
}
```

### `DELETE /api/providers/:provider_id/sessions/:session_id`

Deletes a PDF AI chat session. On success, the endpoint returns:

```json
{
  "code": 200,
  "msg": "success"
}
```

> [!TIP]
> The frontend calls this endpoint when closing a PDF tab to release the chat session for that PDF.

### `POST /api/providers/:provider_id/sessions/:session_id/messages`

Sends a message to a PDF AI chat session. Both streaming and non-streaming modes are supported.

Request:

```json
{
  "content": "What are the main contributions of this paper?",
  "quote": "Optional selected text quote from the PDF",
  "stream": true,
  "temperature": 0.7,
  "top_p": 1,
  "max_tokens": 2048,
  "thinking": false,
  "effort": "default"
}
```

Non-streaming response `data`:

```json
{
  "content": "Answer content",
  "reasoning_content": "Optional reasoning content"
}
```

Streaming responses use SSE:

```http
event: message
data: {"content":"answer delta","reasoning_content":""}

event: message
data: {"content":"","reasoning_content":"reasoning delta"}

event: done
data: [DONE]
```

> [!WARNING]
> The session message endpoint depends on an existing session. Calls fail if the session has been deleted, the provider does not exist, or the model configuration is invalid.

## Status codes and error handling

- `200 OK`: Normal success.
- `201 Created`: File or album created.
- `204 No Content`: File deleted, album deleted, or files removed from an album.
- `400 Bad Request`: Missing path parameter, failed request-body parsing, empty required field, or invalid JSON.
- `404 Not Found`: Target not found in some provider, model, or file scenarios.
- `409 Conflict`: File editor-state save revision conflict.
- `500 Internal Server Error`: Database, filesystem, external provider, or internal logic error.

> [!CAUTION]
> The current API has no unified authentication layer. Before exposing the service to an untrusted network, add authentication, authorization, key protection, upload limits, and CORS policy.

> [!NOTE]
> `router.go` currently does not register `/api/index`. If the frontend or older documentation mentions that endpoint, confirm whether it has been removed or has not yet been connected to the router.
