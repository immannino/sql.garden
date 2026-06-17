import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useSchemaStore } from './schema'

describe('useSchemaStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('addQueryNode adds a node', () => {
    const store = useSchemaStore()
    store.addQueryNode({ id: 'q1', name: 'q1', x: 0, y: 0, sql: 'SELECT 1' })
    expect(store.nodes).toHaveLength(1)
    expect(store.nodes[0].kind).toBe('query')
  })

  it('addQueryNode is idempotent for same id', () => {
    const store = useSchemaStore()
    store.addQueryNode({ id: 'q1', name: 'q1', x: 0, y: 0, sql: 'SELECT 1' })
    store.addQueryNode({ id: 'q1', name: 'q1', x: 0, y: 0, sql: 'SELECT 2' })
    expect(store.nodes).toHaveLength(1)
  })

  it('addChartNode adds a chart node', () => {
    const store = useSchemaStore()
    store.addChartNode({ id: 'c1', name: 'c1', x: 0, y: 0, sourceId: null, sql: '', chartType: 'barY', xColumn: '', yColumn: '' })
    expect(store.nodes[0].kind).toBe('chart')
  })

  it('addSection adds a section node at front of array', () => {
    const store = useSchemaStore()
    store.addQueryNode({ id: 'q1', name: 'q1', x: 0, y: 0, sql: '' })
    store.addSection({ id: 's1', name: 'Group', x: 0, y: 0, w: 400, h: 300 })
    expect(store.nodes[0].kind).toBe('section')
    expect(store.nodes[0].id).toBe('s1')
  })

  it('removeNode removes a node by id', () => {
    const store = useSchemaStore()
    store.addQueryNode({ id: 'q1', name: 'q1', x: 0, y: 0, sql: '' })
    store.removeNode('q1')
    expect(store.nodes).toHaveLength(0)
  })

  it('updatePositions moves multiple nodes', () => {
    const store = useSchemaStore()
    store.addQueryNode({ id: 'q1', name: 'q1', x: 0, y: 0, sql: '' })
    store.addQueryNode({ id: 'q2', name: 'q2', x: 0, y: 0, sql: '' })
    store.updatePositions(new Map([['q1', { x: 10, y: 20 }], ['q2', { x: 30, y: 40 }]]))
    const q1 = store.nodes.find(n => n.id === 'q1')!
    const q2 = store.nodes.find(n => n.id === 'q2')!
    expect(q1.x).toBe(10); expect(q1.y).toBe(20)
    expect(q2.x).toBe(30); expect(q2.y).toBe(40)
  })

  it('setRefreshInterval updates the interval', () => {
    const store = useSchemaStore()
    store.addQueryNode({ id: 'q1', name: 'q1', x: 0, y: 0, sql: '' })
    store.setRefreshInterval('q1', 30)
    const node = store.nodes.find(n => n.id === 'q1') as any
    expect(node.refreshInterval).toBe(30)
  })

  it('updateQuerySql updates SQL', () => {
    const store = useSchemaStore()
    store.addQueryNode({ id: 'q1', name: 'q1', x: 0, y: 0, sql: 'SELECT 1' })
    store.updateQuerySql('q1', 'SELECT 2')
    const node = store.nodes.find(n => n.id === 'q1') as any
    expect(node.sql).toBe('SELECT 2')
  })

  it('clear resets all nodes', () => {
    const store = useSchemaStore()
    store.addQueryNode({ id: 'q1', name: 'q1', x: 0, y: 0, sql: '' })
    store.clear()
    expect(store.nodes).toHaveLength(0)
  })

  it('updateNodeSize sets w and h', () => {
    const store = useSchemaStore()
    store.addQueryNode({ id: 'q1', name: 'q1', x: 0, y: 0, sql: '' })
    store.updateNodeSize('q1', 400, 200)
    const node = store.nodes.find(n => n.id === 'q1') as any
    expect(node.w).toBe(400); expect(node.h).toBe(200)
  })

  it('renameNode changes id and name', () => {
    const store = useSchemaStore()
    store.addQueryNode({ id: 'q1', name: 'q1', x: 0, y: 0, sql: '' })
    store.renameNode('q1', 'renamed')
    expect(store.nodes[0].id).toBe('renamed')
    expect(store.nodes[0].name).toBe('renamed')
  })
})
