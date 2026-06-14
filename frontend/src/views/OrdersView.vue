<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { menuApi, orderApi } from '../api/client'
import { statusOptions } from '../types'
import { formatCents } from '../money'
import { isStaff } from '../session'
import PaginationBar from '../components/PaginationBar.vue'
import type { MenuItem, Order, OrderItemInput, OrderStatus } from '../types'

const PAGE_SIZE = 20
const MENU_FETCH_LIMIT = 100 // enough to populate the item dropdown

const orders = ref<Order[]>([])
const total = ref(0)
const offset = ref(0)
const menu = ref<MenuItem[]>([])
const loading = ref(false)
const error = ref('')
const success = ref('')

interface DraftLine {
  menu_item_id: string
  quantity: number
}

const draft = reactive<{ customer_name: string; note: string; lines: DraftLine[] }>({
  customer_name: '',
  note: '',
  lines: [{ menu_item_id: '', quantity: 1 }],
})

const availableMenu = computed(() => menu.value.filter((m) => m.available))

// Live total preview (in cents) from the current draft and known menu prices.
const draftTotal = computed(() =>
  draft.lines.reduce((sum, line) => {
    const item = menu.value.find((m) => m.id === line.menu_item_id)
    return item ? sum + item.price * line.quantity : sum
  }, 0),
)

function addLine() {
  draft.lines.push({ menu_item_id: '', quantity: 1 })
}

function removeLine(index: number) {
  draft.lines.splice(index, 1)
  if (draft.lines.length === 0) addLine()
}

