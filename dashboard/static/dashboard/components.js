import { getLang, setLang, t } from './i18n.js';
import { formatNum, fmtDuration, fmtPercent, fmtTime } from './utils.js';

export const AppHeader = {
    props: ['active'],
    data() {
        return {
            lang: getLang(),
            items: [
                { key: 'home', labelKey: 'home', href: '/dashboard' },
                { key: 'sessions', labelKey: 'sessions', href: '/dashboard/sessions' },
                { key: 'models', labelKey: 'models', href: '/dashboard/models' },
                { key: 'providers', labelKey: 'providers', fallback: 'Providers', href: '/dashboard/providers' },
                { key: 'failures', labelKey: 'failures', fallback: 'Failures', href: '/dashboard/failures' }
            ]
        };
    },
    computed: {
        switchLabel() {
            return t('switchLang');
        }
    },
    methods: {
        label(item) {
            return t(item.labelKey) === item.labelKey ? (item.fallback || item.labelKey) : t(item.labelKey);
        },
        toggleLang() {
            const nextLang = this.lang === 'en' ? 'zh' : 'en';
            setLang(nextLang);
            document.documentElement.lang = nextLang;
            window.location.reload();
        }
    },
    template: `
        <nav>
            <span class="brand">AI Router</span>
            <a v-for="item in items" :key="item.key" :href="item.href" :class="{ active: item.key === active }">{{ label(item) }}</a>
            <span class="nav-spacer"></span>
            <button id="languageToggle" class="lang-toggle" type="button" @click="toggleLang">{{ switchLabel }}</button>
        </nav>
    `
};

export const TimeRangeSelect = {
    props: ['modelValue', 'label', 'selectId', 'ariaLabel'],
    emits: ['update:modelValue', 'change'],
    data() {
        return { ranges: ['24h', '7d', '30d', '90d'] };
    },
    template: `
        <div class="analytics-toolbar">
            <label :for="selectId">{{ label }}</label>
            <select :id="selectId" :aria-label="ariaLabel" :value="modelValue" @change="$emit('update:modelValue', $event.target.value); $emit('change')">
                <option v-for="range in ranges" :key="range" :value="range">{{ range }}</option>
            </select>
        </div>
    `
};

export const StatCard = {
    props: ['icon', 'label', 'value', 'iconStyle'],
    template: `
        <div class="stat">
            <div class="icon" :style="iconStyle">{{ icon }}</div>
            <div>
                <div class="label">{{ label }}</div>
                <div class="value">{{ value }}</div>
            </div>
        </div>
    `
};

export const MetricItem = {
    props: ['label', 'value', 'description'],
    template: `
        <div class="metric-item">
            <div class="label">{{ label }}</div>
            <div class="value">{{ value }}</div>
            <div class="stat-desc">{{ description }}</div>
        </div>
    `
};

export const MetricPanel = {
    props: ['title', 'metrics'],
    components: { MetricItem },
    template: `
        <section class="metric-panel">
            <div class="metric-panel-title">{{ title }}</div>
            <div class="metric-grid">
                <metric-item v-for="metric in metrics" :key="metric.key || metric.label" :label="metric.label" :value="metric.value" :description="metric.description"></metric-item>
                <slot></slot>
            </div>
        </section>
    `
};

export const CardSection = {
    props: ['title'],
    template: `
        <div class="card">
            <h2 v-if="title">{{ title }}</h2>
            <slot></slot>
        </div>
    `
};

export const UsageList = {
    props: ['rows', 'labelKey', 'state', 'emptyText', 'tokensText', 'valueKey', 'valueSuffix'],
    computed: {
        visibleRows() {
            return Array.isArray(this.rows) ? this.rows.slice(0, 10) : [];
        },
        metricKey() {
            return this.valueKey || 'total_tokens';
        },
        maxValue() {
            return Math.max(...this.visibleRows.map(item => item[this.metricKey] || 0), 1);
        }
    },
    methods: {
        formatNum,
        width(item) {
            return `${Math.max(2, Math.round((item[this.metricKey] || 0) / this.maxValue * 100))}%`;
        },
        label(item) {
            return item[this.labelKey] || '';
        },
        suffix() {
            return this.valueSuffix || this.tokensText || '';
        }
    },
    template: `
        <div class="usage-list">
            <div v-if="state" class="empty-state">{{ state }}</div>
            <div v-else-if="!visibleRows.length" class="empty-state">{{ emptyText }}</div>
            <div v-else v-for="item in visibleRows" :key="label(item)" class="usage-row">
                <div class="usage-meta">
                    <span class="usage-name">{{ label(item) }}</span>
                    <span class="usage-value">{{ formatNum(item[metricKey] || 0) }} {{ suffix() }}</span>
                </div>
                <div class="usage-bar"><div class="usage-fill" :style="{ width: width(item) }"></div></div>
            </div>
        </div>
    `
};

