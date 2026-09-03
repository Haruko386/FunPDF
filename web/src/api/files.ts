import { http, unwrapApiResponse } from './http'
import type { FunPdfEditorState } from '@/types/project'
import type { Album } from './types'

export interface CachedFile {
  id: string
  name: string
  thumbnail: string
  mime_type: string
  size: number
  sha256: string
  revision: number
  status: string
  created_at?: string
  updated_at?: string
}

export interface SaveEditorStateResult {
  file_id: string
  revision: number
  saved_at: string
}

export interface UpdateFileRequest {
  name: string
  mime_type: string
}

export async function listFiles() {
  const response = await http.get<CachedFile[] | { code: number; data: CachedFile[] }>('/files')
  return unwrapApiResponse<CachedFile[]>(response.data)
}

export async function deleteFile(fileId: string) {
  await http.delete(`/files/${encodeURIComponent(fileId)}`)
}

export async function deleteFileCache(fileId: string) {
  await http.delete(`/files/${encodeURIComponent(fileId)}/cache`)
}

export async function listFileAlbums(fileId: string) {
  const response = await http.get<Album[] | { code: number; data: Album[] }>(
    `/files/${encodeURIComponent(fileId)}/album`,
  )
  return unwrapApiResponse<Album[]>(response.data)
}

export async function updateFile(fileId: string, payload: UpdateFileRequest) {
  const response = await http.put<CachedFile | { code: number; data: CachedFile }>(
    `/files/${encodeURIComponent(fileId)}`,
    payload,
  )
  return unwrapApiResponse<CachedFile>(response.data)
}

/** Expected backend endpoint: returns Cache/{id}/source.pdf as application/pdf. */
export async function getCachedFileContent(fileId: string) {
  const response = await http.get<ArrayBuffer>(`/files/${encodeURIComponent(fileId)}/content`, {
    responseType: 'arraybuffer',
    timeout: 120_000,
  })
  return response.data
}

/** Expected backend endpoint: returns the parsed editor-state.json payload. */
export async function getCachedEditorState(fileId: string) {
  const response = await http.get<FunPdfEditorState | { code: number; data: FunPdfEditorState }>(
    `/files/${encodeURIComponent(fileId)}/state`,
  )
  return unwrapApiResponse<FunPdfEditorState>(response.data)
}

/** First Ctrl+S: send the immutable source PDF and the initial editable state. */
export async function cachePdfFile(file: File, editorState: FunPdfEditorState) {
  const form = new FormData()
  form.append('file', file, file.name)
  form.append('editor_state', JSON.stringify(editorState))
  const response = await http.post<CachedFile | { code: number; data: CachedFile }>(
    '/files',
    form,
    { timeout: 120_000 },
  )
  return unwrapApiResponse<CachedFile>(response.data)
}

/** Desktop-only endpoint: imports a local PDF path into the cache library. */
export async function importLocalPdfPath(path: string) {
  const response = await http.post<CachedFile | { code: number; data: CachedFile }>(
    '/files/import-path',
    { path },
    { timeout: 120_000 },
  )
  return unwrapApiResponse<CachedFile>(response.data)
}

/** Later Ctrl+S: update editor-state.json only. */
export async function saveEditorState(
  fileId: string,
  expectedRevision: number,
  editorState: FunPdfEditorState,
  thumbnail?: string,
) {
  const payload: Record<string, unknown> = {
    expected_revision: expectedRevision,
    editor_state: editorState,
  }
  if (thumbnail) payload.thumbnail = thumbnail
  const response = await http.patch<SaveEditorStateResult | { code: number; data?: SaveEditorStateResult }>(
    `/files/${encodeURIComponent(fileId)}/state`,
    payload,
    { timeout: 30_000 },
  )
  if (response.data && typeof response.data === 'object' && 'code' in response.data && !response.data.data) {
    return {
      file_id: fileId,
      revision: expectedRevision + 1,
      saved_at: editorState.saved_at,
    }
  }
  if (response.data && typeof response.data === 'object' && 'code' in response.data) {
    return response.data.data as SaveEditorStateResult
  }
  return response.data as SaveEditorStateResult
}
