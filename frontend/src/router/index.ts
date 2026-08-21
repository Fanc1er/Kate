import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { ORG_ROUTES, STATIC_ROUTES, canAccess } from '../config/routes'
import { getOrgId } from '../api/http'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    ...STATIC_ROUTES,
    {
      path: '/',
      component: () => import('../layouts/DefaultLayout.vue'),
      children: ORG_ROUTES,
    },
    {
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      component: () => import('../views/error/NotFoundView.vue'),
      meta: { title: '404' },
    },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  document.title = `${String(to.meta.title ?? 'CInsight')} · CInsight`

  if (!auth.isLoggedIn) {
    if (to.path === '/login') return true
    return { path: '/login' }
  }

  if (to.path === '/login') {
    return { path: '/' }
  }

  // 确保用户信息已加载。
  if (!auth.user) {
    try {
      await auth.fetchMe()
    } catch {
      return { path: '/login' }
    }
  }

  // 需要组织上下文但未选组织：跳组织选择。
  const needsOrg = to.matched.some((r) => r.meta.requiresOrg)
  const hasOrg = !!getOrgId()
  if (needsOrg && !hasOrg) {
    return { path: '/select-org' }
  }

  // 角色越权。
  if (!canAccess(to.meta.roles as string[] | undefined, auth.role)) {
    return { path: '/403' }
  }

  return true
})

router.addRoute({
  path: '/403',
  name: 'forbidden',
  component: () => import('../views/error/ForbiddenView.vue'),
  meta: { title: '无权限' },
})

export default router
