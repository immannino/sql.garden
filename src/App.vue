<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRegisterSW } from 'virtual:pwa-register/vue'
import Canvas from './components/Canvas.vue'
import QueryPanel from './components/QueryPanel.vue'
import ImportModal from './components/ImportModal.vue'
import { useDuckDB } from './composables/useDuckDB'
import { useSchemaStore } from './stores/schema'
import { usePersistence } from './composables/usePersistence'
import { useAppReady } from './composables/useAppReady'

const { init, isReady, isLoading, initError, exec } = useDuckDB()
const schemaStore = useSchemaStore()
const { loadAll, startAutoSave } = usePersistence()
const { markAppReady } = useAppReady()

function addQueryNode() {
  const center = canvasRef.value?.getCenter() ?? { x: 200, y: 200 }
  const n = schemaStore.nodes.filter((n) => n.kind === 'query').length + 1
  schemaStore.addQueryNode({
    id: `query_${Date.now()}`,
    name: `query_${n}`,
    x: center.x - 140,
    y: center.y - 80,
    sql: 'SELECT\n  *\nFROM users\nLIMIT 100',
  })
}

function addChartNode() {
  const center = canvasRef.value?.getCenter() ?? { x: 200, y: 200 }
  const n = schemaStore.nodes.filter((n) => n.kind === 'chart').length + 1
  schemaStore.addChartNode({
    id: `chart_${Date.now()}`,
    name: `chart_${n}`,
    x: center.x - 170,
    y: center.y - 120,
    sourceId: null,
    sql: '',
    chartType: 'barY',
    xColumn: '',
    yColumn: '',
  })
}

// PWA update toast
const { needRefresh, updateServiceWorker } = useRegisterSW()

// PWA install prompt
const installPrompt = ref<Event | null>(null)
const showInstall = ref(false)
window.addEventListener('beforeinstallprompt', (e) => {
  e.preventDefault()
  installPrompt.value = e
  showInstall.value = true
})

async function installPWA() {
  if (!installPrompt.value) return
  ;(installPrompt.value as Event & { prompt(): Promise<void> }).prompt()
  showInstall.value = false
  installPrompt.value = null
}

function dismissUpdate() {
  needRefresh.value = false
}

const canvasRef = ref<InstanceType<typeof Canvas> | null>(null)
const queryPanelRef = ref<InstanceType<typeof QueryPanel> | null>(null)
const showQuery = ref(true)
const showImport = ref(false)

// Sample e-commerce schema seeded into DuckDB at startup
const SEED_SQL = `
CREATE TABLE users (
  id        INTEGER PRIMARY KEY,
  name      VARCHAR NOT NULL,
  email     VARCHAR UNIQUE NOT NULL,
  created_at TIMESTAMP
);
INSERT INTO users VALUES
  (1, 'Alice Chen',    'alice@example.com',   '2024-01-15 09:00:00'),
  (2, 'Bob Smith',     'bob@example.com',     '2024-02-20 14:30:00'),
  (3, 'Carol Davis',   'carol@example.com',   '2024-03-05 11:15:00'),
  (4, 'Dave Wilson',   'dave@example.com',    '2024-04-10 16:45:00'),
  (5, 'Eve Martinez',  'eve@example.com',     '2024-05-22 08:20:00');

CREATE TABLE products (
  id    INTEGER PRIMARY KEY,
  name  VARCHAR NOT NULL,
  price DECIMAL(10,2) NOT NULL,
  stock INTEGER DEFAULT 0
);
INSERT INTO products VALUES
  (1, 'Ergonomic Chair',  349.99, 42),
  (2, 'Standing Desk',    599.00, 18),
  (3, 'Monitor 27"',      449.95, 31),
  (4, 'Mechanical Keyboard', 129.99, 85),
  (5, 'USB-C Hub',         49.99, 120);

CREATE TABLE orders (
  id         INTEGER PRIMARY KEY,
  user_id    INTEGER REFERENCES users(id),
  status     VARCHAR DEFAULT 'pending',
  total      DECIMAL(10,2),
  created_at TIMESTAMP
);
INSERT INTO orders VALUES
  (1, 1, 'completed', 999.94, '2024-06-01 10:00:00'),
  (2, 2, 'completed', 349.99, '2024-06-03 14:00:00'),
  (3, 1, 'shipped',   449.95, '2024-06-10 09:30:00'),
  (4, 3, 'pending',   179.98, '2024-06-12 16:00:00'),
  (5, 4, 'completed', 649.98, '2024-06-14 11:00:00');

CREATE TABLE order_items (
  id         INTEGER PRIMARY KEY,
  order_id   INTEGER REFERENCES orders(id),
  product_id INTEGER REFERENCES products(id),
  quantity   INTEGER NOT NULL,
  unit_price DECIMAL(10,2) NOT NULL
);
INSERT INTO order_items VALUES
  (1, 1, 2, 1, 599.00),
  (2, 1, 4, 1, 129.99),
  (3, 1, 5, 2, 49.99),
  (4, 1, 5, 4, 49.99),
  (5, 2, 1, 1, 349.99),
  (6, 3, 3, 1, 449.95),
  (7, 4, 4, 1, 129.99),
  (8, 4, 5, 1, 49.99),
  (9, 5, 1, 1, 349.99),
  (10,5, 3, 1, 449.95);
`

