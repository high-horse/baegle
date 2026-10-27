export function quickFilter(column: string, value: unknown, operator: '=' | '<>' | 'LIKE' | 'IS NULL' | 'IS NOT NULL') {
  const name = `"${column.replaceAll('"', '""')}"`;
  if (operator === 'IS NULL' || operator === 'IS NOT NULL') return `${name} ${operator}`;
  if (value == null) return `${name} IS ${operator === '<>' ? 'NOT ' : ''}NULL`;
  const literal = typeof value === 'number' && Number.isFinite(value) ? String(value)
    : typeof value === 'boolean' ? String(value).toUpperCase()
    : `'${String(value).replaceAll("'", "''")}'`;
  if (operator === 'LIKE') return `${name} LIKE '%${String(value).replaceAll("'", "''")}%'`;
  return `${name} ${operator} ${literal}`;
}