export const StackedTokenBarChart = {
    props: ['rows', 'labelKey', 'valueKey', 'state', 'emptyText', 'valueSuffix'],
    data() {
        return { colors: ['#4f46e5', '#06b6d4', '#10a37f', '#d97706', '#db2777', '#7c3aed', '#0891b2', '#65a30d', '#ea580c', '#475569'] };
    },
    computed: {
        metricKey() {
            return this.valueKey || 'total_tokens';
        },
        visibleRows() {
            return Array.isArray(this.rows) ? this.rows.filter(row => (row[this.metricKey] || 0) > 0).slice(0, 10) : [];
        },
        total() {
            return this.visibleRows.reduce((sum, row) => sum + (row[this.metricKey] || 0), 0);
        }
    },
    methods: {
        formatNum,
        label(row) {
            return row[this.labelKey] || '';
        },
        color(index) {
            return this.colors[index % this.colors.length];
        },
        width(row) {
            return `${this.total ? ((row[this.metricKey] || 0) / this.total * 100).toFixed(2) : 0}%`;
        },
        share(row) {
            return `${this.total ? ((row[this.metricKey] || 0) / this.total * 100).toFixed(1) : '0.0'}%`;
        },
        suffix() {
            return this.valueSuffix || 'tokens';
        }
    },
    template: `
        <div class="stacked-token-chart">
            <div v-if="state" class="empty-state">{{ state }}</div>
            <div v-else-if="!visibleRows.length" class="empty-state">{{ emptyText }}</div>
            <template v-else>
                <div class="stacked-token-bar" aria-label="Token distribution">
                    <div v-for="(row, index) in visibleRows" :key="label(row)" class="stacked-token-segment" :title="label(row) + ' · ' + formatNum(row[metricKey] || 0) + ' ' + suffix() + ' · ' + share(row)" :style="{ width: width(row), background: color(index) }"></div>
                </div>
                <div class="stacked-token-legend">
                    <div v-for="(row, index) in visibleRows" :key="label(row)" class="stacked-token-row">
                        <span class="stacked-token-swatch" :style="{ background: color(index) }"></span>
                        <span class="stacked-token-name" :title="label(row)">{{ label(row) }}</span>
                        <span class="stacked-token-value">{{ formatNum(row[metricKey] || 0) }} {{ suffix() }}</span>
                        <span class="stacked-token-share">{{ share(row) }}</span>
                    </div>
                </div>
            </template>
        </div>
    `
};

export const RecentFailuresTable = {
    props: ['rows', 'emptyText'],
    methods: { fmtTime },
    template: `
        <div>
            <div v-if="!rows.length" class="empty-state">{{ emptyText }}</div>
            <table v-else>
                <thead><tr><th>Time</th><th>Provider</th><th>Model</th><th>Error</th><th></th></tr></thead>
                <tbody>
                    <tr v-for="row in rows" :key="row.id">
                        <td>{{ fmtTime(row.started_at) }}</td>
                        <td>{{ row.provider_name }}</td>
                        <td>{{ row.model }}</td>
                        <td>{{ row.error_message || '' }}</td>
                        <td><a :href="'/dashboard/sessions/' + row.id">View</a></td>
                    </tr>
                </tbody>
            </table>
        </div>
    `
};

