<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useDataPanel } from '../composables/useDataPanel'
import { useSchemaStore } from '../stores/schema'
import { useDuckDB } from '../composables/useDuckDB'
import { useQueryResults } from '../composables/useQueryResults'
import { IS_DESKTOP } from '../lib/env'

const { panelNodeId, closePanel } = useDataPanel()
const schemaStore = useSchemaStore()
const { exec, getTableInfo, query } = useDuckDB()
const { setResult } = useQueryResults()

const node = computed(() => {
  if (!panelNodeId.value) return null
  const n = schemaStore.nodes.find((n) => n.id === panelNodeId.value)
  if (!n) return null
  return (n.kind === 'data' || n.kind === 'table') ? n : null
})

watch(node, (n) => { if (!n) closePanel() })

// ── Edit state ────────────────────────────────────────────────────────────────
const editingCol = ref<string | null>(null)
const editType = ref('DOUBLE')
const editExpr = ref('')
const editError = ref<string | null>(null)
const isApplying = ref(false)
const isRemoving = ref(false)

const TYPE_OPTIONS = ['VARCHAR', 'DOUBLE', 'DECIMAL(18,2)', 'BIGINT', 'DATE', 'TIMESTAMP', 'BOOLEAN']

function hasCast(colName: string) {
  return !!node.value?.columnCasts?.[colName]
}

function startEdit(colName: string) {
  editingCol.value = colName
  editError.value = null
  const existing = node.value?.columnCasts?.[colName]
  if (existing) {
    editType.value = existing.type
    editExpr.value = existing.expr
  } else {
    editType.value = 'DOUBLE'
    updateExprDefault(colName, 'DOUBLE')
  }
}

function cancelEdit() {
  editingCol.value = null
  editError.value = null
}

