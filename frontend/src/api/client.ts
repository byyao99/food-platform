import type {
  AuthResponse,
  AuthUser,
  CreateOrderInput,
  MenuItem,
  MenuItemInput,
  Order,
  OrderStatus,
  Role,
} from '../types'
import { clearSession, getToken } from '../session'

const BASE = '/api/v1'

interface Envelope<T> {
  data: T
}

interface ErrorBody {
  error?: string
}

export interface PageMeta {
  limit: number
  offset: number
  total: number
}

export interface Page<T> {
  items: T[]
  pagination: PageMeta
}

interface PageResponse<T> {
  data: T[]
  pagination: PageMeta
}

async function parseError(res: Response): Promise<never> {
  const body = (await res.json().catch(() => null)) as ErrorBody | null
  throw new Error(body?.error ?? `Request failed (${res.status})`)
}

// authHeaders attaches the bearer token when the user is signed in.
function authHeaders(): Record<string, string> {
  const token = getToken()
  return token ? { Authorization: `Bearer ${token}` } : {}
}

// onUnauthorized drops a stale session so route guards can redirect to login.
function handleStatus(res: Response): void {
  if (res.status === 401) clearSession()
}

// request unwraps the `{ data }` envelope and throws on non-2xx responses.
async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...authHeaders(), ...init?.headers },
  })
  if (res.status === 204) return undefined as T
  if (!res.ok) {
    handleStatus(res)
    return parseError(res)
  }
  const body = (await res.json()) as Envelope<T>
  return body.data
}

// requestPage returns both the list and its pagination metadata.
async function requestPage<T>(path: string): Promise<Page<T>> {
  const res = await fetch(`${BASE}${path}`, { headers: authHeaders() })
  if (!res.ok) {
    handleStatus(res)
    return parseError(res)
  }
  const body = (await res.json()) as PageResponse<T>
  return { items: body.data, pagination: body.pagination }
}

function pageQuery(limit: number, offset: number): string {
  return `?limit=${limit}&offset=${offset}`
}

export const authApi = {
  login: (username: string, password: string) =>
    request<AuthResponse>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  register: (username: string, password: string) =>
    request<AuthResponse>('/auth/register', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  changePassword: (oldPassword: string, newPassword: string) =>
    request<void>('/auth/password', {
      method: 'PUT',
      body: JSON.stringify({ old_password: oldPassword, new_password: newPassword }),
    }),
}

export const userApi = {
  list: (limit = 20, offset = 0) =>
    requestPage<AuthUser>(`/users${pageQuery(limit, offset)}`),
  create: (input: { username: string; password: string; role: Role }) =>
    request<AuthUser>('/users', { method: 'POST', body: JSON.stringify(input) }),
  updateRole: (id: string, role: Role) =>
    request<AuthUser>(`/users/${id}/role`, {
      method: 'PUT',
      body: JSON.stringify({ role }),
    }),
  resetPassword: (id: string, password: string) =>
    request<void>(`/users/${id}/password`, {
      method: 'PUT',
      body: JSON.stringify({ password }),
    }),
  remove: (id: string) => request<void>(`/users/${id}`, { method: 'DELETE' }),
}

export interface MenuFilter {
  category?: string
  available?: boolean
}

export const menuApi = {
  list: (limit = 20, offset = 0, filter: MenuFilter = {}) => {
    let query = pageQuery(limit, offset)
    if (filter.category) query += `&category=${encodeURIComponent(filter.category)}`
    if (filter.available !== undefined) query += `&available=${filter.available}`
    return requestPage<MenuItem>(`/menu${query}`)
  },
  get: (id: string) => request<MenuItem>(`/menu/${id}`),
  create: (input: MenuItemInput) =>
    request<MenuItem>('/menu', { method: 'POST', body: JSON.stringify(input) }),
  update: (id: string, input: MenuItemInput) =>
    request<MenuItem>(`/menu/${id}`, { method: 'PUT', body: JSON.stringify(input) }),
  remove: (id: string) => request<void>(`/menu/${id}`, { method: 'DELETE' }),
}

export const orderApi = {
  list: (limit = 20, offset = 0) =>
    requestPage<Order>(`/orders${pageQuery(limit, offset)}`),
  get: (id: string) => request<Order>(`/orders/${id}`),
  create: (input: CreateOrderInput) =>
    request<Order>('/orders', { method: 'POST', body: JSON.stringify(input) }),
  updateStatus: (id: string, status: OrderStatus, note: string) =>
    request<Order>(`/orders/${id}`, {
      method: 'PUT',
      body: JSON.stringify({ status, note }),
    }),
  remove: (id: string) => request<void>(`/orders/${id}`, { method: 'DELETE' }),
}
