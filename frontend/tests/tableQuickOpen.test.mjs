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

test("lists SQLite, MySQL, PostgreSQL, and MongoDB objects before typing", () => {
    const results = filterDatabaseObjects(objects, "");
    assert.deepEqual(results.map(item => item.name), ["customers", "active_customers", "orders", "users", "events"]);
    assert.deepEqual(results.map(item => item.type.toLowerCase()), ["table", "view", "table", "table", "collection"]);
    assert.equal(objects[0].type, "table", "does not mutate shared sidebar metadata");
});

test("searches names and schema-qualified names case-insensitively", () => {
    assert.deepEqual(filterDatabaseObjects(objects, "CUSTOMER").map(item => item.name), ["customers", "active_customers"]);
    assert.deepEqual(filterDatabaseObjects(objects, "sales.orders").map(item => item.name), ["orders"]);
    assert.deepEqual(filterDatabaseObjects(objects, " APP   sales ord ").map(item => item.name), ["orders"]);
    assert.deepEqual(filterDatabaseObjects(objects, "missing"), []);
    assert.deepEqual(filterDatabaseObjects([], ""), []);
});
