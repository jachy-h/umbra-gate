import { createApp } from 'https://unpkg.com/vue@3/dist/vue.esm-browser.prod.js';
import { AppHeader } from './components.js';

createApp({
    components: { AppHeader },
    data() {
        return { active: document.getElementById('appHeader')?.dataset.active || '' };
    },
    template: `<app-header :active="active"></app-header>`
}).mount('#appHeader');
