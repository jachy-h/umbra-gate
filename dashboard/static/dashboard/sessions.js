import { createApp } from 'https://unpkg.com/vue@3/dist/vue.esm-browser.prod.js';
import { CardSection, SessionTable } from './components.js';

createApp({
    components: { CardSection, SessionTable },
    data() {
        return { sessions: null };
    },
    async mounted() {
        document.documentElement.classList.add('vue-ready');
        const response = await fetch('/api/sessions');
        this.sessions = await response.json();
    }
}).mount('#sessionsApp');
