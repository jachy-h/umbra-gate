import { createApp } from 'https://unpkg.com/vue@3/dist/vue.esm-browser.prod.js';
import { CardSection, RecentFailuresTable, StatCard, TimeRangeSelect, UsageList, formatNum } from './components.js';

createApp({
    components: { CardSection, RecentFailuresTable, StatCard, TimeRangeSelect, UsageList },
    data() {
        return {
            range: '7d',
            data: null,
            state: 'Loading...',
            error: ''
        };
    },
    computed: {
        categories() { return this.data?.categories || []; },
        providers() { return this.data?.providers || []; },
        models() { return this.data?.models || []; },
        recent() { return this.data?.recent || []; },
        stats() {
            return [
                { icon: '!', label: 'Total Failures', value: formatNum(this.data?.total_failures || 0) },
                { icon: '#', label: 'Categories', value: formatNum(this.categories.length) },
                { icon: 'P', label: 'Providers', value: formatNum(this.providers.length) },
                { icon: 'M', label: 'Models', value: formatNum(this.models.length) }
            ];
        }
    },
    mounted() {
        document.documentElement.classList.add('vue-ready');
        this.loadFailures();
    },
    methods: {
        async loadFailures() {
            this.error = '';
            this.state = 'Loading...';
            try {
                const response = await fetch(`/api/failures?range=${encodeURIComponent(this.range)}`);
                if (!response.ok) throw new Error('request failed');
                this.data = await response.json();
                this.state = '';
            } catch {
                this.error = 'Failed to load failures.';
                this.state = '';
            }
        }
    }
}).mount('#failuresApp');
