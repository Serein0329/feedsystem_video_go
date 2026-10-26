import { postJson } from './client'
import { listOrEmpty, normalizeVideoList } from './normalize'
import type { IsLikedResponse, PageResponse, Video } from './types'

export function like(videoId: number) {
  return postJson<null>('/like/like', { video_id: videoId }, { authRequired: true })
}

export function unlike(videoId: number) {
  return postJson<null>('/like/unlike', { video_id: videoId }, { authRequired: true })
}

export function isLiked(videoId: number) {
  return postJson<IsLikedResponse>('/like/isLiked', { video_id: videoId }, { authRequired: true })
}

export async function listMyLikedVideos(options?: { last_id?: number; limit?: number }) {
  const page = await postJson<PageResponse<Video>>(
    '/like/listMyLikedVideos',
    { last_id: options?.last_id ?? 0, limit: options?.limit ?? 20 },
    { authRequired: true },
  )
  return {
    ...page,
    list: normalizeVideoList(listOrEmpty(page.list)),
  }
}
