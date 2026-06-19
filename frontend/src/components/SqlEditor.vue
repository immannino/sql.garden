<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue'
import { EditorView, keymap } from '@codemirror/view'
import { EditorState, Prec, Compartment } from '@codemirror/state'
import { basicSetup } from 'codemirror'
import { sql, PostgreSQL } from '@codemirror/lang-sql'

const props = defineProps<{
  modelValue: string
  height?: number
  schema?: Record<string, readonly string[]>
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
  'run': []
}>()

const container = ref<HTMLElement>()
let view: EditorView | null = null
let ignoreNextUpdate = false

const sqlCompartment = new Compartment()

function makeSqlExt(schema?: Record<string, readonly string[]>) {
  return sql({ dialect: PostgreSQL, schema: schema ?? {} })
}

const theme = EditorView.theme({
  '&': {
    backgroundColor: 'transparent',
    color: '#e6edf3',
    height: '100%',
  },
  '&.cm-focused': { outline: 'none' },
  '.cm-scroller': {
    fontFamily: 'var(--font-mono, "JetBrains Mono", ui-monospace, monospace)',
    fontSize: '12px',
    lineHeight: '1.55',
    overflow: 'auto',
  },
  '.cm-content': {
    caretColor: '#58a6ff',
    padding: '5px 0',
    minHeight: '48px',
  },
  '.cm-line': { padding: '0 8px' },
  '.cm-cursor, .cm-dropCursor': { borderLeftColor: '#58a6ff' },
  '&.cm-focused .cm-selectionBackground, .cm-selectionBackground, ::selection': {
    backgroundColor: 'rgba(88, 166, 255, 0.22)',
  },
  '.cm-activeLine': { backgroundColor: 'rgba(255,255,255,0.04)' },
  '.cm-activeLineGutter': { backgroundColor: 'rgba(255,255,255,0.04)' },
  '.cm-gutters': {
    backgroundColor: 'rgba(0,0,0,0.18)',
    color: '#4a5568',
    border: 'none',
    borderRight: '1px solid rgba(48,54,61,0.9)',
  },
  '.cm-lineNumbers .cm-gutterElement': {
    padding: '0 8px 0 4px',
    minWidth: '28px',
  },
  '.cm-matchingBracket': {
    color: 'inherit',
    backgroundColor: 'rgba(88,166,255,0.18)',
    outline: '1px solid rgba(88,166,255,0.35)',
    borderRadius: '2px',
  },
  '.cm-searchMatch': {
    backgroundColor: 'rgba(247,185,85,0.2)',
    outline: '1px solid rgba(247,185,85,0.4)',
  },
  '.cm-searchMatch.cm-searchMatch-selected': {
    backgroundColor: 'rgba(247,185,85,0.35)',
  },
  '.cm-tooltip': {
    backgroundColor: '#1c2128',
    border: '1px solid #30363d',
    borderRadius: '4px',
    boxShadow: '0 4px 16px rgba(0,0,0,0.5)',
  },
  '.cm-tooltip.cm-tooltip-autocomplete > ul > li[aria-selected]': {
    backgroundColor: 'rgba(88,166,255,0.15)',
  },
  '.cm-completionIcon': { paddingRight: '4px' },
  // SQL syntax token colors (GitHub Dark palette)
  '.tok-keyword': { color: '#ff7b72', fontWeight: '500' },
  '.tok-string': { color: '#a5d6ff' },
  '.tok-string2': { color: '#a5d6ff' },
  '.tok-number': { color: '#f2cc60' },
  '.tok-comment': { color: '#6e7681', fontStyle: 'italic' },
  '.tok-operator': { color: '#ff7b72' },
  '.tok-punctuation': { color: '#8b949e' },
  '.tok-name': { color: '#e6edf3' },
  '.tok-variableName': { color: '#ffa657' },
  '.tok-typeName': { color: '#ffa657' },
  '.tok-function(.tok-variableName)': { color: '#d2a8ff' },
}, { dark: true })

// Prec.highest so our Mod-Enter overrides defaultKeymap's insertBlankLine binding
const runKeymap = Prec.highest(keymap.of([
  {
    key: 'Mod-Enter',
    run() { emit('run'); return true },
  },
]))

function createView(parent: HTMLElement) {
  const state = EditorState.create({
    doc: props.modelValue,
    extensions: [
      basicSetup,
      sqlCompartment.of(makeSqlExt(props.schema)),
      theme,
      runKeymap,
      EditorView.updateListener.of((update) => {
        if (update.docChanged) {
          ignoreNextUpdate = true
          emit('update:modelValue', update.state.doc.toString())
        }
      }),
      EditorView.lineWrapping,
    ],
  })
  return new EditorView({ state, parent })
}

onMounted(() => {
  if (!container.value) return
  view = createView(container.value)
})

onUnmounted(() => {
  view?.destroy()
  view = null
})

// Live schema updates — reconfigure the sql() extension in-place
watch(() => props.schema, (newSchema) => {
  view?.dispatch({ effects: sqlCompartment.reconfigure(makeSqlExt(newSchema)) })
})

// Sync external modelValue changes (e.g. loading a different node)
watch(() => props.modelValue, (newVal) => {
  if (ignoreNextUpdate) { ignoreNextUpdate = false; return }
  if (!view) return
  const current = view.state.doc.toString()
  if (current !== newVal) {
    view.dispatch({
      changes: { from: 0, to: current.length, insert: newVal },
    })
  }
})
</script>

<template>
  <div
    ref="container"
    class="sql-editor-wrap"
    :style="height ? { height: `${height}px` } : {}"
  />
</template>

<style scoped>
.sql-editor-wrap {
  width: 100%;
  overflow: hidden;
  border-radius: 0;
}

.sql-editor-wrap :deep(.cm-editor) {
  height: 100%;
}

.sql-editor-wrap :deep(.cm-scroller) {
  height: 100%;
}
</style>
