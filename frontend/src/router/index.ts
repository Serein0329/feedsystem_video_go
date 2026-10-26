import { createRouter, createWebHistory } from 'vue-router'

import HomeView from '../views/HomeView.vue'
import HotView from '../views/HotView.vue'
import SearchView from '../views/SearchView.vue'
import TagView from '../views/TagView.vue'
import NotificationView from '../views/NotificationView.vue'
import VideoView from '../views/VideoView.vue'
import VideoDetailView from '../views/VideoDetailView.vue'
import AccountView from '../views/AccountView.vue'
import ChangePasswordView from '../views/ChangePasswordView.vue'
import RegisterView from '../views/RegisterView.vue'
import SettingsView from '../views/SettingsView.vue'
import UserProfileView from '../views/UserProfileView.vue'
import MessageView from '../views/MessageView.vue'
import { useAuthStore } from '../stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: HomeView },
    { path: '/feed', redirect: '/' },
    { path: '/hot', name: 'hot', component: HotView },
    { path: '/search', name: 'search', component: SearchView },
    { path: '/tag/:name', name: 'tag', component: TagView, props: true },
    { path: '/notifications', name: 'notifications', component: NotificationView, meta: { requiresAuth: true } },
    { path: '/video', name: 'video', component: VideoView, meta: { requiresAuth: true } },
    { path: '/video/:id', name: 'video-detail', component: VideoDetailView, props: true },
    { path: '/account', name: 'account', component: AccountView },
    { path: '/account/register', name: 'account-register', component: RegisterView },
    { path: '/account/change-password', name: 'account-change-password', component: ChangePasswordView, meta: { requiresAuth: true } },
    { path: '/settings', name: 'settings', component: SettingsView, meta: { requiresAuth: true } },
    { path: '/u/:id', name: 'user-profile', component: UserProfileView, props: true },
    { path: '/messages', name: 'message-list', component: MessageView, meta: { requiresAuth: true } },
    { path: '/messages/:peerId', name: 'messages', component: MessageView, meta: { requiresAuth: true } },
  ],
})

router.beforeEach((to, _from, next) => {
  const auth = useAuthStore()
  if (to.meta.requiresAuth && !auth.isLoggedIn) {
    next({ path: '/account', query: { redirect: to.fullPath } })
    return
  }
  next()
})

export default router
