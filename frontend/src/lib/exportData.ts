import { IS_DESKTOP } from './env'

export type ExportFormat = 'csv' | 'tsv' | 'json' | 'md'

// Web-only fallback: blob URL + anchor click.
function triggerWebDownload(content: string, filename: string, mimeType: string) {
  const blob = new Blob([content], { type: mimeType })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  setTimeout(() => URL.revokeObjectURL(url), 150)
}

// Desktop: show native macOS/Windows save dialog, write via Go backend.
async function triggerDesktopSave(content: string, filename: string): Promise<void> {
  const { SaveFileWithDialog } = await import('../../wailsjs/go/main/App')
  await SaveFileWithDialog(filename, content)
}

function escCsv(v: unknown): string {
  const s = v === null || v === undefined ? '' : String(v)
  return s.includes(',') || s.includes('"') || s.includes('\n') || s.includes('\r')
    ? `"${s.replace(/"/g, '""')}"`
    : s
}

function escTsv(v: unknown): string {
  return (v === null || v === undefined ? '' : String(v))
    .replace(/\t/g, ' ')
    .replace(/[\n\r]/g, ' ')
}

function escMd(v: unknown): string {
  return (v === null || v === undefined ? '' : String(v))
    .replace(/\|/g, '\\|')
    .replace(/[\n\r]/g, ' ')
}

export async function exportData(
  fmt: ExportFormat,
  columns: string[],
  rows: Record<string, unknown>[],
  name: string,
): Promise<void> {
  let content: string
  let mimeType: string
  let ext: string

  switch (fmt) {
    case 'csv':
      content = [
        columns.map(escCsv).join(','),
        ...rows.map((r) => columns.map((c) => escCsv(r[c])).join(',')),
      ].join('\n')
      mimeType = 'text/csv'
      ext = 'csv'
      break

    case 'tsv':
      content = [
        columns.join('\t'),
        ...rows.map((r) => columns.map((c) => escTsv(r[c])).join('\t')),
      ].join('\n')
      mimeType = 'text/tab-separated-values'
      ext = 'tsv'
      break

    case 'json':
      content = JSON.stringify(rows, null, 2)
      mimeType = 'application/json'
      ext = 'json'
      break

    case 'md': {
      const header = '| ' + columns.map(escMd).join(' | ') + ' |'
      const sep    = '| ' + columns.map(() => '---').join(' | ') + ' |'
      const body   = rows.map((r) => '| ' + columns.map((c) => escMd(r[c])).join(' | ') + ' |').join('\n')
      content  = [header, sep, body].join('\n')
      mimeType = 'text/markdown'
      ext      = 'md'
      break
    }
  }

  const filename = `${name}.${ext}`
  if (IS_DESKTOP) {
    await triggerDesktopSave(content, filename)
  } else {
    triggerWebDownload(content, filename, mimeType)
  }
}