const SAMPLE_SCHEMA = [
  {
    id: 'users',
    name: 'users',
    x: 60,
    y: 80,
    columns: [
      { name: 'id',         type: 'INTEGER',   primaryKey: true  },
      { name: 'name',       type: 'VARCHAR',   nullable: false   },
      { name: 'email',      type: 'VARCHAR',   nullable: false   },
      { name: 'created_at', type: 'TIMESTAMP', nullable: true    },
    ],
  },
  {
    id: 'products',
    name: 'products',
    x: 360,
    y: 320,
    columns: [
      { name: 'id',    type: 'INTEGER',       primaryKey: true },
      { name: 'name',  type: 'VARCHAR',       nullable: false  },
      { name: 'price', type: 'DECIMAL(10,2)', nullable: false  },
      { name: 'stock', type: 'INTEGER',       nullable: true   },
    ],
  },
  {
    id: 'orders',
    name: 'orders',
    x: 360,
    y: 60,
    columns: [
      { name: 'id',         type: 'INTEGER',       primaryKey: true  },
      { name: 'user_id',    type: 'INTEGER',       references: { table: 'users', column: 'id' } },
      { name: 'status',     type: 'VARCHAR',       nullable: true    },
      { name: 'total',      type: 'DECIMAL(10,2)', nullable: true    },
      { name: 'created_at', type: 'TIMESTAMP',     nullable: true    },
    ],
  },
  {
    id: 'order_items',
    name: 'order_items',
    x: 660,
    y: 180,
    columns: [
      { name: 'id',         type: 'INTEGER',       primaryKey: true  },
      { name: 'order_id',   type: 'INTEGER',       references: { table: 'orders',   column: 'id' } },
      { name: 'product_id', type: 'INTEGER',       references: { table: 'products', column: 'id' } },
      { name: 'quantity',   type: 'INTEGER',       nullable: false   },
      { name: 'unit_price', type: 'DECIMAL(10,2)', nullable: false   },
    ],
  },
]

onMounted(async () => {
  try {
    await init()
    const restored = await loadAll()
    if (!restored) {
      await exec(SEED_SQL)
      for (const table of SAMPLE_SCHEMA) {
        schemaStore.addTable(table)
      }
    }
    await queryPanelRef.value?.refreshStats()
    startAutoSave()
    markAppReady()
  } catch {
    // initError already set by useDuckDB
  }
})
</script>

