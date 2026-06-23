export interface TypeBadge {
  label: string
  category: 'int' | 'float' | 'text' | 'bool' | 'date' | 'other'
}

export function classifyColumnType(raw: string | undefined): TypeBadge {
  if (!raw) return { label: '?', category: 'other' }

  const t = raw.toUpperCase()

  // integers
  if (/^(TINY|SMALL|UBIG|HUGE|UINT|USHORT|UTINY|USMAL)?INT(EGER|2|4|8)?$|^BIGINT$|^HUGEINT$|^UBIGINT$|^SMALLINT$|^TINYINT$|^INT[0-9]*$/.test(t))
    return { label: 'INT', category: 'int' }

  // floats / decimals
  if (/^(FLOAT|DOUBLE|REAL|NUMERIC|DECIMAL)/.test(t))
    return { label: t.startsWith('DECIMAL') || t.startsWith('NUMERIC') ? 'DEC' : 'FLOAT', category: 'float' }

  // text
  if (/^(VARCHAR|TEXT|CHAR|BPCHAR|STRING|CLOB|BLOB|BYTEA|UUID)/.test(t))
    return { label: t.startsWith('UUID') ? 'UUID' : 'TEXT', category: 'text' }

  // boolean
  if (/^BOOL(EAN)?$/.test(t))
    return { label: 'BOOL', category: 'bool' }

  // dates / times
  if (/^TIMESTAMP/.test(t)) return { label: 'TS', category: 'date' }
  if (/^DATE$/.test(t))      return { label: 'DATE', category: 'date' }
  if (/^TIME/.test(t))       return { label: 'TIME', category: 'date' }
  if (/^INTERVAL/.test(t))   return { label: 'IVTL', category: 'date' }

  // Arrow names (from WASM path)
  if (/^Int(8|16|32|64)?$/.test(raw))   return { label: 'INT',   category: 'int' }
  if (/^Uint(8|16|32|64)?$/.test(raw))  return { label: 'INT',   category: 'int' }
  if (/^Float(16|32|64)?$/.test(raw))   return { label: 'FLOAT', category: 'float' }
  if (/^Decimal/.test(raw))             return { label: 'DEC',   category: 'float' }
  if (/^Utf8$|^LargeUtf8$/.test(raw))  return { label: 'TEXT',  category: 'text' }
  if (/^Bool$/.test(raw))               return { label: 'BOOL',  category: 'bool' }
  if (/^Timestamp/.test(raw))           return { label: 'TS',    category: 'date' }
  if (/^Date/.test(raw))                return { label: 'DATE',  category: 'date' }
  if (/^Time/.test(raw))                return { label: 'TIME',  category: 'date' }
  if (/^List$|^FixedSizeList/.test(raw)) return { label: 'LIST', category: 'other' }
  if (/^Struct/.test(raw))              return { label: 'OBJ',   category: 'other' }
  if (/^Map/.test(raw))                 return { label: 'MAP',   category: 'other' }

  // shorten anything else to max 5 chars
  return { label: t.slice(0, 5), category: 'other' }
}
