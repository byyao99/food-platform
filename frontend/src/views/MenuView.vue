<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { menuApi } from '../api/client'
import { formatCents, fromCents, toCents } from '../money'
import PaginationBar from '../components/PaginationBar.vue'
import type { MenuItem, MenuItemInput } from '../types'

const PAGE_SIZE = 20

const items = ref<MenuItem[]>([])
const total = ref(0)
const offset = ref(0)
const loading = ref(false)
const error = ref('')

// editingId is null when the form is creating a new item.
const editingId = ref<string | null>(null)

// `priceDollars` is what the user types; it is converted to cents on submit.
const form = reactive({ name: '', description: '', priceDollars: 0, category: '', available: true })

function resetForm() {
  editingId.value = null
  Object.assign(form, { name: '', description: '', priceDollars: 0, category: '', available: true })
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const page = await menuApi.list(PAGE_SIZE, offset.value)
    items.value = page.items
    total.value = page.pagination.total
    // If a deletion emptied the current page, step back one page.
    if (items.value.length === 0 && offset.value > 0) {
      offset.value = Math.max(0, offset.value - PAGE_SIZE)
      await load()
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

function startEdit(item: MenuItem) {
  editingId.value = item.id
  form.name = item.name
  form.description = item.description
  form.priceDollars = fromCents(item.price)
  form.category = item.category
  form.available = item.available
}

function buildInput(): MenuItemInput {
  return {
    name: form.name,
    description: form.description,
    price: toCents(form.priceDollars),
    category: form.category,
    available: form.available,
  }
}

async function submit() {
  error.value = ''
  try {
    if (editingId.value) {
      await menuApi.update(editingId.value, buildInput())
    } else {
      await menuApi.create(buildInput())
    }
    resetForm()
    await load()
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function remove(item: MenuItem) {
  if (!confirm(`Delete "${item.name}"?`)) return
  error.value = ''
  try {
    await menuApi.remove(item.id)
    if (editingId.value === item.id) resetForm()
    await load()
  } catch (e) {
    error.value = (e as Error).message
  }
}

onMounted(load)
</script>

<template>
  <div>
    <p v-if="error" class="error">{{ error }}</p>

    <section class="card">
      <h2 class="section-title">{{ editingId ? 'Edit Menu Item' : 'New Menu Item' }}</h2>
      <form @submit.prevent="submit">
        <div class="grid">
          <div class="field">
            <label>Name</label>
            <input v-model="form.name" required maxlength="120" placeholder="e.g. Beef Noodles" />
          </div>
          <div class="field">
            <label>Category</label>
            <input v-model="form.category" required maxlength="60" placeholder="e.g. Main" />
          </div>
          <div class="field">
            <label>Price ($)</label>
            <input v-model.number="form.priceDollars" type="number" min="0" step="0.01" required />
          </div>
          <div class="field checkbox">
            <label>
              <input type="checkbox" v-model="form.available" />
              Available
            </label>
          </div>
        </div>
        <div class="field">
          <label>Description</label>
          <textarea v-model="form.description" rows="2" maxlength="500" placeholder="Optional"></textarea>
        </div>
        <div class="actions">
          <button type="submit" class="btn-primary">
            {{ editingId ? 'Save Changes' : 'Add Item' }}
          </button>
          <button v-if="editingId" type="button" class="btn-secondary" @click="resetForm">
            Cancel
          </button>
        </div>
      </form>
    </section>

    <section class="card">
      <h2 class="section-title">Menu ({{ total }})</h2>
      <p v-if="loading" class="muted">Loading…</p>
      <p v-else-if="items.length === 0" class="muted">No menu items yet.</p>
      <table v-else>
        <thead>
          <tr>
            <th>Name</th>
            <th>Category</th>
            <th>Price</th>
            <th>Status</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in items" :key="item.id">
            <td>
              <strong>{{ item.name }}</strong>
              <div class="muted" v-if="item.description">{{ item.description }}</div>
            </td>
            <td>{{ item.category }}</td>
            <td>{{ formatCents(item.price) }}</td>
            <td>
              <span :class="['badge', item.available ? 'badge-ok' : 'badge-off']">
                {{ item.available ? 'Available' : 'Unavailable' }}
              </span>
            </td>
            <td class="row-actions">
              <button class="btn-secondary" @click="startEdit(item)">Edit</button>
              <button class="btn-danger" @click="remove(item)">Delete</button>
            </td>
          </tr>
        </tbody>
      </table>
      <PaginationBar :limit="PAGE_SIZE" :offset="offset" :total="total" @change="changePage" />
    </section>
  </div>
</template>

<style scoped>
.grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}
.checkbox {
  display: flex;
  align-items: flex-end;
}
.checkbox label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 500;
}
.checkbox input {
  width: auto;
}
.actions {
  display: flex;
  gap: 8px;
  margin-top: 8px;
}
.row-actions {
  display: flex;
  gap: 8px;
}
.badge {
  font-size: 12px;
  padding: 3px 10px;
  border-radius: 999px;
  font-weight: 600;
}
.badge-ok {
  background: #dcfce7;
  color: #166534;
}
.badge-off {
  background: #f3f4f6;
  color: #6b7280;
}
</style>
