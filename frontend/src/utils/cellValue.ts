export function isBinaryColumn(type = ''): boolean {
    return /\b(bytea|blob|tinyblob|mediumblob|longblob|binary|varbinary|raw|long raw|image|bfile)\b/i.test(type);
}

export function cellValue(value: unknown, type = '') {
    const raw = value === null ? 'NULL' : value === undefined ? 'undefined'
        : typeof value === 'string' ? value : typeof value === 'object' ? JSON.stringify(value, null, 2) : String(value);
    let bytes: Uint8Array | undefined;
    if (value instanceof Uint8Array) bytes = value;
    else if (value instanceof ArrayBuffer) bytes = new Uint8Array(value);
    else if (isBinaryColumn(type) && value !== null && value !== undefined) {
        if (Array.isArray(value) && value.every(n => Number.isInteger(n) && n >= 0 && n <= 255)) bytes = new Uint8Array(value);
        else if (typeof value === 'string') {
            const hex = /^(?:0x|\\x)([\da-f]*)$/i.exec(value) ?? /^X'([\da-f]*)'$/i.exec(value);
            if (hex && hex[1]!.length % 2 === 0) bytes = Uint8Array.from(hex[1]!.match(/../g) ?? [], part => parseInt(part, 16));
            else bytes = new TextEncoder().encode(value);
        }
    }
    if (bytes) {
        const lines: string[] = [];
        for (let offset = 0; offset < bytes.length; offset += 16) {
            const chunk = bytes.slice(offset, offset + 16);
            const hex = Array.from(chunk, byte => byte.toString(16).padStart(2, '0')).join(' ');
            const ascii = Array.from(chunk, byte => byte >= 32 && byte <= 126 ? String.fromCharCode(byte) : '.').join('');
            lines.push(offset.toString(16).padStart(8, '0') + '  ' + hex.padEnd(47) + '  |' + ascii + '|');
        }
        return { kind: 'Binary', raw, formatted: lines.join('\n'), text: new TextDecoder().decode(bytes), size: `${bytes.length.toLocaleString()} bytes` };
    }
    let kind = /\bxml\b/i.test(type) ? 'XML' : 'Text';
    let formatted = raw;
    if (value !== null && (typeof value === 'object' || /json/i.test(type) || (typeof value === 'string' && /^[\s]*[\[{]/.test(value)))) {
        try { formatted = JSON.stringify(typeof value === 'string' ? JSON.parse(value) : value, null, 2); kind = 'JSON'; } catch { /* Keep malformed JSON readable. */ }
    } else if (typeof value === 'string' && /^\s*<[^>]+>/.test(value)) kind = 'XML';
    // Keep XML whitespace exactly as stored; it can be significant in text nodes.
    return { kind, raw, formatted, text: raw, size: `${Array.from(raw).length.toLocaleString()} characters` };
}