function resetDraft() {
  draft.customer_name = ''
  draft.note = ''
  draft.lines = [{ menu_item_id: '', quantity: 1 }]
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    // Everyone needs the menu to build an order; only staff may list all orders.
    const menuPage = await menuApi.list(MENU_FETCH_LIMIT, 0)
    menu.value = menuPage.items

    if (isStaff.value) {
      const orderPage = await orderApi.list(PAGE_SIZE, offset.value)
      orders.value = orderPage.items
      total.value = orderPage.pagination.total
      if (orders.value.length === 0 && offset.value > 0) {
        offset.value = Math.max(0, offset.value - PAGE_SIZE)
        await load()
      }
    }
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

function changePage(newOffset: number) {
  offset.value = newOffset
  load()
}

async function createOrder() {
  error.value = ''
  success.value = ''
  const items: OrderItemInput[] = draft.lines
    .filter((l) => l.menu_item_id && l.quantity > 0)
    .map((l) => ({ menu_item_id: l.menu_item_id, quantity: l.quantity }))

  if (!draft.customer_name.trim()) {
    error.value = 'Customer name is required.'
    return
  }
  if (items.length === 0) {
    error.value = 'Add at least one item.'
    return
  }

  try {
    await orderApi.create({ customer_name: draft.customer_name, note: draft.note, items })
    resetDraft()
    if (isStaff.value) {
      offset.value = 0 // newest order appears on the first page
      await load()
    } else {
      // Customers cannot list orders, so confirm the placement inline instead.
      success.value = 'Your order has been placed!'
    }
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function changeStatus(order: Order, status: OrderStatus) {
  if (status === order.status) return
  error.value = ''
  try {
    await orderApi.updateStatus(order.id, status, order.note)
    await load()
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function remove(order: Order) {
  if (!confirm(`Delete order for "${order.customer_name}"?`)) return
  error.value = ''
  try {
    await orderApi.remove(order.id)
    await load()
  } catch (e) {
    error.value = (e as Error).message
  }
}

function formatTime(iso: string): string {
  return new Date(iso).toLocaleString()
}

onMounted(load)
</script>

<template>
  <div>
    <p v-if="error" class="error">{{ error }}</p>
    <p v-if="success" class="success">{{ success }}</p>

    <section class="card">
      <h2 class="section-title">New Order</h2>
      <p v-if="availableMenu.length === 0" class="muted">
        No available menu items. Add some on the Menu page first.
      </p>
      <form v-else @submit.prevent="createOrder">
        <div class="field">
          <label>Customer Name</label>
          <input v-model="draft.customer_name" required maxlength="120" placeholder="e.g. Alice" />
        </div>

        <label class="field-label">Items</label>
        <div v-for="(line, i) in draft.lines" :key="i" class="line">
          <select v-model="line.menu_item_id">
            <option value="" disabled>Select an item…</option>
            <option v-for="m in availableMenu" :key="m.id" :value="m.id">
              {{ m.name }} — {{ formatCents(m.price) }}
            </option>
          </select>
          <input v-model.number="line.quantity" type="number" min="1" class="qty" />
          <button type="button" class="btn-danger" @click="removeLine(i)">✕</button>
        </div>
        <button type="button" class="btn-secondary add-line" @click="addLine">
          + Add item
        </button>

        <div class="field">
          <label>Note</label>
          <input v-model="draft.note" maxlength="500" placeholder="Optional (e.g. No cilantro)" />
        </div>

        <div class="order-footer">
          <span class="total">Estimated total: <strong>{{ formatCents(draftTotal) }}</strong></span>
          <button type="submit" class="btn-primary">Place Order</button>
        </div>
      </form>
    </section>

    <section v-if="isStaff" class="card">
      <h2 class="section-title">Orders ({{ total }})</h2>
      <p v-if="loading" class="muted">Loading…</p>
      <p v-else-if="orders.length === 0" class="muted">No orders yet.</p>
      <div v-else>
        <div v-for="order in orders" :key="order.id" class="order">
          <div class="order-head">
            <div>
              <strong>{{ order.customer_name }}</strong>
              <span class="muted"> · {{ formatTime(order.created_at) }}</span>
            </div>
            <span :class="['badge', `status-${order.status}`]">{{ order.status }}</span>
          </div>

          <ul class="items">
            <li v-for="(it, idx) in order.items" :key="idx">
              {{ it.quantity }} × {{ it.name }}
              <span class="muted">({{ formatCents(it.unit_price) }})</span>
              — {{ formatCents(it.subtotal) }}
            </li>
          </ul>

          <p v-if="order.note" class="muted note">Note: {{ order.note }}</p>

          <div class="order-controls">
            <strong>Total: {{ formatCents(order.total_amount) }}</strong>
            <div class="controls-right">
              <select
                :value="order.status"
                @change="changeStatus(order, ($event.target as HTMLSelectElement).value as OrderStatus)"
              >
                <option v-for="s in statusOptions(order.status)" :key="s" :value="s">{{ s }}</option>
              </select>
              <button class="btn-danger" @click="remove(order)">Delete</button>
            </div>
          </div>
        </div>
        <PaginationBar :limit="PAGE_SIZE" :offset="offset" :total="total" @change="changePage" />
      </div>
    </section>
  </div>
</template>

<style scoped>
.field-label {
  display: block;
  margin-bottom: 6px;
  font-size: 13px;
  font-weight: 600;
  color: #374151;
}
.line {
  display: flex;
  gap: 8px;
  margin-bottom: 8px;
}
.qty {
  width: 80px;
}
.add-line {
  margin-bottom: 16px;
}
.order-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 8px;
}
.total {
  font-size: 15px;
}
.order {
  border: 1px solid #f0f0f0;
  border-radius: 10px;
  padding: 14px;
  margin-bottom: 12px;
}
.order-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}
.items {
  margin: 8px 0;
  padding-left: 18px;
}
.items li {
  margin-bottom: 2px;
}
.note {
  margin: 4px 0 8px;
}
.order-controls {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-top: 1px solid #f0f0f0;
  padding-top: 10px;
}
.controls-right {
  display: flex;
  gap: 8px;
}
.controls-right select {
  width: auto;
}
.badge {
  font-size: 12px;
  padding: 3px 12px;
  border-radius: 999px;
  font-weight: 600;
  text-transform: capitalize;
}
.status-pending {
  background: #fef3c7;
  color: #92400e;
}
.status-preparing {
  background: #dbeafe;
  color: #1e40af;
}
.status-ready {
  background: #d1fae5;
  color: #065f46;
}
.status-completed {
  background: #e5e7eb;
  color: #374151;
}
.status-cancelled {
  background: #fee2e2;
  color: #b91c1c;
}
</style>
