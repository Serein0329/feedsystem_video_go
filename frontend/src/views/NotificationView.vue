<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'

import AppShell from '../components/AppShell.vue'
import { ApiError } from '../api/client'
import type { Notification } from '../api/types'
import { useAuthStore } from '../stores/auth'
import { useNotificationStore } from '../stores/notification'
import { useToastStore } from '../stores/toast'

const router = useRouter()
const auth = useAuthStore()
const notifications = useNotificationStore()
const toast = useToastStore()

const typeLabel: Record<string, string> = {
  like: '点赞',
  comment: '评论',
  mention: '提及',
  follow: '关注',
}

const empty = computed(() => !notifications.loading && notifications.items.length === 0)

function targetRoute(item: Notification) {
  return item.type === 'follow' ? `/u/${item.target_id}` : `/video/${item.target_id}`
}

async function open(item: Notification) {
  try {
    if (!item.is_read) await notifications.markRead(item.id)
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e))
  }
  await router.push(targetRoute(item))
}

async function markAll() {
  try {
    await notifications.markRead()
    toast.success('已全部标为已读')
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e))
  }
}

async function loadMore() {
  await notifications.load(false)
}

onMounted(async () => {
  if (!auth.isLoggedIn) return
  notifications.connect()
  await notifications.load(true)
})
</script>

<template>
  <AppShell>
    <div class="card">
      <div class="row" style="justify-content: space-between; align-items: baseline">
        <div>
          <p class="title" style="margin: 0">通知</p>
          <p class="subtle" style="margin: 6px 0 0">点赞、评论、提及和关注（/notification）</p>
        </div>
        <div class="row">
          <span class="pill">未读 {{ notifications.unread }}</span>
          <button type="button" :disabled="notifications.unread === 0" @click="markAll">全部已读</button>
        </div>
      </div>

      <div v-if="notifications.error" class="pill bad" style="margin-top: 12px">{{ notifications.error }}</div>
      <div v-else-if="notifications.loading && notifications.items.length === 0" class="subtle" style="margin-top: 12px">
        加载中…
      </div>
      <div v-else-if="empty" class="subtle" style="margin-top: 12px">暂无通知</div>

      <div v-else class="notice-list" style="margin-top: 14px">
        <button
          v-for="item in notifications.items"
          :key="item.id"
          class="notice"
          :class="{ unread: !item.is_read }"
          type="button"
          @click="open(item)"
        >
          <span class="dot" :class="{ on: !item.is_read }" />
          <span class="notice-body">
            <span class="notice-line">
              <strong>@{{ item.username }}</strong>
              <span>{{ item.content }}</span>
              <span class="pill">{{ typeLabel[item.type] ?? item.type }}</span>
            </span>
            <span class="subtle mono">{{ new Date(item.created_at).toLocaleString() }}</span>
          </span>
        </button>
      </div>

      <div v-if="notifications.hasMore" class="row" style="justify-content: center; margin-top: 14px">
        <button type="button" :disabled="notifications.loading" @click="loadMore">加载更多</button>
      </div>
    </div>
  </AppShell>
</template>

<style scoped>
.notice-list {
  display: grid;
  gap: 10px;
}
.notice {
  display: grid;
  grid-template-columns: 12px minmax(0, 1fr);
  gap: 10px;
  align-items: start;
  text-align: left;
  padding: 12px 14px;
  border-radius: 12px;
  border: 1px solid var(--border);
  background: rgba(255, 255, 255, 0.04);
  color: var(--text);
  cursor: pointer;
}
.notice:hover {
  background: rgba(255, 255, 255, 0.08);
}
.notice.unread {
  border-color: rgba(43, 161, 255, 0.42);
  background: rgba(43, 161, 255, 0.1);
}
.dot {
  width: 10px;
  height: 10px;
  margin-top: 5px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.18);
}
.dot.on {
  background: rgba(43, 161, 255, 1);
}
.notice-body {
  display: grid;
  gap: 6px;
  min-width: 0;
}
.notice-line {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  font-size: 13px;
}
</style>