export const SessionTable = {
    props: ['sessions'],
    methods: { formatNum, fmtDuration, fmtTime },
    template: `
        <table>
            <thead>
                <tr><th>Time</th><th>Provider</th><th>Model</th><th>Tokens</th><th>Duration</th><th>Status</th><th></th></tr>
            </thead>
            <tbody>
                <tr v-if="sessions === null"><td colspan="7">Loading...</td></tr>
                <tr v-else-if="!sessions.length"><td colspan="7">No sessions yet.</td></tr>
                <tr v-for="session in sessions" :key="session.id">
                    <td>{{ fmtTime(session.started_at) }}</td>
                    <td>{{ session.provider_name }}</td>
                    <td>{{ session.model }}</td>
                    <td>{{ formatNum((session.prompt_tokens || 0) + (session.completion_tokens || 0)) }}</td>
                    <td>{{ fmtDuration(session.duration_ms || 0) }}</td>
                    <td><span :class="['badge', 'badge-' + session.status]">{{ session.status }}</span></td>
                    <td><a :href="'/dashboard/sessions/' + session.id">View</a></td>
                </tr>
            </tbody>
        </table>
    `
};

export const ProviderAnalyticsRows = {
    props: ['rows'],
    methods: { formatNum, fmtPercent, fmtDuration },
    template: `
        <div class="provider-analytics-grid">
            <div v-if="!rows.length" class="empty-state">No provider usage yet.</div>
            <div v-for="row in rows" :key="row.provider_name" class="provider-analytics-row">
                <div class="provider-analytics-name">{{ row.provider_name }}</div>
                <div class="provider-analytics-metric"><div class="provider-analytics-label">Requests</div><div class="provider-analytics-value">{{ formatNum(row.request_count || 0) }}</div></div>
                <div class="provider-analytics-metric"><div class="provider-analytics-label">Tokens</div><div class="provider-analytics-value">{{ formatNum(row.total_tokens || 0) }}</div></div>
                <div class="provider-analytics-metric"><div class="provider-analytics-label">Success Rate</div><div class="provider-analytics-value">{{ fmtPercent(row.success_rate || 0) }}</div></div>
                <div class="provider-analytics-metric"><div class="provider-analytics-label">Avg Latency</div><div class="provider-analytics-value">{{ fmtDuration(row.avg_duration_ms || 0) }}</div></div>
                <div class="provider-analytics-metric"><div class="provider-analytics-label">Errors</div><div class="provider-analytics-value">{{ formatNum(row.error_count || 0) }}</div></div>
            </div>
        </div>
    `
};

export const ProviderManagementTable = {
    props: ['rows', 'configPath'],
    emits: ['toggle'],
    methods: {
        tags(provider) {
            const tags = [];
            if (provider.built_in) tags.push({ label: 'built-in', className: 'badge-success' });
            if (provider.configured) tags.push({ label: 'configured', className: 'badge-success' });
            if (provider.hasGw) tags.push({ label: 'gateway upstream', className: 'badge-success' });
            if (!tags.length) tags.push({ label: 'unconfigured', className: 'badge-pending' });
            return tags;
        }
    },
    template: `
        <div>
            <div v-if="configPath" class="config-path">writes to: {{ configPath }}</div>
            <div v-if="!rows.length" class="empty-state">No providers available.</div>
            <table v-else>
                <thead><tr><th>Provider</th><th>Status</th><th>Gateway forwarding</th></tr></thead>
                <tbody>
                    <tr v-for="provider in rows" :key="provider.source + ':' + provider.id">
                        <td><strong>{{ provider.name }}</strong><span v-if="provider.source === 'gateway'" class="row-source">gateway-only</span></td>
                        <td><span v-for="tag in tags(provider)" :key="tag.label" :class="['badge', tag.className]">{{ tag.label }}</span></td>
                        <td>
                            <div v-if="provider.source === 'opencode'" class="gw-cell">
                                <label><input type="checkbox" role="switch" class="gw-switch" :checked="provider.gateway_enabled" :disabled="provider.updating" @change="$emit('toggle', provider, $event.target.checked)"></label>
                                <span :class="['gw-status', { on: provider.gateway_enabled }]">{{ provider.statusText || (provider.gateway_enabled ? 'On' : 'Off') }}</span>
                            </div>
                            <span v-else class="empty-state" style="padding:0;font-size:13px;">— gateway upstream</span>
                        </td>
                    </tr>
                </tbody>
            </table>
        </div>
    `
};

export { formatNum, fmtDuration, fmtPercent, fmtTime };
