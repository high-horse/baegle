// Preserve SQL Server batches and Oracle/Db2 routine bodies when choosing the
// statement at the cursor. CodeMirror's SQL tree splits at inner semicolons.
export function enterpriseStatementAt(text: string, cursor: number, dialect: string): { from: number; to: number } | undefined {
  if (!['oracle', 'sqlserver', 'db2'].includes(dialect)) return;
  const ranges: { from: number; to: number }[] = [];
  let start = 0, i = 0, delimiter = ';', first = '', object = '', block = false, code = false;
  const add = (to: number, next: number) => {
    if (code) ranges.push({ from: start, to });
    start = next; first = ''; object = ''; block = false; code = false;
  };
  while (i < text.length) {
    const c = text[i];
    if (/\s/.test(c)) { i++; continue; }
    if (/[Gg/\-]/.test(c) && !text.slice(text.lastIndexOf('\n', i - 1) + 1, i).trim()) {
      const end = text.indexOf('\n', i) < 0 ? text.length : text.indexOf('\n', i);
      const line = text.slice(i, end).trim();
      if (dialect === 'db2' && /^--#SET TERMINATOR\s+.$/i.test(line)) {
        if (code) return;
        delimiter = line.slice(-1); start = end; i = end; continue;
      }
      if (dialect === 'sqlserver' && /^GO(?:\s*--.*)?$/i.test(line) || dialect === 'oracle' && line === '/') {
        add(i, end); i = end; continue;
      }
    }
    if (text.startsWith('--', i)) { const end = text.indexOf('\n', i); i = end < 0 ? text.length : end; continue; }
    if (text.startsWith('/*', i)) {
      let depth = 1; i += 2;
      while (i < text.length && depth) {
        if (dialect === 'sqlserver' && text.startsWith('/*', i)) { depth++; i += 2; }
        else if (text.startsWith('*/', i)) { depth--; i += 2; }
        else i++;
      }
      if (depth) return;
      continue;
    }
    if (c === delimiter && dialect !== 'sqlserver' && !block) { add(i + 1, i + 1); i++; continue; }
    code = true;
    if (dialect === 'oracle' && /[qQ]/.test(c) && text[i + 1] === "'") {
      const end = ({ '[': ']', '{': '}', '(': ')', '<': '>' } as Record<string, string>)[text[i + 2]] ?? text[i + 2];
      const close = text.indexOf(end + "'", i + 3); if (close < 0) return;
      i = close + 2; continue;
    }
    if (c === "'" || c === '"' || c === '[' && dialect === 'sqlserver') {
      const end = c === '[' ? ']' : c; i++; let closed = false;
      while (i < text.length) {
        if (text[i++] === end) { if (text[i] === end) i++; else { closed = true; break; } }
      }
      if (!closed) return;
      continue;
    }
    if (/[a-zA-Z_]/.test(c)) {
      let end = i + 1; while (end < text.length && /[\w$#]/.test(text[end])) end++;
      const word = text.slice(i, end).toUpperCase(); first ||= word;
      if (first === 'CREATE' && !object && !['CREATE','OR','REPLACE','EDITIONABLE','NONEDITIONABLE'].includes(word)) object = word;
      if (dialect === 'oracle' && (['BEGIN','DECLARE'].includes(first) || first === 'CREATE' && ['PROCEDURE','FUNCTION','PACKAGE','TRIGGER','TYPE'].includes(object))) block = true;
      i = end; continue;
    }
    i++;
  }
  add(text.length, text.length);
  return ranges.find(range => cursor >= range.from && cursor <= range.to);
}
