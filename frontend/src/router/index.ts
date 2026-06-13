import { createRouter, createWebHistory } from 'vue-router'
import MenuView from '../views/MenuView.vue'
import OrdersView from '../views/OrdersView.vue'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/menu' },
    { path: '/menu', name: 'menu', component: MenuView },
    { path: '/orders', name: 'orders', component: OrdersView },
  ],
})
