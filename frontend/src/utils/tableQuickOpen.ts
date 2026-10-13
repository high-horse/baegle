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
    return items.map(item => ({ ...item, type: normalizeTableType(item.type) })).filter(item => {
        if (!["TABLE", "VIEW", "COLLECTION"].includes(item.type)) return false;
        const name = [item.database, item.schema, item.name].filter(Boolean).join(".").toLowerCase();
        return terms.every(term => name.includes(term));
    });
}
