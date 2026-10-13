import assert from "node:assert/strict";
import test from "node:test";
import { orderResultRows, parseResultOrder, sortResultRows } from "../src/utils/resultSort.ts";
import { compileResultFilter } from "../src/utils/resultFilter.ts";

test("parses multiple sort keys, quoted names, directions, and column positions", () => {
    assert.deepEqual(parseResultOrder('ORDER BY "last, name" asc, `score` DESC, 3', ['last, name', 'score', 'id']), [
        { column: 1, direction: 'asc' }, { column: 2, direction: 'desc' }, { column: 3, direction: 'asc' },
    ]);
    assert.deepEqual(parseResultOrder('SCORE', ['score']), [{ column: 1, direction: 'asc' }]);
    assert.deepEqual(parseResultOrder('"a""b"', ['a"b']), [{ column: 1, direction: 'asc' }]);
    assert.deepEqual(parseResultOrder('', ['score']), []);
});

test("rejects invalid or ambiguous clauses without executing SQL", () => {
    for (const clause of ['missing', 'score,', 'score sideways', 'score; DELETE FROM users', 'random()', '0', '3']) {
        assert.throws(() => parseResultOrder(clause, ['score', 'id']), undefined, clause);
    }
    assert.throws(() => parseResultOrder('id', ['id', 'id']), /Ambiguous/);
    assert.deepEqual(parseResultOrder('2 DESC', ['id', 'id']), [{ column: 2, direction: 'desc' }]);
});

test("orders filtered rows by multiple keys while preserving row identities and ties", () => {
    const rows = [
        { row: ['B', 8], index: 0, key: 'a' },
        { row: ['A', 3], index: 1, key: 'b' },
        { row: ['A', 9], index: 2, key: 'c' },
        { row: ['A', 9], index: 3, key: 'd' },
        { row: ['C', 1], index: 4, key: 'e' },
    ];
    const columns = ['name', 'score'];
    const predicate = compileResultFilter('score >= 3', columns);
    const sorted = orderResultRows(rows.filter(item => predicate(item.row)), parseResultOrder('name, score DESC', columns));
    assert.deepEqual(sorted.map(item => item.key), ['c', 'd', 'b', 'a']);
    assert.deepEqual(rows.map(item => item.index), [0, 1, 2, 3, 4]);
    assert.equal(sorted[0], rows[2]);
    assert.equal(orderResultRows(rows, []), rows);
});

test("retains existing single-column sorting and BSON integer precision", () => {
    const rows = [
        { row: ['{"$numberLong":"9007199254740993"}'], index: 0 },
        { row: [null], index: 1 },
        { row: ['{"$numberLong":"9007199254740992"}'], index: 2 },
    ];
    assert.deepEqual(sortResultRows(rows, 1, 'asc').map(item => item.index), [1, 2, 0]);
    assert.deepEqual(sortResultRows(rows, 1, 'desc').map(item => item.index), [0, 2, 1]);
});
