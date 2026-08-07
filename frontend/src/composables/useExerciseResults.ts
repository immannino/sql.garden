import { reactive, ref } from 'vue'

export interface ExerciseResult {
  attempts:  number
  passed:    boolean | null  // null = never run
  passCount: number
  failCount: number
}

// Module-level singleton — shared across all component instances in the session
const results = reactive<Record<string, ExerciseResult>>({})
let _seq = 0
const navigateRequest = ref<{ id: string; seq: number } | null>(null)

export function useExerciseResults() {
  function recordAttempt(id: string, passed: boolean) {
    const prev = results[id]
    results[id] = {
      attempts:  (prev?.attempts  ?? 0) + 1,
      passed,
      passCount: (prev?.passCount ?? 0) + (passed ? 1 : 0),
      failCount: (prev?.failCount ?? 0) + (passed ? 0 : 1),
    }
  }

  function getResult(id: string): ExerciseResult {
    return results[id] ?? { attempts: 0, passed: null, passCount: 0, failCount: 0 }
  }

  function resetResult(id: string) {
    delete results[id]
  }

  function navigateToNode(id: string) {
    navigateRequest.value = { id, seq: ++_seq }
  }

  return { results, navigateRequest, recordAttempt, getResult, resetResult, navigateToNode }
}
