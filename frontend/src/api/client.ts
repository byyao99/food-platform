import type {
  CreateOrderInput,
  MenuItem,
  MenuItemInput,
  Order,
  OrderStatus,
} from '../types'

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

// request unwraps the `{ data }` envelope and throws on non-2xx responses.
async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  })
  if (res.status === 204) return undefined as T
  if (!res.ok) return parseError(res)
  const body = (await res.json()) as Envelope<T>
  return body.data
}

// requestPage returns both the list and its pagination metadata.
async function requestPage<T>(path: string): Promise<Page<T>> {
  const res = await fetch(`${BASE}${path}`)
  if (!res.ok) return parseError(res)
  const body = (await res.json()) as PageResponse<T>
  return { items: body.data, pagination: body.pagination }
}

function pageQuery(limit: number, offset: number): string {
  return `?limit=${limit}&offset=${offset}`
}

export const menuApi = {
  list: (limit = 20, offset = 0) =>
    requestPage<MenuItem>(`/menu${pageQuery(limit, offset)}`),
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
