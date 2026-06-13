# Food Platform — Frontend

A Vue 3 + TypeScript single-page app (Vite + vue-router) for managing the menu and orders served by the Go backend.

## Prerequisites

Start the Go backend first (from the repo root):

```bash
go run .          # serves the API on http://localhost:8080
```

## Setup & run

```bash
cd frontend
npm install
npm run dev       # http://localhost:5173
```

The dev server proxies `/api/*` to `http://localhost:8080`, so no extra config is needed.

## Scripts

| Command            | Description                       |
|--------------------|-----------------------------------|
| `npm run dev`      | Start the Vite dev server         |
| `npm run build`    | Type-check and build for production |
| `npm run preview`  | Preview the production build       |
| `npm run type-check` | Run `vue-tsc` type checking      |

## Structure

```
frontend/src/
├── main.ts              # app bootstrap
├── App.vue              # layout + nav
├── router/index.ts      # routes (/menu, /orders)
├── api/client.ts        # typed fetch wrapper for the REST API
├── types.ts             # shared domain types
└── views/
    ├── MenuView.vue     # menu CRUD
    └── OrdersView.vue   # create orders, update status, delete
```

## Pages

- **Menu** — create, edit, delete menu items and toggle availability.
- **Orders** — build an order from available menu items (with a live total preview), place it, then update its status through the lifecycle (`pending → preparing → ready → completed`, or `cancelled`).
