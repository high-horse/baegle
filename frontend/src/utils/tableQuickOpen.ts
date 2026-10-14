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

// Keep the index outside Vue's deep reactivity. Build it lazily and yield while
// scanning large catalogs so typing and painting can run between batches.
export function createDatabaseObjectSearch<T extends { name: string; type: string; database: string; schema?: string }>(items: T[]) {
    const index: ({ item: T; text: string } | null | undefined)[] = [];
    return async (query: string, cancelled: () => boolean = () => false): Promise<T[] | null> => {
        const terms = query.toLowerCase().trim().split(/\s+/).filter(Boolean);
        if (!terms.length) return [];
        const matches: T[] = [];
        let batchStart = performance.now();
        for (let i = 0; i < items.length; i++) {
            if (cancelled()) return null;
            let entry = index[i];
            if (entry === undefined) {
                const item = items[i];
                const type = normalizeTableType(item.type);
                entry = ["TABLE", "VIEW", "COLLECTION"].includes(type)
                    ? { item: { ...item, type }, text: [item.database, item.schema, item.name].filter(Boolean).join(".").toLowerCase() }
                    : null;
                index[i] = entry;
            }
            if (entry && terms.every(term => entry.text.includes(term))) matches.push(entry.item);
            if (matches.length === 100) break;
            if (i % 128 === 127 && performance.now() - batchStart >= 4) {
                await new Promise<void>(resolve => setTimeout(resolve, 0));
                batchStart = performance.now();
            }
        }
        return cancelled() ? null : matches;
    };
}
