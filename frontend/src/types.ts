// Domain types mirroring the Go API models.
// All monetary fields (`price`, `unit_price`, `subtotal`, `total_amount`) are
// integer cents; use the helpers in `money.ts` to convert for display/input.

export interface MenuItem {
  id: string
  name: string
  description: string
  price: number // cents
  category: string
  available: boolean
  created_at: string
  updated_at: string
}

export interface MenuItemInput {
  name: string
  description: string
  price: number
  category: string
  available: boolean
}

export type OrderStatus =
  | 'pending'
  | 'preparing'
  | 'ready'
  | 'completed'
  | 'cancelled'

export const ORDER_STATUSES: OrderStatus[] = [
  'pending',
  'preparing',
  'ready',
  'completed',
  'cancelled',
]

// Allowed status transitions, mirroring the backend state machine.
// completed and cancelled are terminal.
const ORDER_STATUS_TRANSITIONS: Record<OrderStatus, OrderStatus[]> = {
  pending: ['preparing', 'cancelled'],
  preparing: ['ready', 'cancelled'],
  ready: ['completed', 'cancelled'],
  completed: [],
  cancelled: [],
}

// Status options to offer for an order: the current status (no-op) plus the
// legal next ones. Terminal statuses therefore offer only themselves.
export function statusOptions(current: OrderStatus): OrderStatus[] {
  return [current, ...ORDER_STATUS_TRANSITIONS[current]]
}

export interface OrderItem {
  menu_item_id: string
  name: string
  unit_price: number
  quantity: number
  subtotal: number
}

export interface Order {
  id: string
  customer_name: string
  items: OrderItem[]
  total_amount: number
  status: OrderStatus
  note: string
  created_at: string
  updated_at: string
}

export interface OrderItemInput {
  menu_item_id: string
  quantity: number
}

export interface CreateOrderInput {
  customer_name: string
  items: OrderItemInput[]
  note: string
}
