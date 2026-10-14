import assert from "node:assert/strict";
import test from "node:test";
import { filterDatabaseObjects } from "../src/utils/tableQuickOpen.ts";

const objects = [
    { name: "customers", type: "table", database: "app", schema: "main" },
    { name: "active_customers", type: "view", database: "app", schema: "main" },
    { name: "orders", type: "BASE TABLE", database: "app", schema: "sales" },
    { name: "users", type: "TABLE", database: "app", schema: "public" },
    { name: "events", type: "COLLECTION", database: "logs", schema: "logs" },
    { name: "customers_idx", type: "index", database: "app", schema: "main" },
];

test("does not scan objects before typing", () => {
    const unreadable = new Proxy([], { get() { throw new Error("should not read metadata"); } });
    assert.deepEqual(filterDatabaseObjects(unreadable, ""), []);
    assert.deepEqual(filterDatabaseObjects(unreadable, "   "), []);
});

test("normalizes SQLite, MySQL, PostgreSQL, and MongoDB objects", () => {
    const results = filterDatabaseObjects(objects, "s");
    assert.deepEqual(results.map(item => item.name), ["customers", "active_customers", "orders", "users", "events"]);
    assert.deepEqual(results.map(item => item.type.toLowerCase()), ["table", "view", "table", "table", "collection"]);
    assert.equal(objects[0].type, "table", "does not mutate shared sidebar metadata");
});

test("caps broad searches without scanning the remaining objects", () => {
    const many = Array.from({ length: 100 }, (_, index) => ({ ...objects[0], name: `table_${index}` }));
    Object.defineProperty(many, 100, { get() { throw new Error("searched beyond result limit"); } });
    assert.equal(filterDatabaseObjects(many, "table").length, 100);
});

test("searches names and schema-qualified names case-insensitively", () => {
    assert.deepEqual(filterDatabaseObjects(objects, "CUSTOMER").map(item => item.name), ["customers", "active_customers"]);
    assert.deepEqual(filterDatabaseObjects(objects, "sales.orders").map(item => item.name), ["orders"]);
    assert.deepEqual(filterDatabaseObjects(objects, " APP   sales ord ").map(item => item.name), ["orders"]);
    assert.deepEqual(filterDatabaseObjects(objects, "missing"), []);
    assert.deepEqual(filterDatabaseObjects([], ""), []);
});