<template>
  <div class="app-shell">
    <!-- Toolbar -->
    <header class="toolbar">
      <div class="toolbar-left">
        <span class="logo">
          🌱 sql.garden
        </span>
      </div>

      <div class="toolbar-center">
        <div class="db-status" :class="{ ready: isReady, loading: isLoading, error: !!initError }">
          <span class="status-dot" />
          <span class="status-text">
            {{ initError ? 'DuckDB error' : isLoading ? 'Initializing DuckDB…' : 'DuckDB ready' }}
          </span>
        </div>
      </div>

      <div class="toolbar-right">
        <button class="toolbar-btn" :disabled="!isReady" title="Add a query node to the canvas" @click="addQueryNode">
          <svg viewBox="0 0 16 16" fill="none">
            <polyline points="2,5 6,9 2,13" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
            <line x1="8" y1="4" x2="14" y2="4" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
            <line x1="8" y1="8" x2="14" y2="8" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
            <line x1="8" y1="12" x2="14" y2="12" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
          </svg>
          Query
        </button>

        <button class="toolbar-btn" :disabled="!isReady" title="Add a chart node to the canvas" @click="addChartNode">
          <svg viewBox="0 0 16 16" fill="none">
            <rect x="1" y="1" width="14" height="14" rx="2" stroke="currentColor" stroke-width="1.3"/>
            <polyline points="3,11 6,6 9,9 13,4" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
          Chart
        </button>

        <div class="toolbar-divider" />

        <button
          v-if="showInstall"
          class="toolbar-btn install-btn"
          @click="installPWA"
        >
          <svg viewBox="0 0 16 16" fill="none">
            <path d="M8 2v8M5 7l3 3 3-3" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
            <path d="M3 11v1a1 1 0 001 1h8a1 1 0 001-1v-1" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
          </svg>
          Install app
        </button>

        <button
          class="toolbar-btn"
          :disabled="!isReady"
          @click="showImport = true"
        >
          <svg viewBox="0 0 16 16" fill="none">
            <path d="M8 2v8M5 7l3 3 3-3" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
            <path d="M3 11v1a1 1 0 001 1h8a1 1 0 001-1v-1" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
          </svg>
          Import CSV
        </button>

        <div class="toolbar-divider" />

        <button
          class="toolbar-btn"
          title="Fit all tables in view"
          @click="canvasRef?.fitView()"
        >
          <svg viewBox="0 0 16 16" fill="none">
            <rect x="1" y="1" width="5" height="5" rx="1" stroke="currentColor" stroke-width="1.3"/>
            <rect x="10" y="1" width="5" height="5" rx="1" stroke="currentColor" stroke-width="1.3"/>
            <rect x="1" y="10" width="5" height="5" rx="1" stroke="currentColor" stroke-width="1.3"/>
            <rect x="10" y="10" width="5" height="5" rx="1" stroke="currentColor" stroke-width="1.3"/>
          </svg>
          Fit view
        </button>

        <button
          class="toolbar-btn"
          :class="{ active: showQuery }"
          @click="showQuery = !showQuery"
        >
          <svg viewBox="0 0 16 16" fill="none">
            <polyline points="2,5 7,10 2,15" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round" :transform="showQuery ? 'rotate(90 8 10)' : ''"/>
            <line x1="8" y1="12" x2="14" y2="12" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
            <line x1="8" y1="8" x2="14" y2="8" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
            <line x1="8" y1="4" x2="14" y2="4" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
          </svg>
          Query
        </button>
      </div>
    </header>

    <!-- Main content -->
    <div class="main-area">
      <Canvas ref="canvasRef" />
      <QueryPanel v-if="showQuery" ref="queryPanelRef" @close="showQuery = false" />
    </div>

    <!-- Import modal -->
    <ImportModal v-if="showImport" @close="showImport = false" />

    <!-- PWA update toast -->
    <div v-if="needRefresh" class="pwa-toast">
      <span>A new version is available.</span>
      <button class="pwa-toast-btn" @click="updateServiceWorker()">Update</button>
      <button class="pwa-toast-dismiss" @click="dismissUpdate">✕</button>
    </div>

    <!-- Init error overlay -->
    <div v-if="initError" class="error-overlay">
      <div class="error-card">
        <h2>DuckDB failed to load</h2>
        <pre>{{ initError }}</pre>
        <p class="error-hint">
          This app requires <a href="https://webassembly.org/" target="_blank">WebAssembly</a> support.
          Make sure you're using a modern browser and the dev server is running with the correct headers.
        </p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.app-shell {
  height: 100vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.toolbar {
  height: 44px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  padding: 0 12px;
  background: var(--surface-1);
  border-bottom: 1px solid var(--border);
  gap: 12px;
  z-index: 10;
}

.toolbar-left,
.toolbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
}

