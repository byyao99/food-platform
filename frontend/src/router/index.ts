import { createRouter, createWebHistory } from 'vue-router'
import MenuView from '../views/MenuView.vue'
import OrdersView from '../views/OrdersView.vue'
import LoginView from '../views/LoginView.vue'
import UsersView from '../views/UsersView.vue'
import { isAdmin, isAuthenticated } from '../session'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/menu' },
    { path: '/menu', name: 'menu', component: MenuView },
    { path: '/orders', name: 'orders', component: OrdersView, meta: { requiresAuth: true } },
    { path: '/users', name: 'users', component: UsersView, meta: { requiresAuth: true, requiresAdmin: true } },
    { path: '/login', name: 'login', component: LoginView },
  ],
})

// Gate routes that require a signed-in user (and, for some, an admin), and keep
// authenticated users away from the login page.
router.beforeEach((to) => {
  if (to.meta.requiresAuth && !isAuthenticated.value) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (to.meta.requiresAdmin && !isAdmin.value) {
    return { name: 'menu' }
  }
  if (to.name === 'login' && isAuthenticated.value) {
    return { name: 'menu' }
  }
})