function updateExprDefault(colName: string, type: string) {
  const safeCol = colName.replace(/"/g, '""')
  if (type === 'DOUBLE' || type.startsWith('DECIMAL')) {
    editExpr.value = `regexp_replace("${safeCol}", '[^0-9.]', '', 'g')::${type}`
  } else {
    editExpr.value = ''
  }
}

function onTypeChange() {
  if (editingCol.value) updateExprDefault(editingCol.value, editType.value)
}

async function applyEdit() {
  const colName = editingCol.value
  if (!colName || !node.value) return
  editError.value = null
  isApplying.value = true
  const safe = node.value.name.replace(/"/g, '""')
  const safeCol = colName.replace(/"/g, '""')
  const alterSQL = editExpr.value.trim()
    ? `ALTER TABLE "${safe}" ALTER COLUMN "${safeCol}" TYPE ${editType.value} USING (${editExpr.value.trim()})`
    : `ALTER TABLE "${safe}" ALTER COLUMN "${safeCol}" TYPE ${editType.value}`
  try {
    await exec(alterSQL)
    const columns = await getTableInfo(node.value.name)
    const newCasts = { ...(node.value.columnCasts ?? {}), [colName]: { type: editType.value, expr: editExpr.value.trim() } }
    if (node.value.kind === 'data') {
      schemaStore.updateDataNode(node.value.id, { columns, columnCasts: newCasts })
    } else {
      schemaStore.updateTableNode(node.value.id, { columns, columnCasts: newCasts })
    }
    editingCol.value = null
    if (IS_DESKTOP) {
      const { SaveTableData } = await import('../../wailsjs/go/main/App')
      await SaveTableData(node.value.name).catch(console.warn)
    }
  } catch (err) {
    editError.value = err instanceof Error ? err.message : String(err)
  } finally {
    isApplying.value = false
  }
}

async function removeCast(colName: string) {
  const n = node.value
  if (!n || n.kind !== 'data') return
  isRemoving.value = true
  editError.value = null

  const newCasts = { ...(n.columnCasts ?? {}) }
  delete newCasts[colName]
  schemaStore.updateDataNode(n.id, { columnCasts: newCasts })

  // Re-materialize from source to restore the original column type
  const sourceNode = n.sourceId
    ? schemaStore.nodes.find((nd) => nd.id === n.sourceId)
    : null
  const sql = (sourceNode?.kind === 'query' ? sourceNode.sql : n.sourceSql)?.trim().replace(/;+$/, '')

  if (sql) {
    const safe = n.name.replace(/"/g, '""')
    try {
      await exec(`CREATE OR REPLACE TABLE "${safe}" AS (${sql})`)
      for (const [cn, cast] of Object.entries(newCasts)) {
        const sc = cn.replace(/"/g, '""')
        const s = cast.expr
          ? `ALTER TABLE "${safe}" ALTER COLUMN "${sc}" TYPE ${cast.type} USING (${cast.expr})`
          : `ALTER TABLE "${safe}" ALTER COLUMN "${sc}" TYPE ${cast.type}`
        await exec(s).catch(console.warn)
      }
      const [columns, countResult] = await Promise.all([
        getTableInfo(n.name),
        query(`SELECT COUNT(*) AS n FROM "${safe}"`),
      ])
      schemaStore.updateDataNode(n.id, { columns, rowCount: Number(countResult.rows[0]?.n ?? 0) })
      setResult(n.id, { columns: [], rows: [], error: null, isRunning: false })
      if (IS_DESKTOP) {
        const { SaveTableData } = await import('../../wailsjs/go/main/App')
        await SaveTableData(n.name).catch(console.warn)
      }
    } catch (err) {
      editError.value = err instanceof Error ? err.message : String(err)
    }
  }

  editingCol.value = null
  isRemoving.value = false
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    if (editingCol.value) cancelEdit()
    else closePanel()
  }
}

onMounted(() => window.addEventListener('keydown', onKeydown))
onUnmounted(() => window.removeEventListener('keydown', onKeydown))
</script>

<template>
  <Transition name="panel-slide">
    <div v-if="node" class="props-panel" @mousedown.stop>
      <!-- Header -->
      <div class="pp-header">
        <!-- Cylinder icon for DataNode -->
        <svg v-if="node.kind === 'data'" class="pp-header-icon" viewBox="0 0 14 16" fill="none">
          <ellipse cx="7" cy="3.5" rx="5" ry="2" stroke="currentColor" stroke-width="1.3"/>
          <path d="M2 3.5v9c0 1.1 2.24 2 5 2s5-.9 5-2v-9" stroke="currentColor" stroke-width="1.3"/>
          <path d="M2 8c0 1.1 2.24 2 5 2s5-.9 5-2" stroke="currentColor" stroke-width="1.3" opacity="0.5"/>
        </svg>
        <!-- Table grid icon for TableNode -->
        <svg v-else class="pp-header-icon" viewBox="0 0 16 16" fill="none">
          <rect x="1" y="1" width="14" height="14" rx="2" stroke="currentColor" stroke-width="1.3"/>
          <line x1="1" y1="5.5" x2="15" y2="5.5" stroke="currentColor" stroke-width="1.3"/>
          <line x1="5.5" y1="5.5" x2="5.5" y2="15" stroke="currentColor" stroke-width="1.3"/>
        </svg>
        <span class="pp-header-title">{{ node.name }}</span>
        <button class="pp-close" title="Close (Esc)" @click="closePanel">
          <svg viewBox="0 0 10 10" fill="none">
            <path d="M2 2l6 6M8 2l-6 6" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
          </svg>
        </button>
      </div>

      <div class="pp-body">
        <!-- ── Columns ─────────────────────────────────────────────────────── -->
        <section class="pp-section">
          <div class="pp-section-label">Columns</div>

          <div v-if="node.columns.length" class="col-list">
            <div
              v-for="col in node.columns"
              :key="col.name"
              class="col-entry"
              :class="{ editing: editingCol === col.name }"
            >
              <!-- Row header -->
              <div class="col-row-header" @click="editingCol === col.name ? cancelEdit() : startEdit(col.name)">
                <span class="col-name">{{ col.name }}</span>
                <span class="col-type" :class="{ 'col-type-cast': hasCast(col.name) }">
                  {{ col.type }}
                </span>
                <svg v-if="hasCast(col.name)" class="cast-indicator" viewBox="0 0 8 8" fill="none" title="Type cast applied">
                  <circle cx="4" cy="4" r="3" stroke="currentColor" stroke-width="1.1"/>
                  <path d="M2.5 4l1 1 2-2" stroke="currentColor" stroke-width="1.1" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
                <svg class="col-chevron" :class="{ open: editingCol === col.name }" viewBox="0 0 8 8" fill="none">
                  <path d="M2 3l2 2 2-2" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
              </div>

              <!-- Edit form (expands inline) -->
              <div v-if="editingCol === col.name" class="col-edit-form" @mousedown.stop @click.stop>
                <div class="form-row">
                  <label class="form-label">Target type</label>
                  <select v-model="editType" class="type-select" @change="onTypeChange">
                    <option v-for="t in TYPE_OPTIONS" :key="t" :value="t">{{ t }}</option>
                  </select>
                </div>

                <div class="form-row">
                  <label class="form-label">
                    USING expression
                    <span class="form-label-hint">optional</span>
                  </label>
                  <input
                    v-model="editExpr"
                    class="expr-input"
                    placeholder="e.g. regexp_replace(&quot;col&quot;, '[^0-9.]', '', 'g')::DOUBLE"
                    spellcheck="false"
                    @keydown.enter.prevent="applyEdit"
                    @keydown.esc="cancelEdit"
                  />
                </div>

                <div v-if="editError" class="edit-error">{{ editError }}</div>

                <div class="form-actions">
                  <button
                    v-if="hasCast(col.name) && node.kind === 'data'"
                    class="action-btn remove-btn"
                    :disabled="isRemoving"
                    @click="removeCast(col.name)"
                  >
                    {{ isRemoving ? 'Removing…' : 'Remove cast' }}
                  </button>
                  <span class="actions-spacer" />
                  <button class="action-btn cancel-btn" @click="cancelEdit">Cancel</button>
                  <button class="action-btn apply-btn" :disabled="isApplying" @click="applyEdit">
                    {{ isApplying ? 'Applying…' : 'Apply' }}
                  </button>
                </div>
              </div>
            </div>
          </div>

          <div v-else class="empty-note">No columns</div>
        </section>

        <div class="pp-hint">
          Click a column to change its DuckDB type. Use a USING expression to transform values (e.g. strip commas from formatted numbers).
        </div>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.props-panel {
  position: fixed;
  top: 44px;
  right: 0;
  bottom: 0;
  width: 290px;
  background: var(--surface-1);
  border-left: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  z-index: 50;
  box-shadow: -4px 0 20px rgba(0, 0, 0, 0.25);
}

.panel-slide-enter-active, .panel-slide-leave-active { transition: transform 0.2s ease, opacity 0.2s ease; }
.panel-slide-enter-from, .panel-slide-leave-to { transform: translateX(20px); opacity: 0; }

/* ── Header ──────────────────────────────────────────────────────────────── */
.pp-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.pp-header-icon {
  width: 14px;
  height: 16px;
  color: var(--text-muted);
  flex-shrink: 0;
}

.pp-header-title {
  flex: 1;
  font-size: 12px;
  font-weight: 600;
  color: var(--text-primary);
  font-family: var(--font-mono);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pp-close {
  width: 22px;
  height: 22px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: none;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  color: var(--text-muted);
  flex-shrink: 0;
}
.pp-close svg { width: 10px; height: 10px; }
.pp-close:hover { background: var(--surface-2); color: var(--text-primary); }

/* ── Body ────────────────────────────────────────────────────────────────── */
.pp-body {
  flex: 1;
  overflow-y: auto;
  scrollbar-width: thin;
  scrollbar-color: var(--border) transparent;
}

.pp-section {
  padding: 12px;
  border-bottom: 1px solid var(--border);
}

.pp-section-label {
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--text-muted);
  margin-bottom: 8px;
}

.pp-hint {
  padding: 10px 12px;
  font-size: 10.5px;
  color: var(--text-muted);
  line-height: 1.5;
}

/* ── Column list ─────────────────────────────────────────────────────────── */
.col-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.col-entry {
  border-radius: 5px;
  border: 1px solid transparent;
  overflow: hidden;
  transition: border-color 0.12s;
}
.col-entry.editing {
  border-color: var(--border);
  background: var(--surface-0);
}

.col-row-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 5px 8px;
  border-radius: 5px;
  cursor: pointer;
  transition: background 0.1s;
}
.col-row-header:hover { background: var(--surface-2); }
.col-entry.editing .col-row-header { border-radius: 5px 5px 0 0; background: var(--surface-2); }

.col-name {
  flex: 1;
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.col-type {
  font-family: var(--font-mono);
  font-size: 10px;
  color: var(--text-muted);
  text-transform: uppercase;
  flex-shrink: 0;
}
.col-type-cast { color: var(--accent); }

.cast-indicator {
  width: 10px;
  height: 10px;
  color: var(--accent);
  flex-shrink: 0;
}

.col-chevron {
  width: 8px;
  height: 8px;
  color: var(--text-muted);
  flex-shrink: 0;
  transition: transform 0.15s;
}
.col-chevron.open { transform: rotate(180deg); }

/* ── Edit form ───────────────────────────────────────────────────────────── */
.col-edit-form {
  padding: 8px 8px 10px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  border-top: 1px solid var(--border);
}

.form-row {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.form-label {
  font-size: 10px;
  font-weight: 600;
  color: var(--text-muted);
  display: flex;
  align-items: center;
  gap: 5px;
}

.form-label-hint {
  font-weight: 400;
  font-style: italic;
  opacity: 0.75;
}

.type-select {
  font-size: 11px;
  font-family: var(--font-mono);
  background: var(--surface-1);
  border: 1px solid var(--border);
  color: var(--text-primary);
  border-radius: 4px;
  padding: 4px 6px;
  cursor: pointer;
  outline: none;
}
.type-select:focus { border-color: var(--accent); }

.expr-input {
  width: 100%;
  font-size: 10.5px;
  font-family: var(--font-mono);
  background: var(--surface-1);
  border: 1px solid var(--border);
  color: var(--text-primary);
  border-radius: 4px;
  padding: 5px 7px;
  outline: none;
  box-sizing: border-box;
  resize: vertical;
  min-height: 32px;
}
.expr-input:focus { border-color: var(--accent); }

.edit-error {
  font-size: 10px;
  font-family: var(--font-mono);
  color: var(--error);
  background: rgba(248, 81, 73, 0.07);
  border: 1px solid rgba(248, 81, 73, 0.2);
  border-radius: 4px;
  padding: 4px 7px;
  line-height: 1.4;
}

.form-actions {
  display: flex;
  align-items: center;
  gap: 5px;
}

.actions-spacer { flex: 1; }

.action-btn {
  padding: 3px 10px;
  font-size: 11px;
  border-radius: 4px;
  cursor: pointer;
  border: 1px solid var(--border);
  transition: background 0.1s, color 0.1s;
}
.action-btn:disabled { opacity: 0.6; cursor: not-allowed; }

.cancel-btn {
  background: none;
  color: var(--text-muted);
}
.cancel-btn:hover { background: var(--surface-2); color: var(--text-primary); }

.apply-btn {
  background: var(--accent);
  border-color: var(--accent);
  color: white;
}
.apply-btn:hover:not(:disabled) { opacity: 0.85; }

.remove-btn {
  background: none;
  color: var(--error, #f85149);
  border-color: rgba(248, 81, 73, 0.35);
  font-size: 10.5px;
  padding: 3px 8px;
}
.remove-btn:hover:not(:disabled) { background: rgba(248, 81, 73, 0.08); }

.empty-note {
  font-size: 11px;
  color: var(--text-muted);
  font-style: italic;
  text-align: center;
  padding: 12px;
}
</style>
