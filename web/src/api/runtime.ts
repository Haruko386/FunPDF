import { http, unwrapApiResponse } from '@/api/http'
import type { RuntimeInfo } from '@/api/types'

export async function getRuntimeInfo() {
  const response = await http.get<RuntimeInfo | { code: number; data: RuntimeInfo }>('/runtime/info')
  return unwrapApiResponse(response.data)
}

export async function openRuntimePath(path: string) {
  await http.post('/runtime/open-path', { path })
}
