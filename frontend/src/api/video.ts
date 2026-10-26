import { postForm, postJson } from './client'
import { listOrEmpty, normalizeVideoList } from './normalize'
import type { PageResponse, Video } from './types'

export function publishVideo(input: { title: string; description: string; play_url: string; cover_url: string }) {
  return postJson<Video>('/video/publish', input, { authRequired: true })
}

export function deleteVideo(id: number) {
  return postJson<null>('/video/deleteVideo', { id }, { authRequired: true })
}

export type UploadResponse = { play_url?: string; cover_url?: string }

export function uploadVideo(file: File) {
  const fd = new FormData()
  fd.append('file', file)
  return postForm<UploadResponse>('/video/uploadVideo', fd, { authRequired: true })
}

export function uploadCover(file: File) {
  const fd = new FormData()
  fd.append('file', file)
  return postForm<UploadResponse>('/video/uploadCover', fd, { authRequired: true })
}

export async function listByAuthorId(authorId: number, options?: { last_id?: number; limit?: number }) {
  const page = await postJson<PageResponse<Video>>('/video/listByAuthorID', {
    author_id: authorId,
    last_id: options?.last_id ?? 0,
    limit: options?.limit ?? 20,
  })
  return {
    ...page,
    list: normalizeVideoList(listOrEmpty(page.list)),
  }
}

export async function searchVideos(input: { keyword: string; limit?: number; last_id?: number }) {
  const page = await postJson<PageResponse<Video>>('/video/search', {
    keyword: input.keyword,
    limit: input.limit ?? 20,
    last_id: input.last_id ?? 0,
  })
  return {
    ...page,
    list: normalizeVideoList(listOrEmpty(page.list)),
  }
}

export function getDetail(id: number) {
  return postJson<Video>('/video/getByID', { id })
}

// --- Chunk Upload API ---

export type InitChunkUploadResponse = {
  upload_id: string
  uploaded_chunks: number[]
}

export function initChunkUpload(input: {
  filename: string
  file_size: number
  chunk_size: number
  total_chunks: number
  file_hash: string
}) {
  return postJson<InitChunkUploadResponse>('/video/chunk/init', input, { authRequired: true })
}

export function uploadChunk(uploadId: string, chunkIndex: number, chunkHash: string, blob: Blob) {
  const fd = new FormData()
  fd.append('upload_id', uploadId)
  fd.append('chunk_index', String(chunkIndex))
  fd.append('chunk_hash', chunkHash)
  fd.append('file', blob)
  return postForm<{ chunk_index: number }>('/video/chunk/upload', fd, { authRequired: true })
}

export function chunkStatus(uploadId: string) {
  return postJson<{ upload_id: string; uploaded_chunks: number[]; total_chunks: number }>(
    '/video/chunk/status',
    { upload_id: uploadId },
    { authRequired: true },
  )
}

export function completeChunkUpload(uploadId: string) {
  return postJson<UploadResponse>('/video/chunk/complete', { upload_id: uploadId }, { authRequired: true })
}
