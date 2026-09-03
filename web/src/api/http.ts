import axios from 'axios'

export interface ApiEnvelope<T> {
  code: number
  data: T
  message?: string
}

export const http = axios.create({
  baseURL: '/api',
  timeout: 15000,
})

export function unwrapApiResponse<T>(payload: T | ApiEnvelope<T>): T {
  if (payload && typeof payload === 'object' && 'data' in payload && 'code' in payload) {
    return (payload as ApiEnvelope<T>).data
  }
  return payload as T
}

export function apiErrorMessage(error: unknown, fallback = '请求失败，请稍后重试') {
  if (axios.isAxiosError(error)) {
    const payload = error.response?.data as { message?: string } | undefined
    return payload?.message || error.message || fallback
  }
  return error instanceof Error ? error.message : fallback
}
