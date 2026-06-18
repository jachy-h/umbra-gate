export function formatNum(n) {
    const value = Number(n || 0);
    if (value >= 1000000) return ((value / 1000000).toFixed(1) + 'M').replace('.0', '');
    if (value >= 1000) return ((value / 1000).toFixed(1) + 'K').replace('.0', '');
    return String(value);
}

export function fmtPercent(n) {
    return `${Math.round(Number(n || 0) * 100)}%`;
}

export function fmtDuration(ms) {
    const value = Number(ms || 0);
    return value >= 1000 ? `${(value / 1000).toFixed(1)}s` : `${Math.round(value)}ms`;
}

export function fmtTime(t) {
    return new Date(`${t}Z`).toLocaleString();
}