.toolbar-right {
  justify-content: flex-end;
}

.toolbar-center {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: 1;
}

.logo {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 13.5px;
  font-weight: 600;
  color: var(--text-primary);
  letter-spacing: -0.01em;
}

.logo svg {
  width: 18px;
  height: 18px;
}

.db-status {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11.5px;
  color: var(--text-muted);
}

.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--text-muted);
  transition: background 0.3s;
}

.db-status.loading .status-dot {
  background: var(--warning);
  animation: pulse 1s ease-in-out infinite;
}

.db-status.ready .status-dot {
  background: var(--success);
}

.db-status.ready .status-text {
  color: var(--success);
}

.db-status.error .status-dot {
  background: var(--error);
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.3; }
}

.toolbar-btn {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 4px 10px;
  font-size: 12px;
  color: var(--text-secondary);
  background: transparent;
  border: 1px solid transparent;
  border-radius: 5px;
  transition: color 0.15s, border-color 0.15s, background 0.15s;
}

.toolbar-btn svg {
  width: 14px;
  height: 14px;
  flex-shrink: 0;
}

.toolbar-btn:hover {
  color: var(--text-primary);
  background: var(--surface-2);
  border-color: var(--border);
}

.toolbar-divider {
  width: 1px;
  height: 18px;
  background: var(--border);
  flex-shrink: 0;
}

.toolbar-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.toolbar-btn.active {
  color: var(--accent);
  background: rgba(88, 166, 255, 0.08);
  border-color: rgba(88, 166, 255, 0.25);
}

.main-area {
  flex: 1;
  display: flex;
  overflow: hidden;
}

.error-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.7);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}

.error-card {
  background: var(--surface-1);
  border: 1px solid var(--error);
  border-radius: 10px;
  padding: 28px 32px;
  max-width: 480px;
  width: 90%;
}

.error-card h2 {
  font-size: 16px;
  color: var(--error);
  margin-bottom: 12px;
}

.error-card pre {
  font-family: var(--font-mono);
  font-size: 11.5px;
  color: var(--text-secondary);
  white-space: pre-wrap;
  margin-bottom: 16px;
}

.error-hint {
  font-size: 13px;
  color: var(--text-muted);
  line-height: 1.5;
}

.error-hint a {
  color: var(--accent);
}

.install-btn {
  color: var(--accent);
  border-color: rgba(88, 166, 255, 0.25);
}

.pwa-toast {
  position: fixed;
  bottom: 20px;
  right: 20px;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  background: var(--surface-1);
  border: 1px solid var(--border);
  border-radius: 8px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
  font-size: 12.5px;
  color: var(--text-secondary);
  z-index: 300;
}

.pwa-toast-btn {
  padding: 4px 10px;
  font-size: 12px;
  font-weight: 600;
  background: var(--accent);
  color: #0d1117;
  border: none;
  border-radius: 4px;
  cursor: pointer;
}

.pwa-toast-btn:hover {
  background: var(--accent-hover);
}

.pwa-toast-dismiss {
  background: transparent;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 11px;
  padding: 2px 4px;
}

.pwa-toast-dismiss:hover {
  color: var(--text-primary);
}
</style>
