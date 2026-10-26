<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { useRoute } from 'vue-router'

import AppShell from '../components/AppShell.vue'
import FeedVideoCard from '../components/FeedVideoCard.vue'
import { ApiError } from '../api/client'
import * as feedApi from '../api/feed'
import * as likeApi from '../api/like'
import type { FeedVideoItem } from '../api/types'
import { useAuthStore } from '../stores/auth'
import { useToastStore } from '../stores/toast'

const route = useRoute()
const auth = useAuthStore()
const toast = useToastStore()

const tagName = computed(() => (typeof route.params.name === 'string' ? route.params.name : ''))

const state = reactive({
  loading: false,
  error: '',
  items: [] as FeedVideoItem[],
  hasMore: false,
  nextId: 0,
})

const likeBusy = reactive<Record<string, boolean>>({})

let loadReq = 0

async function load(reset: boolean) {
  if (!tagName.value) return
  if (!reset && state.loading) return
  const req = ++loadReq
  if (reset) {
    state.items = []
    state.hasMore = false
    state.nextId = 0
  }
  state.loading = true
  state.error = ''
  try {
    const res = await feedApi.listByTag({
      tag_name: tagName.value,
      limit: 20,
      last_id: reset ? 0 : state.nextId,
    })
    if (req !== loadReq) return
    state.items = reset ? res.video_list : state.items.concat(res.video_list)
    state.hasMore = res.has_more
    state.nextId = res.next_id
  } catch (e) {
    if (req !== loadReq) return
    state.error = e instanceof ApiError ? e.message : String(e)
    if (reset) state.items = []
  } finally {
    if (req === loadReq) state.loading = false
  }
}

async function toggleLike(item: FeedVideoItem) {
  if (!auth.isLoggedIn) {
    toast.error('请先登录')
    return
  }
  const key = String(item.id)
  if (likeBusy[key]) return
  likeBusy[key] = true
  try {
    if (item.is_liked) await likeApi.unlike(item.id)
    else await likeApi.like(item.id)
    item.is_liked = !item.is_liked
    item.likes_count = Math.max(0, item.likes_count + (item.is_liked ? 1 : -1))
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e))
  } finally {
    likeBusy[key] = false
  }
}

watch(tagName, () => {
  void load(true)
}, { immediate: true })
</script>

<template>
  <AppShell>
    <div class="card">
      <p class="title" style="margin: 0">#{{ tagName }}</p>
      <p class="subtle" style="margin: 6px 0 0">按标签筛选（/feed/listByTag）</p>

      <div v-if="state.loading && state.items.length === 0" class="subtle" style="margin-top: 12px">加载中…</div>
      <div v-else-if="state.error" class="pill bad" style="margin-top: 12px">{{ state.error }}</div>
      <div v-else-if="state.items.length === 0" class="subtle" style="margin-top: 12px">该标签下暂无视频</div>

      <div v-else class="grid" style="gap: 12px; margin-top: 14px">
        <FeedVideoCard
          v-for="item in state.items"
          :key="item.id"
          :item="item"
          :can-like="auth.isLoggedIn"
          :busy="!!likeBusy[String(item.id)]"
          @toggle-like="toggleLike"
        />
      </div>

      <div v-if="state.hasMore" class="row" style="justify-content: center; margin-top: 14px">
        <button type="button" :disabled="state.loading" @click="load(false)">加载更多</button>
      </div>
    </div>
  </AppShell>
</template>
