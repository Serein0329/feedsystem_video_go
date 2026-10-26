import { defineStore } from 'pinia'
import { ref } from 'vue'

import { ApiError } from '../api/client'
import * as notificationApi from '../api/notification'
import type { Notification } from '../api/types'
import { useAuthStore } from './auth'

const PAGE_SIZE = 20

export const useNotificationStore = defineStore('notification', () => {
  const auth = useAuthStore()

  const items = ref<Notification[]>([])
  const unread = ref(0)
  const loading = ref(false)
  const error = ref('')
  const hasMore = ref(false)

  let closeStream: (() => void) | null = null
  let streamToken = ''
  let retryTimer: ReturnType<typeof setTimeout> | null = null
  let retryDelay = 5000

  function clearRetry() {
    if (retryTimer !== null) {
      clearTimeout(retryTimer)
      retryTimer = null
    }
  }

  function disconnect() {
    closeStream?.()
    closeStream = null
    streamToken = ''
    clearRetry()
  }

  function scheduleReconnect() {
    if (retryTimer !== null || !auth.isLoggedIn) return
    const delay = retryDelay
    retryDelay = Math.min(retryDelay * 2, 60000)
    retryTimer = setTimeout(() => {
      retryTimer = null
      connect()
    }, delay)
  }

  function clear() {
    disconnect()
    items.value = []
    unread.value = 0
    hasMore.value = false
    error.value = ''
  }

  async function refreshUnread() {
    if (!auth.isLoggedIn) {
      unread.value = 0
      return
    }
    try {
      const res = await notificationApi.unreadCount()
      unread.value = res.count
    } catch {
      // 未读数失败不阻塞页面
    }
  }

  async function load(reset = true) {
    if (!auth.isLoggedIn) {
      clear()
      return
    }
    if (loading.value) return
    loading.value = true
    error.value = ''
    try {
      const lastId = reset ? 0 : items.value[items.value.length - 1]?.id ?? 0
      const page = await notificationApi.listNotifications({ limit: PAGE_SIZE, last_id: lastId })
      items.value = reset ? page.list : items.value.concat(page.list)
      hasMore.value = page.has_more
      await refreshUnread()
    } catch (e) {
      error.value = e instanceof ApiError ? e.message : String(e)
    } finally {
      loading.value = false
    }
  }

  function connect() {
    if (!auth.isLoggedIn || !auth.token) {
      disconnect()
      return
    }
    if (closeStream && streamToken === auth.token) return
    disconnect()
    streamToken = auth.token
    closeStream = notificationApi.openNotificationStream(auth.token, {
      onOpen: () => {
        retryDelay = 5000
      },
      onNotification: (notification) => {
        items.value = [notification, ...items.value]
        unread.value += 1
      },
      onError: () => {
        if (streamToken !== auth.token) return
        closeStream = null
        streamToken = ''
        scheduleReconnect()
      },
    })
  }

  async function markRead(id?: number) {
    await notificationApi.markRead(id)
    if (typeof id === 'number') {
      const target = items.value.find((item) => item.id === id)
      if (target) target.is_read = true
    } else {
      for (const item of items.value) item.is_read = true
    }
    await refreshUnread()
  }

  return { items, unread, loading, error, hasMore, connect, disconnect, clear, load, markRead, refreshUnread }
})
