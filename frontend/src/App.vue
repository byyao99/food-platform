<script setup lang="ts">
import { RouterLink, RouterView, useRouter } from 'vue-router'
import { currentUser, isAuthenticated, isAdmin, clearSession } from './session'

const router = useRouter()

function logout() {
  clearSession()
  router.push('/login')
}
</script>

<template>
  <div class="app">
    <header class="topbar">
      <h1 class="brand">🍜 Food Platform</h1>
      <nav class="nav">
        <RouterLink to="/menu" class="nav-link">Menu</RouterLink>
        <RouterLink v-if="isAuthenticated" to="/orders" class="nav-link">Orders</RouterLink>
        <RouterLink v-if="isAdmin" to="/users" class="nav-link">Users</RouterLink>
      </nav>
      <div class="account">
        <template v-if="isAuthenticated && currentUser">
          <span class="who">{{ currentUser.username }} <span class="role">{{ currentUser.role }}</span></span>
          <button class="logout" @click="logout">Logout</button>
        </template>
        <RouterLink v-else to="/login" class="nav-link">Login</RouterLink>
      </div>
    </header>

    <main class="content">
      <RouterView />
    </main>
  </div>
</template>

<style scoped>
.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 24px;
  background: #1f2937;
  color: #fff;
}
.brand {
  margin: 0;
  font-size: 20px;
}
.nav {
  display: flex;
  gap: 8px;
  margin-right: auto;
  margin-left: 24px;
}
.account {
  display: flex;
  align-items: center;
  gap: 12px;
}
.who {
  color: #d1d5db;
  font-size: 14px;
}
.role {
  display: inline-block;
  margin-left: 4px;
  padding: 1px 8px;
  border-radius: 999px;
  background: #374151;
  color: #f9fafb;
  font-size: 11px;
  text-transform: capitalize;
}
.logout {
  background: transparent;
  border: 1px solid #4b5563;
  color: #d1d5db;
  padding: 6px 12px;
  border-radius: 8px;
  cursor: pointer;
  font-weight: 500;
}
.logout:hover {
  background: #374151;
  color: #fff;
}
.nav-link {
  color: #d1d5db;
  text-decoration: none;
  padding: 8px 14px;
  border-radius: 8px;
  font-weight: 500;
}
.nav-link:hover {
  background: #374151;
  color: #fff;
}
.nav-link.router-link-active {
  background: #f97316;
  color: #fff;
}
.content {
  max-width: 960px;
  margin: 0 auto;
  padding: 24px;
}
</style>
