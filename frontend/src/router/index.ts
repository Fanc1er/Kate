import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useLicenseStore } from '../stores/license'
import { APP_ROUTES, STATIC_ROUTES, canAccess } from '../config/routes'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    ...STATIC_ROUTES,
    {
      path: '/',
      component: () => import('../layouts/DefaultLayout.vue'),
      children: APP_ROUTES,
    },
    {
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      component: () => import('../views/error/NotFoundView.vue'),
      meta: { title: '404' },
    },
  ] as RouteRecordRaw[],
})

router.beforeEach(async (to) => {
  document.title = `${String(to.meta.title ?? 'CInsight')} · CInsight`

  // 授权门禁：任何登录/角色判断之前先校验授权状态。
  const license = useLicenseStore()
  if (!license.loaded) {
    try {
      await license.fetchStatus()
    } catch {
      license.loaded = true
      license.status = 'missing'
    }
  }
  if (to.path !== '/license') {
    if (license.status !== 'valid') {
      return { path: '/license' }
    }
  } else if (license.status === 'valid') {
    return { path: '/login' }
  }

  const auth = useAuthStore()

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
