<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { IS_DESKTOP } from '../lib/env'
import type { main } from '../../wailsjs/go/models'

const emit = defineEmits<{
  close: []
  load: [id: string]
}>()

const datasets = ref<main.SampleDataset[]>([])
const loading = ref<string | null>(null)

onMounted(async () => {
  try {
    if (IS_DESKTOP) {
      const { ListSampleDatasets } = await import('../../wailsjs/go/main/App')
      datasets.value = await ListSampleDatasets()
    } else {
      const { WEB_DATASETS } = await import('../lib/webSampleData')
      datasets.value = WEB_DATASETS as unknown as main.SampleDataset[]
    }
  } catch { /* ignore */ }
  window.addEventListener('keydown', onKey)
})
onUnmounted(() => window.removeEventListener('keydown', onKey))

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') emit('close')
}

function onBackdrop(e: MouseEvent) {
  if ((e.target as HTMLElement).classList.contains('modal-backdrop')) emit('close')
}

async function select(id: string) {
  loading.value = id
  emit('load', id)
}
</script>

<template>
  <div class="modal-backdrop" @mousedown="onBackdrop">
    <div class="modal" role="dialog" aria-label="Sample Datasets">
      <div class="modal-header">
        <div class="modal-title-group">
          <h2 class="modal-title">Sample Datasets</h2>
          <p class="modal-subtitle">Load a pre-built dataset to explore sql.garden's features</p>
        </div>
        <button class="close-btn" title="Close" @click="emit('close')">
          <svg viewBox="0 0 16 16" fill="none">
            <line x1="3" y1="3" x2="13" y2="13" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
            <line x1="13" y1="3" x2="3" y2="13" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
          </svg>
        </button>
      </div>

      <div class="dataset-grid">
        <div
          v-for="ds in datasets"
          :key="ds.id"
          class="dataset-card"
          :class="{ loading: loading === ds.id }"
        >
          <div class="card-icon">{{ ds.icon }}</div>
          <div class="card-body">
            <div class="card-name">{{ ds.name }}</div>
            <div class="card-desc">{{ ds.description }}</div>
            <div class="card-meta">
              <span class="card-tables">{{ ds.tables.length }} tables</span>
              <span class="card-dot">·</span>
              <span class="card-rows">{{ ds.rowCount.toLocaleString() }} rows</span>
            </div>
            <div class="card-tags">
              <span v-for="t in ds.tables" :key="t" class="tag">{{ t }}</span>
            </div>
          </div>
          <button
            class="load-btn"
            :disabled="loading !== null"
            @click="select(ds.id)"
          >
            <span v-if="loading === ds.id" class="spinner" />
            <span v-else>Load</span>
          </button>
        </div>
      </div>

      <div class="modal-footer">
        <p class="footer-note">
          Loading a dataset will clear your current canvas. Your attached databases remain connected.
        </p>
        <button class="skip-btn" @click="emit('close')">Start with blank canvas</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal {
  width: 660px;
  max-height: 80vh;
  background: var(--surface-1);
  border: 1px solid var(--border);
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: 0 32px 80px rgba(0, 0, 0, 0.5);
}

.modal-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  padding: 20px 22px 16px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.modal-title-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.modal-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
}

.modal-subtitle {
  font-size: 12px;
  color: var(--text-muted);
}

.close-btn {
  width: 26px;
  height: 26px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  background: transparent;
  border: none;
  border-radius: 5px;
  flex-shrink: 0;
  margin-top: 2px;
  transition: color 0.15s, background 0.15s;
}
.close-btn:hover { color: var(--text-primary); background: var(--surface-2); }
.close-btn svg { width: 14px; height: 14px; }

/* Dataset grid */
.dataset-grid {
  overflow-y: auto;
  padding: 16px 22px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.dataset-card {
  display: flex;
  align-items: flex-start;
  gap: 14px;
  padding: 14px 16px;
  background: var(--surface-0);
  border: 1px solid var(--border);
  border-radius: 8px;
  transition: border-color 0.15s, background 0.15s;
}

.dataset-card:hover {
  border-color: var(--accent);
  background: var(--surface-1);
}

.dataset-card.loading {
  opacity: 0.7;
}

.card-icon {
  font-size: 26px;
  line-height: 1;
  flex-shrink: 0;
  margin-top: 2px;
}

.card-body {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.card-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
}

.card-desc {
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.5;
}

.card-meta {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
  color: var(--text-muted);
}

.card-dot { opacity: 0.5; }

.card-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 2px;
}

.tag {
  font-family: var(--font-mono);
  font-size: 10.5px;
  color: var(--text-secondary);
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 3px;
  padding: 1px 5px;
}

.load-btn {
  flex-shrink: 0;
  padding: 6px 16px;
  font-size: 12px;
  font-weight: 500;
  color: #fff;
  background: var(--accent);
  border: none;
  border-radius: 6px;
  transition: background 0.15s, opacity 0.15s;
  min-width: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
  align-self: center;
}
.load-btn:hover:not(:disabled) { background: var(--accent-hover); }
.load-btn:disabled { opacity: 0.5; cursor: not-allowed; }

.spinner {
  width: 12px;
  height: 12px;
  border: 2px solid rgba(255,255,255,0.3);
  border-top-color: #fff;
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }

/* Footer */
.modal-footer {
  border-top: 1px solid var(--border);
  padding: 14px 22px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-shrink: 0;
  gap: 16px;
}

.footer-note {
  font-size: 11.5px;
  color: var(--text-muted);
  line-height: 1.5;
  flex: 1;
}

.skip-btn {
  flex-shrink: 0;
  padding: 6px 14px;
  font-size: 12px;
  color: var(--text-secondary);
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 6px;
  transition: color 0.15s, background 0.15s;
  white-space: nowrap;
}
.skip-btn:hover { color: var(--text-primary); background: var(--surface-3); }
</style>
