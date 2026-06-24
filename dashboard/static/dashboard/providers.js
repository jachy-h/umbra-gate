import { createApp } from 'https://unpkg.com/vue@3/dist/vue.esm-browser.prod.js';
import { ProviderAnalyticsRows } from './components.js';

const emptyForm = () => ({ id: '', base_url: '', api_key: '' });

createApp({
    components: { ProviderAnalyticsRows },
    data() {
        return {
            range: '7d',
            providerRows: null,
            analyticsRows: [],
            form: emptyForm(),
            editingExisting: false,
            saving: false,
            status: '',
            statusError: false,
            tokenChart: null,
            successChart: null
        };
    },
    mounted() {
        document.documentElement.classList.add('vue-ready');
        this.loadProviderAnalytics();
        this.loadProviders();
    },
    methods: {
        setStatus(text, isError = false) {
            this.status = text || '';
            this.statusError = isError;
            if (text && !isError) setTimeout(() => { if (this.status === text) this.status = ''; }, 3000);
        },
        async loadProviders() {
            try {
                const response = await fetch('/api/gateway/providers');
                if (!response.ok) throw new Error('failed to load providers');
                this.providerRows = await response.json();
            } catch (error) {
                this.providerRows = [];
                this.setStatus(error.message || 'Failed to load providers.', true);
            }
        },
        newProvider() {
            this.resetForm();
        },
        editProvider(provider) {
            this.editingExisting = true;
            this.form = {
                id: provider.id,
                base_url: provider.base_url || '',
                api_key: ''
            };
        },
        resetForm(clearStatus = true) {
            this.editingExisting = false;
            this.form = emptyForm();
            if (clearStatus) this.setStatus('');
        },
        async saveProvider() {
            this.saving = true;
            try {
                const payload = {
                    id: this.form.id,
                    base_url: this.form.base_url,
                    api_key: this.form.api_key
                };
                const url = this.editingExisting ? `/api/gateway/providers/${encodeURIComponent(this.form.id)}` : '/api/gateway/providers';
                const method = this.editingExisting ? 'PUT' : 'POST';
                const response = await fetch(url, {
                    method,
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(payload)
                });
                if (!response.ok) {
                    const data = await response.json().catch(() => ({}));
                    throw new Error(data.error || 'failed to save provider');
                }
                this.setStatus(`Provider ${this.form.id} saved.`);
                this.resetForm(false);
                await this.loadProviders();
            } catch (error) {
                this.setStatus(error.message || 'Failed to save provider.', true);
            } finally {
                this.saving = false;
            }
        },
        async deleteProvider(provider) {
            try {
                const response = await fetch(`/api/gateway/providers/${encodeURIComponent(provider.id)}`, { method: 'DELETE' });
                if (!response.ok) {
                    const data = await response.json().catch(() => ({}));
                    throw new Error(data.error || 'failed to delete provider');
                }
                this.setStatus(`Provider ${provider.id} deleted.`);
                if (this.form.id === provider.id) this.resetForm();
                await this.loadProviders();
            } catch (error) {
                this.setStatus(error.message || 'Failed to delete provider.', true);
            }
        },
        async loadProviderAnalytics() {
            try {
                const response = await fetch(`/api/providers/analytics?range=${encodeURIComponent(this.range)}`);
                if (!response.ok) throw new Error('request failed');
                this.analyticsRows = await response.json();
                this.$nextTick(() => this.renderCharts());
            } catch {
                this.analyticsRows = [];
            }
        },
        renderCharts() {
            if (!window.Chart || !this.analyticsRows.length) return;
            const tokenCtx = document.getElementById('providerTokenChart')?.getContext('2d');
            const successCtx = document.getElementById('providerSuccessChart')?.getContext('2d');
            if (!tokenCtx || !successCtx) return;
            if (this.tokenChart) this.tokenChart.destroy();
            if (this.successChart) this.successChart.destroy();
            this.tokenChart = new Chart(tokenCtx, {
                type: 'doughnut',
                data: { labels: this.analyticsRows.map(row => row.provider_name), datasets: [{ data: this.analyticsRows.map(row => row.total_tokens || 0), backgroundColor: ['#4f46e5','#06b6d4','#10a37f','#d97757','#4285f4','#0866ff','#ff7000','#615ced'], borderColor:'#fff', borderWidth:2 }] },
                options: { responsive:true, maintainAspectRatio:false, cutout:'58%', plugins:{ legend:{position:'bottom',labels:{boxWidth:10,font:{size:11},padding:12}} } }
            });
            this.successChart = new Chart(successCtx, {
                type: 'bar',
                data: { labels: this.analyticsRows.map(row => row.provider_name), datasets: [{ label:'Success', data:this.analyticsRows.map(row => row.success_count || 0), backgroundColor:'#10a37f' }, { label:'Errors', data:this.analyticsRows.map(row => row.error_count || 0), backgroundColor:'#ef4444' }] },
                options: { responsive:true, maintainAspectRatio:false, scales:{ x:{stacked:true}, y:{stacked:true,beginAtZero:true,title:{display:true,text:'Requests'}} }, plugins:{ legend:{position:'bottom',labels:{boxWidth:10,font:{size:11},padding:8}} } }
            });
        }
    }
}).mount('#providersApp');
