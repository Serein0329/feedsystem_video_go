import { postJson } from './client'
import type { Notification, PageResponse } from './types'

const API_BASE = (import.meta.env.VITE_API_BASE as string | undefined) ?? '/api'

export type NotificationStreamHandlers = {
  onNotification: (notification: Notification) => void
  onOpen?: () => void
  onError: () => void
}

/**
 * 用 fetch 读取 SSE 流，token 走 Authorization 头，避免出现在 URL/访问日志里。
 * 返回关闭函数。
 */
export function openNotificationStream(token: string, handlers: NotificationStreamHandlers): () => void {
  const controller = new AbortController()
  let closed = false

  async function run() {
    const res = await fetch(`${API_BASE}/notification/stream`, {
      method: 'GET',
      headers: { Authorization: `Bearer ${token}`, Accept: 'text/event-stream' },
      signal: controller.signal,
    })
    if (!res.ok || !res.body) {
      throw new Error(`notification stream failed (${res.status})`)
    }

    handlers.onOpen?.()
    const reader = res.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''

    for (;;) {
      const { value, done } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true }).replace(/\r\n/g, '\n')

      let sep = buffer.indexOf('\n\n')
      while (sep >= 0) {
        const frame = buffer.slice(0, sep)
        buffer = buffer.slice(sep + 2)
        const data = frame
          .split('\n')
          .filter((line) => line.startsWith('data:'))
          .map((line) => line.slice(5).trimStart())
          .join('\n')
        if (data) {
          try {
            handlers.onNotification(JSON.parse(data) as Notification)
          } catch {
            // 忽略坏帧
          }
        }
        sep = buffer.indexOf('\n\n')
      }
    }
    if (!closed) handlers.onError()
  }

  run().catch(() => {
    if (!closed) handlers.onError()
  })

  return () => {
    closed = true
    controller.abort()
  }
}

export async function listNotifications(options?: { limit?: number; last_id?: number }) {
  return postJson<PageResponse<Notification>>(
    '/notification/list',
    { limit: options?.limit ?? 50, last_id: options?.last_id ?? 0 },
    { authRequired: true },
  )
}

export function markRead(id?: number) {
  return postJson<null>('/notification/markRead', id ? { id } : {}, { authRequired: true })
}

export function unreadCount() {
  return postJson<{ count: number }>('/notification/unreadCount', {}, { authRequired: true })
}
