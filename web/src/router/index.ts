import { createRouter, createWebHashHistory } from 'vue-router'
import ConfigPage from '@/views/ConfigPage.vue'
import MonitorPage from '@/views/MonitorPage.vue'

const routes = [
  { path: '/', redirect: '/login' },
  { path: '/login', name: 'login', component: () => import('@/views/LoginPage.vue'), meta: { title: '登录', noAuth: true } },
  { path: '/config', name: 'config', component: ConfigPage, meta: { title: '配置管理' } },
  { path: '/monitor', name: 'monitor', component: MonitorPage, meta: { title: '运行监控' } },
  { path: '/trend', name: 'trend', component: () => import('@/views/TrendPage.vue'), meta: { title: '实时趋势' } },
  { path: '/users', name: 'users', component: () => import('@/views/UserManage.vue'), meta: { title: '用户管理' } },
  { path: '/detail/:id', name: 'detail', component: () => import('@/views/DetailPage.vue'), meta: { title: '实例详情' } },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

// Auth guard
router.beforeEach((to, _from, next) => {
  if (to.meta.noAuth) {
    next()
    return
  }
  const token = localStorage.getItem('token')
  if (!token) {
    next('/login')
    return
  }
  const payload = token.split('.')[0]
  try {
    const claims = JSON.parse(atob(payload))
    if (claims.exp * 1000 < Date.now()) {
      localStorage.removeItem('token')
      localStorage.removeItem('user')
      next('/login')
      return
    }
  } catch {
    // Invalid token, let it through - backend will reject
  }
  next()
})

export default router
