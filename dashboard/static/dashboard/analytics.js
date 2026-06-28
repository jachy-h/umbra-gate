import { createApp } from 'https://unpkg.com/vue@3/dist/vue.esm-browser.prod.js';
import { formatNum, fmtDuration, fmtPercent } from './utils.js';

const chartPalette = {
    blue: '#0070f3',
    cyan: '#50e3c2',
    violet: '#7928ca',
    pink: '#ff0080',
    amber: '#f5a623',
    red: '#ee0000',
    ink: '#171717',
    hairline: '#ebebeb',
    text: '#4d4d4d',
    mute: '#888888',
    surface: '#ffffff'
};
const colors = [chartPalette.blue, chartPalette.cyan, chartPalette.violet, chartPalette.pink, chartPalette.amber, chartPalette.red, chartPalette.ink];
const chartColors = {
    tokens: { border: chartPalette.blue, background: 'rgba(0, 112, 243, 0.18)' },
    requests: { border: chartPalette.ink, background: 'rgba(23, 23, 23, 0.12)' },
    success: { border: chartPalette.cyan, background: 'rgba(80, 227, 194, 0.26)' },
    errors: { border: chartPalette.red, background: 'rgba(238, 0, 0, 0.16)' }
};

const charts = {
    tokens: { key: 'tokens', title: 'Tokens', type: 'bar', metric: 'total_tokens', formatter: formatNum },
    requests: { key: 'requests', title: 'Requests', type: 'bar', metric: 'request_count', formatter: formatNum },
    success: { key: 'success', title: 'Success Rate', type: 'bar', metric: 'success_rate', formatter: fmtPercent },
    errors: { key: 'errors', title: 'Errors', type: 'bar', metric: 'error_count', formatter: formatNum },
    share: { key: 'share', title: 'Usage Share', type: 'doughnut', metric: 'total_tokens', formatter: formatNum }
};

const sections = [
    { key: 'agent', label: 'Agent', title: 'By Agent', charts: [charts.tokens, charts.requests] },
    { key: 'provider', label: 'Provider', title: 'By Provider', charts: [charts.share, charts.success, charts.errors] },
    { key: 'model', label: 'Model', title: 'By Model', charts: [charts.share, charts.tokens, charts.success] },
    { key: 'project', label: 'Project', title: 'By Project', charts: [charts.tokens, charts.requests] },
    { key: 'endpoint', label: 'Endpoint', title: 'By Endpoint', charts: [charts.requests, charts.errors] },
    { key: 'status', label: 'Status', title: 'By Status', charts: [charts.requests, charts.share] }
];

createApp({
    data() {
        return {
            range: '7d',
            sections,
            overview: {},
            breakdowns: Object.fromEntries(sections.map(section => [section.key, []])),
            loading: Object.fromEntries(sections.map(section => [section.key, true])),
            chartInstances: {},
            activeSection: 'overview'
        };
    },
    mounted() {
        document.documentElement.classList.add('vue-ready');
        this.loadAll();
        this.$nextTick(() => this.updateActiveSection());
    },
    methods: {

        formatNum,
        fmtDuration,
        fmtPercent,
        chartID(sectionKey, chartKey) {
            return `analytics-${sectionKey}-${chartKey}`;
        },
        rows(key) {
            return this.breakdowns[key] || [];
        },
        chartRows(key, metric) {
            return this.rows(key).filter(row => (row[metric] || 0) > 0).slice(0, 10);
        },
        async loadAll() {
            await Promise.all([
                this.loadOverview(),
                ...this.sections.map(section => this.loadBreakdown(section.key))
            ]);
        },
        async loadOverview() {
            const response = await fetch(`/api/analytics/overview?range=${encodeURIComponent(this.range)}`);
            this.overview = response.ok ? await response.json() : {};
        },
        async loadBreakdown(key) {
            this.loading[key] = true;
            try {
                const response = await fetch(`/api/analytics/breakdown?range=${encodeURIComponent(this.range)}&dimension=${encodeURIComponent(key)}`);
                this.breakdowns[key] = response.ok ? await response.json() : [];
            } finally {
                this.loading[key] = false;
                this.$nextTick(() => this.renderSectionCharts(key));
            }
        },
        updateActiveSection() {
            const container = this.$refs.analyticsContent;
            if (!container) return;
            const containerRect = container.getBoundingClientRect();
            const ids = ['overview', ...this.sections.map(section => section.key)];
            let nextActive = this.activeSection;
            let maxVisible = 0;
            ids.forEach(id => {
                const section = document.getElementById(id);
                if (!section) return;
                const rect = section.getBoundingClientRect();
                const visible = Math.max(0, Math.min(rect.bottom, containerRect.bottom) - Math.max(rect.top, containerRect.top));
                if (visible > maxVisible) {
                    maxVisible = visible;
                    nextActive = id;
                }
            });
            this.activeSection = nextActive;
        },
        renderSectionCharts(key) {
            if (!window.Chart) return;
            const section = this.sections.find(item => item.key === key);
            if (!section) return;
            section.charts.forEach(chart => this.renderChart(section, chart));
        },
        renderChart(section, chart) {
            const rows = this.chartRows(section.key, chart.metric);
            const id = this.chartID(section.key, chart.key);
            const canvas = document.getElementById(id);
            if (!canvas) return;
            if (this.chartInstances[id]) this.chartInstances[id].destroy();
            if (!rows.length) return;
            const data = rows.map(row => row[chart.metric] || 0);
            const labels = rows.map(row => row.name || 'unknown');
            const barColor = chartColors[chart.key] || chartColors.tokens;
            this.chartInstances[id] = new Chart(canvas.getContext('2d'), {
                type: chart.type,
                data: {
                    labels,
                    datasets: [{
                        label: chart.title,
                        data,
                        backgroundColor: chart.type === 'doughnut' ? labels.map((_, index) => colors[index % colors.length]) : barColor.background,
                        borderColor: chart.type === 'doughnut' ? chartPalette.surface : barColor.border,
                        borderWidth: chart.type === 'doughnut' ? 2 : 1,
                        borderRadius: chart.type === 'doughnut' ? 0 : 6,
                        hoverBackgroundColor: chart.type === 'doughnut' ? labels.map((_, index) => colors[index % colors.length]) : barColor.border
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: {
                        legend: { display: chart.type === 'doughnut', position: 'bottom', labels: { color: chartPalette.text, boxWidth: 10, boxHeight: 10 } },
                        tooltip: { callbacks: { label: context => `${context.label}: ${chart.formatter(context.parsed.y ?? context.parsed)}` } }
                    },
                    scales: chart.type === 'doughnut' ? {} : {
                        x: { grid: { display: false }, ticks: { color: chartPalette.mute } },
                        y: { beginAtZero: true, grid: { color: chartPalette.hairline }, ticks: { color: chartPalette.mute, callback: value => chart.formatter(value) } }
                    }
                }
            });
        }
    }
}).mount('#analyticsApp');
