import { postJson } from './client'
import type { DirectMessage, ListMessagesResponse, PageResponse } from './types'

export function sendMessage(toId: number, content: string) {
  return postJson<DirectMessage>('/message/send', { to_id: toId, content }, { authRequired: true })
}

export async function listMessages(peerId: number, options?: { limit?: number; last_id?: number }) {
  const page = await postJson<PageResponse<DirectMessage>>(
    '/message/list',
    { peer_id: peerId, limit: options?.limit ?? 50, last_id: options?.last_id ?? 0 },
    { authRequired: true },
  )
  const res: ListMessagesResponse = {
    ...page,
    messages: page.list ?? [],
  }
  return res
}
