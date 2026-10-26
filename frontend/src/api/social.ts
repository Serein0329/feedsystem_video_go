import { postJson } from './client'
import { listOrEmpty, normalizeAccount } from './normalize'
import type {
  Account,
  GetAllFollowersResponse,
  GetAllVloggersResponse,
  IsFollowedResponse,
  PageResponse,
} from './types'

export function follow(vloggerId: number) {
  return postJson<null>('/social/follow', { vlogger_id: vloggerId }, { authRequired: true })
}

export function unfollow(vloggerId: number) {
  return postJson<null>('/social/unfollow', { vlogger_id: vloggerId }, { authRequired: true })
}

export function isfollowed(vloggerId: number) {
  return postJson<IsFollowedResponse>('/social/isfollowed', { vlogger_id: vloggerId }, { authRequired: true })
}

export async function getAllFollowers(vloggerId?: number, options?: { limit?: number; last_id?: number }) {
  const page = await postJson<PageResponse<Account>>(
    '/social/getAllFollowers',
    {
      ...(vloggerId ? { vlogger_id: vloggerId } : {}),
      limit: options?.limit ?? 20,
      last_id: options?.last_id ?? 0,
    },
    { authRequired: true },
  )
  const res: GetAllFollowersResponse = {
    ...page,
    followers: listOrEmpty(page.list).map(normalizeAccount),
  }
  return res
}

export async function getAllVloggers(followerId?: number, options?: { limit?: number; last_id?: number }) {
  const page = await postJson<PageResponse<Account>>(
    '/social/getAllVloggers',
    {
      ...(followerId ? { follower_id: followerId } : {}),
      limit: options?.limit ?? 20,
      last_id: options?.last_id ?? 0,
    },
    { authRequired: true },
  )
  const res: GetAllVloggersResponse = {
    ...page,
    vloggers: listOrEmpty(page.list).map(normalizeAccount),
  }
  return res
}
