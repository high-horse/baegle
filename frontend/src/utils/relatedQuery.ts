type Relation = { target: { name: string; schema: string; database: string }; columns: string[]; targetColumns: string[] };
export function relatedQuery(relation: Relation, row: Record<string, unknown>, driver: string): string {
  const mysql = driver === 'mysql';
  const quote = (name: string) => mysql ? '`' + name.replaceAll('`', '``') + '`' : '"' + name.replaceAll('"', '""') + '"';
  const literal = (value: unknown) => {
    if (value == null) throw new Error('This foreign key is NULL; there is no related record.');
    if (typeof value === 'number') {
      if (!Number.isFinite(value) || (Number.isInteger(value) && !Number.isSafeInteger(value))) throw new Error('The key cannot be represented precisely.');
      return String(value);
    }
    if (typeof value === 'boolean') return value ? 'TRUE' : 'FALSE';
    if (typeof value !== 'string' || value.includes('\0')) throw new Error('This key type is not supported for navigation.');
    if (mysql) {
      // Hex avoids dependence on the server's NO_BACKSLASH_ESCAPES mode.
      const hex = Array.from(new TextEncoder().encode(value), byte => byte.toString(16).padStart(2, '0')).join('');
      return `CONVERT(X'${hex}' USING utf8mb4)`;
    }
    const escaped = value.replaceAll("'", "''");
    return driver === 'sqlite' ? `'${escaped}'` : `E'${escaped.replaceAll('\\', '\\\\')}'`;
  };
  if (!relation.columns.length || relation.columns.length !== relation.targetColumns.length || relation.targetColumns.some(c => !c)) throw new Error('Incomplete foreign-key metadata.');
  const namespace = mysql ? relation.target.database : relation.target.schema;
  const table = [namespace, relation.target.name].filter(Boolean).map(quote).join('.');
  return `SELECT * FROM ${table} WHERE ${relation.columns.map((column, i) => `${quote(relation.targetColumns[i])} = ${literal(row[column])}`).join(' AND ')};`;
}
