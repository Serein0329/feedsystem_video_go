import { postForm, postJson } from './client'
import { listOrEmpty, normalizeAccount } from './normalize'
import type { Account, PageResponse, ProfileSummary, TokenResponse } from './types'

export function register(username: string, password: string) {
  return postJson<null>('/account/register', { username, password })
}

export function login(username: string, password: string) {
  return postJson<TokenResponse>('/account/login', { username, password })
}

export function logout() {
  return postJson<null>('/account/logout', {}, { authRequired: true })
}

export function rename(newUsername: string) {
  return postJson<TokenResponse>('/account/rename', { new_username: newUsername }, { authRequired: true })
}

export function changePassword(oldPassword: string, newPassword: string) {
  return postJson<null>(
    '/account/changePassword',
    { old_password: oldPassword, new_password: newPassword },
    { authRequired: true },
  )
}

export function findById(id: number) {
  return postJson<Account>('/account/findByID', { id })
}

export function findByUsername(username: string) {
  return postJson<Account>('/account/findByUsername', { username })
}

export async function findSimilarName(input: { username: string; limit?: number; last_id?: number }) {
  const page = await postJson<PageResponse<Account>>('/account/findSimilarName', {
    username: input.username,
    limit: input.limit ?? 20,
    last_id: input.last_id ?? 0,
  })
  return { ...page, list: listOrEmpty(page.list).map(normalizeAccount) }
}

export function getProfile(accountId: number) {
  return postJson<ProfileSummary>('/account/getProfile', { account_id: accountId })
}

export function uploadAvatar(file: File) {
  const fd = new FormData()
  fd.append('file', file)
  return postForm<{ avatar_url: string }>('/account/uploadAvatar', fd, { authRequired: true })
}

export function updateProfile(data: { avatar_url?: string; bio?: string }) {
  return postJson<null>('/account/updateProfile', data, { authRequired: true })
}
