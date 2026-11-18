import { createRouter, createWebHistory } from 'vue-router'
import { useSessionStore } from './stores/session'
import MainLayout from './components/MainLayout.vue'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: () => import('./pages/LoginPage.vue'), meta: { public: true } },
    {
      path: '/',
      component: MainLayout,
      redirect: '/record',
      children: [
        { path: 'record', component: () => import('./pages/RecordPage.vue') },
        { path: 'trend', component: () => import('./pages/TrendPage.vue') },
        { path: 'family', component: () => import('./pages/FamilyPage.vue') },
        { path: 'me', component: () => import('./pages/MePage.vue') },
      ],
    },
    { path: '/readings/:id/edit', component: () => import('./pages/ReadingEditPage.vue'), props: true },
    { path: '/profiles/new', component: () => import('./pages/ProfileEditPage.vue') },
    { path: '/profiles/:id/edit', component: () => import('./pages/ProfileEditPage.vue'), props: true },
    { path: '/:pathMatch(.*)*', redirect: '/record' },
  ],
})

router.beforeEach(async (to) => {
  const session = useSessionStore()
  if (!session.loaded) {
    await session.load()
  }
  if (to.meta.public) {
    return session.me ? '/record' : true
  }
  return session.me ? true : '/login'
})
