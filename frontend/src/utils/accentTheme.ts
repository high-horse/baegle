export const DEFAULT_ACCENT = '#59cbb7';
export const isAccentColor = (color: string) => /^#[\da-f]{6}$/i.test(color);

type RGB = [number, number, number];
const rgb = (hex: string): RGB => [1, 3, 5].map(offset => parseInt(hex.slice(offset, offset + 2), 16)) as RGB;
const hex = (color: RGB) => '#' + color.map(value => Math.round(value).toString(16).padStart(2, '0')).join('');
const mix = (a: RGB, b: RGB, amount: number): RGB => a.map((value, i) => value * (1 - amount) + b[i] * amount) as RGB;
const luminance = (color: RGB) => color.map(value => {
    const channel = value / 255;
    return channel <= .04045 ? channel / 12.92 : ((channel + .055) / 1.055) ** 2.4;
}).reduce((total, value, i) => total + value * [.2126, .7152, .0722][i], 0);

// Keep the chosen fill exact, while lifting dark accent text for charcoal panels.
export function accentTheme(input: string): Record<string, string> {
    const fill = isAccentColor(input) ? input.toLowerCase() : DEFAULT_ACCENT;
    const color = rgb(fill), panel = rgb('#44484d');
    let readable = color;
    for (let amount = 0; amount <= 1; amount += .05) {
        readable = mix(color, [255, 255, 255], amount);
        if ((luminance(readable) + .05) / (luminance(panel) + .05) >= 4.5) break;
    }
    const lightness = luminance(color);
    return {
        '--q-primary': hex(readable), '--q-accent': hex(readable),
        '--accent-color': hex(readable), '--accent-fill': fill,
        '--accent-on': (lightness + .05) / .05 >= 1.05 / (lightness + .05) ? '#000000' : '#ffffff',
        '--accent-surface': hex(mix(rgb('#35383c'), color, .2)),
        '--accent-tint': `rgba(${color.join(', ')}, 0.14)`,
        '--selection-bg': hex(mix(rgb('#35383c'), color, .3)),
        '--selection-border': hex(mix(readable, [255, 255, 255], .2)),
    };
}
