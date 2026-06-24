import { createApp } from 'https://unpkg.com/vue@3/dist/vue.esm-browser.prod.js';
import { CardSection, SessionTable } from './components.js';

createApp({
    components: { CardSection, SessionTable },
    data() {
        return {
            sessions: null,
            page: 1,
            pageSize: 50,
            hasNext: false,
            loading: false
        };
    },
    computed: {
        offset() {
            return (this.page - 1) * this.pageSize;
        },
        pageSummary() {
            const start = this.sessions && this.sessions.length ? this.offset + 1 : 0;
            const end = this.offset + (this.sessions ? this.sessions.length : 0);
            return `Page ${this.page} · ${start}-${end}`;
        }
    },
    async mounted() {
        document.documentElement.classList.add('vue-ready');
        await this.loadSessions();
    },
    methods: {
        async loadSessions() {
            this.loading = true;
            this.sessions = null;
            const limit = this.pageSize + 1;
            const response = await fetch(`/api/sessions?limit=${limit}&offset=${this.offset}`);
            const rows = await response.json();
            this.hasNext = rows.length > this.pageSize;
            this.sessions = rows.slice(0, this.pageSize);
            this.loading = false;
        },
        async previousPage() {
            if (this.page <= 1 || this.loading) return;
            this.page--;
            await this.loadSessions();
        },
        async nextPage() {
            if (!this.hasNext || this.loading) return;
            this.page++;
            await this.loadSessions();
        },
        async changePageSize(event) {
            this.pageSize = Number(event.target.value);
            this.page = 1;
            await this.loadSessions();
        }
    }
}).mount('#sessionsApp');
