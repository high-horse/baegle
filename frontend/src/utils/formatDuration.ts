export function formatDuration(milliseconds: number): string {
    return milliseconds < 1000
        ? `${milliseconds} ms`
        : `${Number((milliseconds / 1000).toFixed(2))} s`;
}
