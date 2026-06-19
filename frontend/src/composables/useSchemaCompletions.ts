import { computed } from 'vue'
import { useSchemaStore } from '../stores/schema'
import { useQueryResults } from './useQueryResults'

/**
 * Builds a CodeMirror sql() schema map from the current canvas nodes.
 * Table nodes contribute their declared columns; query nodes contribute
 * whatever columns their last result returned.
 */
export function useSchemaCompletions() {
  const schemaStore = useSchemaStore()
  const { results } = useQueryResults()

  const sqlSchema = computed<Record<string, readonly string[]>>(() => {
    const schema: Record<string, string[]> = {}
    for (const node of schemaStore.nodes) {
      if (node.kind === 'table') {
        schema[node.name] = node.columns.map((c) => c.name)
      } else if (node.kind === 'query') {
        const r = results[node.id]
        if (r?.columns?.length) {
          schema[node.name] = r.columns
        } else if (node.isView) {
          // View exists but hasn't been run yet — list it with no columns
          schema[node.name] = []
        }
      }
    }
    return schema
  })

  return { sqlSchema }
}
