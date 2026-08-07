<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { IS_DESKTOP } from '../lib/env'
import type { main } from '../../wailsjs/go/models'

const emit = defineEmits<{
  close: []
  load: [id: string]
  loadLearn: [id: string]
}>()

type Section = 'builtin' | 'learn' | 'community'
const activeSection = ref<Section>('builtin')

const datasets = ref<main.SampleDataset[]>([])
const learnTracks = ref<main.LearnTrack[]>([])
const loading = ref<string | null>(null)

onMounted(async () => {
  try {
    if (IS_DESKTOP) {
      const { ListSampleDatasets, ListLearnTracks } = await import('../../wailsjs/go/main/App')
      datasets.value = await ListSampleDatasets()
      learnTracks.value = await ListLearnTracks()
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
async function selectLearn(id: string) {
  loading.value = id
  emit('loadLearn', id)
}
</script>

<template>
  <div class="modal-backdrop" @mousedown="onBackdrop">
    <div class="modal" role="dialog" aria-label="Samples">
      <!-- Header -->
      <div class="modal-header">
        <div class="modal-title-group">
          <h2 class="modal-title">Open Sample</h2>
          <p class="modal-subtitle">Load a dataset or learning canvas to get started</p>
        </div>
        <button class="close-btn" title="Close" @click="emit('close')">
          <svg viewBox="0 0 16 16" fill="none">
            <line x1="3" y1="3" x2="13" y2="13" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
            <line x1="13" y1="3" x2="3" y2="13" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
          </svg>
        </button>
      </div>

      <!-- Section tabs -->
      <div class="section-tabs">
        <button
          v-for="s in (['builtin', 'learn', 'community'] as Section[])"
          :key="s"
          class="section-tab"
          :class="{ active: activeSection === s }"
          @click="activeSection = s"
        >
          <template v-if="s === 'builtin'">
            <svg viewBox="0 0 14 14" fill="none"><rect x="1" y="1" width="12" height="12" rx="1.5" stroke="currentColor" stroke-width="1.2"/><line x1="1" y1="4.5" x2="13" y2="4.5" stroke="currentColor" stroke-width="1.2"/><line x1="4.5" y1="4.5" x2="4.5" y2="13" stroke="currentColor" stroke-width="1.2"/></svg>
            Built-in
          </template>
          <template v-else-if="s === 'learn'">
            <svg viewBox="0 0 14 14" fill="none"><path d="M7 1L1 4.5v1l6 3.5 6-3.5v-1L7 1z" stroke="currentColor" stroke-width="1.2" stroke-linejoin="round"/><path d="M1 8l6 3.5L13 8" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/></svg>
            Learn SQL
          </template>
          <template v-else>
            <svg viewBox="0 0 14 14" fill="none"><circle cx="7" cy="7" r="5.5" stroke="currentColor" stroke-width="1.2"/><path d="M7 4v3.5l2 1.5" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/></svg>
            Community
            <span class="soon-chip">Soon</span>
          </template>
        </button>
      </div>

      <!-- Built-in datasets -->
      <div v-if="activeSection === 'builtin'" class="dataset-grid">
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
          <button class="load-btn" :disabled="loading !== null" @click="select(ds.id)">
            <span v-if="loading === ds.id" class="spinner" />
            <span v-else>Load</span>
          </button>
        </div>
      </div>

      <!-- Learn SQL tracks -->
      <div v-else-if="activeSection === 'learn'" class="dataset-grid">
        <div v-if="!IS_DESKTOP" class="learn-notice">
          <svg viewBox="0 0 16 16" fill="none"><circle cx="8" cy="8" r="6.5" stroke="currentColor" stroke-width="1.3"/><line x1="8" y1="5" x2="8" y2="8.5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/><circle cx="8" cy="11" r="0.7" fill="currentColor"/></svg>
          Learning tracks require the desktop app — data is generated locally in DuckDB.
        </div>
        <template v-else>
          <div
            v-for="lt in learnTracks"
            :key="lt.id"
            class="dataset-card learn-card"
            :class="{ loading: loading === lt.id }"
          >
            <div class="card-icon">{{ lt.icon }}</div>
            <div class="card-body">
              <div class="card-name">
                {{ lt.name }}
                <span class="level-badge" :class="`level-${lt.level.toLowerCase()}`">{{ lt.level }}</span>
              </div>
              <div class="card-desc">{{ lt.description }}</div>
              <div class="card-meta">
                <span>{{ lt.chapters }} chapters</span>
                <span class="card-dot">·</span>
                <span>{{ lt.tables.length }} tables</span>
                <span class="card-dot">·</span>
                <span>{{ lt.rowCount.toLocaleString() }} rows</span>
              </div>
              <div class="card-tags">
                <span v-for="tag in lt.tags" :key="tag" class="tag">{{ tag }}</span>
              </div>
            </div>
            <button class="load-btn" :disabled="loading !== null" @click="selectLearn(lt.id)">
              <span v-if="loading === lt.id" class="spinner" />
              <span v-else>Open</span>
            </button>
          </div>
          <div v-if="learnTracks.length === 0" class="empty-section">
            No learning tracks available.
          </div>
        </template>
      </div>

      <!-- Community (coming soon) -->
      <div v-else class="coming-soon">
        <svg viewBox="0 0 48 48" fill="none">
          <circle cx="24" cy="24" r="20" stroke="currentColor" stroke-width="1.5" opacity="0.3"/>
          <path d="M24 8C15.2 8 8 15.2 8 24s7.2 16 16 16 16-7.2 16-16" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
          <path d="M32 16l4-4m0 0l-4-4m4 4H24" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
        <h3>Community packs coming soon</h3>
        <p>
          Share canvases as <code>.sql.garden.json</code> files and host them anywhere.
          Open packs directly with a <code>sqlgarden://import?pack=&lt;url&gt;</code> link
          or paste a URL into the import dialog.
        </p>
      </div>

      <div class="modal-footer">
        <p class="footer-note">
          Loading a sample will clear your current canvas. Attached databases stay connected.
        </p>
        <button class="skip-btn" @click="emit('close')">Start blank</button>
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
  width: 680px;
  max-height: 82vh;
  background: var(--surface-1);
  border: 1px solid var(--border);
  border-radius: 8px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: 0 32px 80px rgba(0, 0, 0, 0.5);
}

/* Header */
.modal-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  padding: 20px 22px 16px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}
.modal-title-group { display: flex; flex-direction: column; gap: 3px; }
.modal-title { font-size: 15px; font-weight: 600; color: var(--text-primary); margin: 0; }
.modal-subtitle { font-size: 12px; color: var(--text-muted); margin: 0; }
.close-btn {
  width: 26px; height: 26px;
  display: flex; align-items: center; justify-content: center;
  color: var(--text-muted); background: transparent; border: none; border-radius: 5px;
  flex-shrink: 0; margin-top: 2px; transition: color 0.15s, background 0.15s;
}
.close-btn:hover { color: var(--text-primary); background: var(--surface-2); }
.close-btn svg { width: 14px; height: 14px; }

/* Section tabs */
.section-tabs {
  display: flex;
  border-bottom: 1px solid var(--border);
  background: var(--surface-0);
  flex-shrink: 0;
}
.section-tab {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 9px 18px;
  font-size: 12px;
  font-weight: 500;
  color: var(--text-secondary);
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  cursor: pointer;
  transition: color 0.1s, border-color 0.1s;
  white-space: nowrap;
}
.section-tab svg { width: 13px; height: 13px; flex-shrink: 0; }
.section-tab:hover:not(.active) { color: var(--text-primary); }
.section-tab.active { color: var(--accent); border-bottom-color: var(--accent); }
.soon-chip {
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.04em;
  color: var(--text-muted);
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 3px;
  padding: 1px 4px;
}

/* Dataset grid */
.dataset-grid {
  overflow-y: auto;
  padding: 14px 22px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  flex: 1;
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
.dataset-card:hover { border-color: var(--accent); background: var(--surface-1); }
.dataset-card.loading { opacity: 0.7; }

.card-icon { font-size: 26px; line-height: 1; flex-shrink: 0; margin-top: 2px; }
.card-body { flex: 1; display: flex; flex-direction: column; gap: 4px; min-width: 0; }
.card-name {
  font-size: 13px; font-weight: 600; color: var(--text-primary);
  display: flex; align-items: center; gap: 7px;
}
.card-desc { font-size: 12px; color: var(--text-secondary); line-height: 1.5; }
.card-meta { display: flex; align-items: center; gap: 5px; font-size: 11px; color: var(--text-muted); }
.card-dot { opacity: 0.5; }
.card-tags { display: flex; flex-wrap: wrap; gap: 4px; margin-top: 2px; }
.tag {
  font-family: var(--font-mono); font-size: 10.5px;
  color: var(--text-secondary); background: var(--surface-2);
  border: 1px solid var(--border); border-radius: 3px; padding: 1px 5px;
}

/* Level badge */
.level-badge {
  font-size: 9px; font-weight: 700; letter-spacing: 0.05em;
  border-radius: 3px; padding: 1px 5px; flex-shrink: 0;
}
.level-beginner { background: rgba(16,185,129,0.15); color: #34d399; }
.level-intermediate { background: rgba(245,158,11,0.15); color: #f59e0b; }
.level-advanced { background: rgba(239,68,68,0.15); color: #f87171; }

/* Load button */
.load-btn {
  flex-shrink: 0; padding: 6px 16px; font-size: 12px; font-weight: 500;
  color: #fff; background: var(--accent); border: none; border-radius: 6px;
  transition: opacity 0.15s; min-width: 60px;
  display: flex; align-items: center; justify-content: center; align-self: center;
}
.load-btn:hover:not(:disabled) { opacity: 0.85; }
.load-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.spinner {
  width: 12px; height: 12px;
  border: 2px solid rgba(255,255,255,0.3); border-top-color: #fff;
  border-radius: 50%; animation: spin 0.7s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }

/* Learn notices */
.learn-notice {
  display: flex; align-items: flex-start; gap: 10px;
  padding: 14px 16px; background: var(--surface-0); border: 1px solid var(--border);
  border-radius: 8px; font-size: 12px; color: var(--text-secondary); line-height: 1.5;
}
.learn-notice svg { width: 16px; height: 16px; flex-shrink: 0; margin-top: 1px; color: var(--text-muted); }
.empty-section { padding: 32px; text-align: center; color: var(--text-muted); font-size: 13px; }

/* Community coming soon */
.coming-soon {
  flex: 1; display: flex; flex-direction: column; align-items: center;
  justify-content: center; gap: 12px; padding: 40px 40px;
  text-align: center; color: var(--text-muted);
}
.coming-soon svg { width: 48px; height: 48px; opacity: 0.4; }
.coming-soon h3 { font-size: 14px; font-weight: 600; color: var(--text-secondary); margin: 0; }
.coming-soon p { font-size: 12.5px; line-height: 1.6; max-width: 360px; margin: 0; }
.coming-soon code {
  font-family: var(--font-mono); font-size: 11px;
  background: var(--surface-2); border: 1px solid var(--border);
  border-radius: 3px; padding: 1px 4px;
}

/* Footer */
.modal-footer {
  border-top: 1px solid var(--border); padding: 14px 22px;
  display: flex; align-items: center; justify-content: space-between;
  flex-shrink: 0; gap: 16px;
}
.footer-note { font-size: 11.5px; color: var(--text-muted); line-height: 1.5; flex: 1; }
.skip-btn {
  flex-shrink: 0; padding: 6px 14px; font-size: 12px;
  color: var(--text-secondary); background: var(--surface-2);
  border: 1px solid var(--border); border-radius: 6px;
  transition: color 0.15s, background 0.15s; white-space: nowrap;
}
.skip-btn:hover { color: var(--text-primary); background: var(--surface-3); }
</style>
