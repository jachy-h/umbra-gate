import { createApp } from 'https://unpkg.com/vue@3/dist/vue.esm-browser.prod.js';
import { getLang, t } from './i18n.js';
import { CardSection, MetricPanel, StackedTokenBarChart, TimeRangeSelect, formatNum, fmtDuration, fmtPercent } from './components.js';

createApp({
    components: { CardSection, MetricPanel, StackedTokenBarChart, TimeRangeSelect },
    data() {
        return {
            range: '7d',
            overview: null,
            providers: [],
            models: [],
            providersState: t('loading'),
            modelsState: t('loading'),
            trendState: '',
            chart: null,
            stats: window.dashboardStats || {}
        };
    },
    computed: {
        languageToggle() { return t('switchLang'); },
        performanceMetrics() {
            return [
                { key: 'successRate', label: t('successRate'), value: this.overview ? fmtPercent(this.overview.success_rate || 0) : '—', description: t('successRateDesc') },
                { key: 'errors', label: t('errors'), value: this.overview ? formatNum(this.overview.error_count || 0) : '—', description: t('errorsDesc') },
                { key: 'avgLatency', label: t('avgLatency'), value: this.overview ? fmtDuration(this.overview.avg_duration_ms || 0) : '—', description: t('avgLatencyDesc') },
                { key: 'p95Latency', label: t('p95Latency'), value: this.overview ? fmtDuration(this.overview.p95_duration_ms || 0) : '—', description: t('p95LatencyDesc') }
            ];
        },
        usageMetrics() {
            return [
                { key: 'todayRequests', label: t('todayRequests'), value: this.stats.todayRequests || 0, description: t('todayRequestsDesc') },
                { key: 'todayTokens', label: t('todayTokens'), value: formatNum(this.stats.todayTokens || 0), description: t('todayTokensDesc') },
                { key: 'monthRequests', label: t('monthRequests'), value: this.stats.monthRequests || 0, description: t('monthRequestsDesc') },
                { key: 'monthTokens', label: t('monthTokens'), value: formatNum(this.stats.monthTokens || 0), description: t('monthTokensDesc') }
            ];
        }
    },
    mounted() {
        document.documentElement.classList.add('vue-ready');
        document.documentElement.lang = getLang();
        this.loadOverview();
        this.loadUsage('/api/providers', 'providers', 'providersState');
        this.loadUsage('/api/models', 'models', 'modelsState');
        this.loadTrend();
    },
    methods: {
        t,
        async loadOverview() {
            try {
                const response = await fetch(`/api/overview?range=${encodeURIComponent(this.range)}`);
                if (!response.ok) throw new Error('request failed');
                this.overview = await response.json();
            } catch {
                this.overview = null;
            }
        },
        async loadUsage(url, rowsKey, stateKey) {
            this[stateKey] = t('loading');
            try {
                const response = await fetch(url);
                if (!response.ok) throw new Error('request failed');
                this[rowsKey] = await response.json();
                this[stateKey] = '';
            } catch {
                this[stateKey] = t('failedToLoadUsage');
            }
        },
        async loadTrend() {
            this.trendState = '';
            if (!window.Chart) {
                this.trendState = t('chartUnavailable');
                return;
            }
            try {
                const response = await fetch(`/api/timeseries?range=${encodeURIComponent(this.range)}`);
                if (!response.ok) throw new Error('request failed');
                this.renderTrend(await response.json());
            } catch {
                this.trendState = t('failedToLoadUsage');
            }
        },
        renderTrend(data) {
            if (!Array.isArray(data) || data.length === 0) {
                this.trendState = t('noUsageYet');
                return;
            }
            const ctx = document.getElementById('usageTrendChart').getContext('2d');
            if (this.chart) this.chart.destroy();
            this.chart = new Chart(ctx, {
                data: {
                    labels: data.map(item => item.date),
                    datasets: [
                        { type: 'bar', label: t('requests'), data: data.map(item => item.request_count), backgroundColor: 'rgba(79, 70, 229, 0.25)', borderColor: '#4f46e5', yAxisID: 'requests' },
                        { type: 'line', label: 'Tokens (M)', data: data.map(item => (item.total_tokens || 0) / 1000000), borderColor: '#06b6d4', backgroundColor: 'rgba(6, 182, 212, 0.12)', tension: 0.35, yAxisID: 'tokens' }
                    ]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    interaction: { mode: 'index', intersect: false },
                    scales: {
                        tokens: { type: 'linear', position: 'left', beginAtZero: true, title: { display: true, text: 'Tokens (M)' }, ticks: { callback: v => `${v}M` } },
                        requests: { type: 'linear', position: 'right', beginAtZero: true, grid: { drawOnChartArea: false } }
                    }
                }
            });
        },
        refreshAnalytics() {
            this.loadOverview();
            this.loadTrend();
        }
    }
}).mount('#dashboardApp');
