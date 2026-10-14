export function normalizeTableType(type: string): string {
    switch (type.trim().toUpperCase()) {
        case "BASE TABLE":
        case "FOREIGN TABLE":
            return "TABLE";
        case "MATERIALIZED VIEW":
            return "VIEW";
        default:
            return type.trim().toUpperCase();
    }
}

export function filterDatabaseObjects<T extends { name: string; type: string; database: string; schema?: string }>(items: T[], query: string): T[] {
    const terms = query.toLowerCase().trim().split(/\s+/).filter(Boolean);
    if (!terms.length) return [];
    const matches: T[] = [];
    for (const item of items) {
        const type = normalizeTableType(item.type);
        if (!["TABLE", "VIEW", "COLLECTION"].includes(type)) continue;
        const name = [item.database, item.schema, item.name].filter(Boolean).join(".").toLowerCase();
        if (terms.every(term => name.includes(term))) matches.push({ ...item, type });
        if (matches.length === 100) break;
    }
    return matches;
}
