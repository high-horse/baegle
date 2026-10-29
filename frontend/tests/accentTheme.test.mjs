import assert from 'node:assert/strict';
import test from 'node:test';
import { accentTheme, isAccentColor, DEFAULT_ACCENT } from '../src/utils/accentTheme.ts';

test('custom accents retain their fill and derive related highlight colors', () => {
    const teal = accentTheme(DEFAULT_ACCENT), purple = accentTheme('#B7A5E8');
    assert.equal(purple['--accent-fill'], '#b7a5e8');
    assert.notEqual(purple['--selection-bg'], teal['--selection-bg']);
    assert.notEqual(purple['--accent-surface'], teal['--accent-surface']);
    assert.equal(purple['--q-primary'], purple['--accent-color']);
});
test('dark and light fills get readable button text, dark accents are lifted', () => {
    const dark = accentTheme('#000000'), light = accentTheme('#ffffff');
    assert.equal(dark['--accent-on'], '#ffffff');
    assert.notEqual(dark['--accent-color'], '#000000');
    assert.equal(light['--accent-on'], '#000000');
    assert.equal(light['--accent-fill'], '#ffffff');
});
test('invalid color input cannot enter theme CSS', () => {
    for (const color of ['', '#123', '#gggggg', 'red', 'red; color: white']) {
        assert.equal(isAccentColor(color), false);
        assert.deepEqual(accentTheme(color), accentTheme(DEFAULT_ACCENT));
    }
});
