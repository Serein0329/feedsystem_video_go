import { postJson } from './client'
import { listOrEmpty, normalizeCommentList } from './normalize'
import type { Comment, PageResponse } from './types'

export async function listAll(videoId: number, options?: { limit?: number; last_id?: number }) {
  const page = await postJson<PageResponse<Comment>>('/comment/listAll', {
    video_id: videoId,
    limit: options?.limit ?? 20,
    last_id: options?.last_id ?? 0,
  })
  return normalizeCommentList(listOrEmpty(page.list))
}

export function publish(videoId: number, content: string) {
  return postJson<null>('/comment/publish', { video_id: videoId, content }, { authRequired: true })
}

export function remove(commentId: number) {
  return postJson<null>('/comment/delete', { comment_id: commentId }, { authRequired: true })
}
