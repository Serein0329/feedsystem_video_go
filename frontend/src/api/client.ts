import { useAuthStore } from '../stores/auth'

export class ApiError extends Error {
  status: number
  code?: number
  payload?: unknown

  constructor(message: string, status: number, payload?: unknown, code?: number) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.payload = payload
  }
}

type ApiEnvelope = {
  code?: number
  msg?: string
  data?: unknown
}

type ApiErrorData = {
  error?: string
}

const API_BASE = (import.meta.env.VITE_API_BASE as string | undefined) ?? '/api'

let isRefreshing = false
let refreshPromise: Promise<string | null> | null = null

function isEnvelope(value: unknown): value is ApiEnvelope {
  return !!value && typeof value === 'object' && typeof (value as ApiEnvelope).code === 'number'
}

function unwrapEnvelope(value: unknown): unknown {
  return isEnvelope(value) ? value.data : value
}

function errorMessage(value: unknown, fallback: string): string {
  if (!isEnvelope(value)) return fallback
  const data = value.data
  if (data && typeof data === 'object' && typeof (data as ApiErrorData).error === 'string') {
    return (data as ApiErrorData).error as string
  }
  if (typeof value.msg === 'string' && value.msg) return value.msg
  return fallback
}

async function tryRefresh(): Promise<string | null> {
  const auth = useAuthStore()
  if (!auth.refreshToken) return null
  if (isRefreshing) return refreshPromise
  isRefreshing = true
  refreshPromise = (async () => {
    try {
      const res = await fetch(`${API_BASE}/account/refresh`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refresh_token: auth.refreshToken }),
      })
      if (!res.ok) { auth.clearTokens(); return null }
      const body = await res.json()
      const data = unwrapEnvelope(body) as { token?: string } | null
      if (!data?.token) { auth.clearTokens(); return null }
      auth.setToken(data.token)
      return data.token
    } catch {
      auth.clearTokens()
      return null
    } finally {
      isRefreshing = false
    }
  })()
  return refreshPromise
}

export async function postJson<T>(path: string, body: unknown, options?: { authRequired?: boolean }): Promise<T> {
  const auth = useAuthStore()
  const token = auth.token

  if (options?.authRequired && !token) {
    throw new ApiError('需要先登录（缺少 token）', 401)
  }

  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (token) headers.Authorization = `Bearer ${token}`

  const res = await fetch(`${API_BASE}${path}`, {
    method: 'POST',
    headers,
    body: JSON.stringify(body ?? {}),
  })

  if (res.status === 401 && path !== '/account/refresh') {
    const newToken = await tryRefresh()
    if (newToken) {
      headers.Authorization = `Bearer ${newToken}`
      const retryRes = await fetch(`${API_BASE}${path}`, {
        method: 'POST', headers, body: JSON.stringify(body ?? {}),
      })
      return handleResponse<T>(retryRes, path)
    }
  }

  return handleResponse<T>(res, path)
}

export async function postForm<T>(path: string, body: FormData, options?: { authRequired?: boolean }): Promise<T> {
  const auth = useAuthStore()
  const token = auth.token

  if (options?.authRequired && !token) {
    throw new ApiError('需要先登录（缺少 token）', 401)
  }

  const headers: Record<string, string> = {}
  if (token) headers.Authorization = `Bearer ${token}`

  const res = await fetch(`${API_BASE}${path}`, {
    method: 'POST',
    headers,
    body,
  })

  if (res.status === 401 && path !== '/account/refresh') {
    const newToken = await tryRefresh()
    if (newToken) {
      headers.Authorization = `Bearer ${newToken}`
      const retryRes = await fetch(`${API_BASE}${path}`, {
        method: 'POST', headers, body,
      })
      return handleResponse<T>(retryRes, path)
    }
  }

  return handleResponse<T>(res, path)
}

async function handleResponse<T>(res: Response, path: string): Promise<T> {
  const auth = useAuthStore()
  const text = await res.text()
  let body: unknown = null
  if (text) {
    try { body = JSON.parse(text) } catch { body = text }
  }

  if (!res.ok) {
    if (res.status === 401) auth.clearTokens()
    const code = isEnvelope(body) ? body.code : undefined
    const apiErr = new ApiError(errorMessage(body, `请求失败 (${res.status})`), res.status, body, code)
    if (import.meta.env.DEV) console.error('[api]', path, res.status, apiErr.message)
    throw apiErr
  }

  return unwrapEnvelope(body) as T
}
