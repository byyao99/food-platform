<script setup lang="ts">
import { RouterLink, RouterView } from 'vue-router'
</script>

<template>
  <div class="app">
    <header class="topbar">
      <h1 class="brand">🍜 Food Platform</h1>
      <nav class="nav">
        <RouterLink to="/menu" class="nav-link">Menu</RouterLink>
        <RouterLink to="/orders" class="nav-link">Orders</RouterLink>
      </nav>
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
