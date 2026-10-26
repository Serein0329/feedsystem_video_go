import { postJson } from './client'
import { normalizeFeedVideoList } from './normalize'
import type {
  ListByFollowingResponse,
  ListByPopularityResponse,
  ListByTagResponse,
  ListLatestResponse,
  ListLikesCountResponse,
} from './types'

export async function listLatest(input: { limit: number; latest_time: number }) {
  const res = await postJson<ListLatestResponse>('/feed/listLatest', input)
  return { ...res, video_list: normalizeFeedVideoList(res.video_list) }
}

export async function listLikesCount(input: { limit: number; last_likes_count?: number; last_id?: number }) {
  const body: Record<string, unknown> = { limit: input.limit }
  if (typeof input.last_likes_count === 'number' && typeof input.last_id === 'number') {
    body.last_likes_count = input.last_likes_count
    body.last_id = input.last_id
  }
  const res = await postJson<ListLikesCountResponse>('/feed/listLikesCount', body)
  return { ...res, video_list: normalizeFeedVideoList(res.video_list) }
}

export async function listByPopularity(input: { limit: number; asof: number; offset: number }) {
  const res = await postJson<ListByPopularityResponse>('/feed/listByPopularity', input)
  return { ...res, video_list: normalizeFeedVideoList(res.video_list) }
}

export async function listByFollowing(input: { limit: number; latest_id: number }) {
  const res = await postJson<ListByFollowingResponse>('/feed/listByFollowing', input, { authRequired: true })
  return { ...res, video_list: normalizeFeedVideoList(res.video_list) }
}

export async function listByTag(input: { tag_name: string; limit: number; last_id: number }) {
  const res = await postJson<ListByTagResponse>('/feed/listByTag', input)
  return { ...res, video_list: normalizeFeedVideoList(res.video_list) }
}
