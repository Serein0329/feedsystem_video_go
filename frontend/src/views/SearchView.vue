<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import AppShell from '../components/AppShell.vue'
import UserAvatar from '../components/UserAvatar.vue'
import { ApiError } from '../api/client'
import * as accountApi from '../api/account'
import * as videoApi from '../api/video'
import type { Account, Video } from '../api/types'

const route = useRoute()
const router = useRouter()

const q = computed(() => (typeof route.query.q === 'string' ? route.query.q.trim() : ''))

const videos = reactive({
  loading: false,
  error: '',
  items: [] as Video[],
  hasMore: false,
  nextId: 0,
})

const users = reactive({
  loading: false,
  error: '',
  items: [] as Account[],
})

let videosReq = 0
let usersReq = 0

async function loadVideos(reset: boolean) {
  if (!q.value) return
  if (!reset && videos.loading) return
  const req = ++videosReq
  if (reset) {
    videos.items = []
    videos.hasMore = false
    videos.nextId = 0
  }
  videos.loading = true
  videos.error = ''
  try {
    const page = await videoApi.searchVideos({ keyword: q.value, limit: 20, last_id: reset ? 0 : videos.nextId })
    if (req !== videosReq) return
    videos.items = reset ? page.list : videos.items.concat(page.list)
    videos.hasMore = page.has_more
    videos.nextId = page.next_id
  } catch (e) {
    if (req !== videosReq) return
    videos.error = e instanceof ApiError ? e.message : String(e)
    if (reset) videos.items = []
  } finally {
    if (req === videosReq) videos.loading = false
  }
}

async function loadUsers() {
  if (!q.value) {
    users.items = []
    return
  }
  const req = ++usersReq
  users.loading = true
  users.error = ''
  try {
    const page = await accountApi.findSimilarName({ username: q.value, limit: 10, last_id: 0 })
    if (req !== usersReq) return
    users.items = page.list
  } catch (e) {
    if (req !== usersReq) return
    users.error = e instanceof ApiError ? e.message : String(e)
    users.items = []
  } finally {
    if (req === usersReq) users.loading = false
  }
}

async function search() {
  if (!q.value) {
    videosReq += 1
    usersReq += 1
    videos.items = []
    videos.hasMore = false
    videos.nextId = 0
    videos.loading = false
    users.items = []
    users.loading = false
    return
  }
  await Promise.all([loadVideos(true), loadUsers()])
}

async function goVideo(id: number) {
  await router.push(`/video/${id}`)
}

async function goUser(id: number) {
  await router.push(`/u/${id}`)
}

watch(q, () => {
  void search()
}, { immediate: true })
</script>

<template>
  <AppShell>
    <div class="card">
      <div class="row" style="justify-content: space-between; align-items: baseline">
        <div>
          <p class="title" style="margin: 0">搜索</p>
          <p class="subtle" style="margin: 6px 0 0">
            关键词：{{ q || '（空）' }} · 视频 /video/search · 用户 /account/findSimilarName
          </p>
        </div>
      </div>

      <div v-if="!q" class="subtle" style="margin-top: 12px">在顶部输入关键词后回车</div>
    </div>

    <template v-if="q">
      <div class="card" style="margin-top: 14px">
        <p class="title" style="margin: 0">用户</p>
        <div v-if="users.loading" class="subtle" style="margin-top: 10px">加载中…</div>
        <div v-else-if="users.error" class="pill bad" style="margin-top: 10px">{{ users.error }}</div>
        <div v-else-if="users.items.length === 0" class="subtle" style="margin-top: 10px">没有匹配的用户</div>
        <div v-else class="user-list" style="margin-top: 10px">
          <button v-for="u in users.items" :key="u.id" class="user-row" type="button" @click="goUser(u.id)">
            <UserAvatar :username="u.username" :id="u.id" :size="40" />
            <span class="user-meta">
              <span class="user-name">@{{ u.username }}</span>
              <span class="user-id mono">#{{ u.id }}</span>
            </span>
          </button>
        </div>
      </div>

      <div class="card" style="margin-top: 14px">
        <p class="title" style="margin: 0">视频</p>
        <div v-if="videos.loading && videos.items.length === 0" class="subtle" style="margin-top: 10px">加载中…</div>
        <div v-else-if="videos.error" class="pill bad" style="margin-top: 10px">{{ videos.error }}</div>
        <div v-else-if="videos.items.length === 0" class="subtle" style="margin-top: 10px">没有匹配的视频</div>

        <div v-else class="video-grid" style="margin-top: 12px">
          <button v-for="v in videos.items" :key="v.id" class="video-card" type="button" @click="goVideo(v.id)">
            <img class="video-cover" :src="v.cover_url" :alt="v.title" loading="lazy" />
            <div class="video-meta">
              <div class="video-title">{{ v.title }}</div>
              <div class="video-sub subtle">@{{ v.username }} · ❤️ {{ v.likes_count }}</div>
            </div>
          </button>
        </div>

        <div v-if="videos.hasMore" class="row" style="justify-content: center; margin-top: 14px">
          <button type="button" :disabled="videos.loading" @click="loadVideos(false)">加载更多</button>
        </div>
      </div>
    </template>
  </AppShell>
</template>

<style scoped>
.user-list {
  display: grid;
  gap: 10px;
}
.user-row {
  display: flex;
  gap: 10px;
  align-items: center;
  padding: 10px 12px;
  border-radius: 12px;
  border: 1px solid var(--border);
  background: rgba(255, 255, 255, 0.04);
  color: var(--text);
  cursor: pointer;
  text-align: left;
}
.user-row:hover {
  background: rgba(255, 255, 255, 0.08);
}
.user-meta {
  display: grid;
  gap: 2px;
}
.user-name {
  font-weight: 700;
  font-size: 13px;
}
.user-id {
  font-size: 12px;
  color: var(--muted);
}
.video-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 12px;
}
.video-card {
  display: grid;
  gap: 8px;
  padding: 8px;
  border-radius: 12px;
  border: 1px solid var(--border);
  background: rgba(255, 255, 255, 0.04);
  color: var(--text);
  cursor: pointer;
  text-align: left;
}
.video-card:hover {
  background: rgba(255, 255, 255, 0.08);
}
.video-cover {
  width: 100%;
  aspect-ratio: 9 / 12;
  object-fit: cover;
  border-radius: 10px;
  background: rgba(0, 0, 0, 0.35);
}
.video-meta {
  display: grid;
  gap: 4px;
  min-width: 0;
}
.video-title {
  font-size: 13px;
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.video-sub {
  font-size: 12px;
}
</style>
