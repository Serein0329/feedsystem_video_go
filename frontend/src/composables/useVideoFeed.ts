import { computed, reactive, ref } from 'vue'
import { ApiError } from '../api/client'
import * as feedApi from '../api/feed'
import type { FeedVideoItem } from '../api/types'
import { useAuthStore } from '../stores/auth'

export type TabKey = 'recommend' | 'hot' | 'following'

export function useVideoFeed() {
  const auth = useAuthStore()
  const tab = ref<TabKey>('recommend')

  const recommend = reactive({
    items: [] as FeedVideoItem[],
    loading: false, error: '',
    hasMore: false, nextTime: 0,
  })

  const hot = reactive({
    items: [] as FeedVideoItem[],
    loading: false, error: '',
    hasMore: false,
    nextLikesCount: undefined as number | undefined,
    nextId: undefined as number | undefined,
  })

  const following = reactive({
    items: [] as FeedVideoItem[],
    loading: false, error: '',
    hasMore: false, nextId: 0,
  })

  const currentState = computed(() => {
    if (tab.value === 'hot') return hot
    if (tab.value === 'following') return following
    return recommend
  })

  async function loadRecommend(reset: boolean) {
    if (recommend.loading) return
    recommend.loading = true
    recommend.error = ''
    try {
      const res = await feedApi.listLatest({ limit: 10, latest_time: reset ? 0 : recommend.nextTime })
      recommend.hasMore = res.has_more
      recommend.nextTime = res.next_time
      recommend.items = reset ? res.video_list : recommend.items.concat(res.video_list)
    } catch (e) {
      recommend.error = e instanceof ApiError ? e.message : String(e)
    } finally {
      recommend.loading = false
    }
  }

  async function loadHot(reset: boolean) {
    if (hot.loading) return
    hot.loading = true
    hot.error = ''
    try {
      const res = await feedApi.listLikesCount({
        limit: 10,
        last_likes_count: reset ? undefined : hot.nextLikesCount,
        last_id: reset ? undefined : hot.nextId,
      })
      hot.hasMore = res.has_more
      hot.nextLikesCount = res.next_likes_count
      hot.nextId = res.next_id
      hot.items = reset ? res.video_list : hot.items.concat(res.video_list)
    } catch (e) {
      hot.error = e instanceof ApiError ? e.message : String(e)
    } finally {
      hot.loading = false
    }
  }

  async function loadFollowing(reset: boolean) {
    if (!auth.isLoggedIn) {
      following.error = '登录后才能查看关注流'
      return
    }
    if (following.loading) return
    following.loading = true
    following.error = ''
    try {
      const res = await feedApi.listByFollowing({ limit: 10, latest_id: reset ? 0 : following.nextId })
      following.hasMore = res.has_more
      following.nextId = res.next_id
      following.items = reset ? res.video_list : following.items.concat(res.video_list)
    } catch (e) {
      following.error = e instanceof ApiError ? e.message : String(e)
    } finally {
      following.loading = false
    }
  }

  async function ensureTabLoaded() {
    if (tab.value === 'recommend' && recommend.items.length === 0) await loadRecommend(true)
    if (tab.value === 'hot' && hot.items.length === 0) await loadHot(true)
    if (tab.value === 'following' && following.items.length === 0) await loadFollowing(true)
  }

  async function loadMoreIfNeeded(activeIndex: number) {
    const items = currentState.value.items
    if (items.length === 0) return
    if (activeIndex < items.length - 3) return
    if (tab.value === 'recommend' && recommend.hasMore) await loadRecommend(false)
    if (tab.value === 'hot' && hot.hasMore) await loadHot(false)
    if (tab.value === 'following' && following.hasMore) await loadFollowing(false)
  }

  return { tab, recommend, hot, following, currentState, loadRecommend, loadHot, loadFollowing, ensureTabLoaded, loadMoreIfNeeded }
}
