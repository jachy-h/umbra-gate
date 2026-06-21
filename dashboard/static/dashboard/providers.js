import { createApp } from 'https://unpkg.com/vue@3/dist/vue.esm-browser.prod.js';
import { CodexManagementTable, ProviderAnalyticsRows, ProviderManagementTable } from './components.js';

createApp({
    components: { CodexManagementTable, ProviderAnalyticsRows, ProviderManagementTable },
    data() {
        return {
            range: '7d',
            configPath: '',
            codexConfigPath: '',
            providerRows: [],
            codexRows: [],
            analyticsRows: [],
            status: '',
            statusError: false,
            tokenChart: null,
            successChart: null
        };
    },
    mounted() {
        document.documentElement.classList.add('vue-ready');
        this.loadProviderAnalytics();
        this.loadData();
        this.loadCodexData();
    },
    methods: {
        buildRows(openRows, gwRows) {
            const combined = [];
            openRows.forEach(provider => {
                combined.push({ id: provider.id, name: provider.name, source: 'opencode', built_in: provider.built_in, configured: provider.configured, gateway_enabled: provider.gateway_enabled, hasGw: false, gwType: '', gwBaseUrl: '' });
            });
            gwRows.forEach(provider => {
                const existing = combined.find(row => row.id.toLowerCase() === provider.id.toLowerCase());
                if (existing) {
                    existing.hasGw = true;
                    existing.gwType = provider.type;
                    existing.gwBaseUrl = provider.base_url;
                } else {
                    combined.push({ id: provider.id, name: provider.id, source: 'gateway', type: provider.type, built_in: false, configured: false, gateway_enabled: false, hasGw: true, gwType: provider.type, gwBaseUrl: provider.base_url });
                }
            });
            return combined;
        },
        setStatus(text, isError = false) {
            this.status = text || '';
            this.statusError = isError;
            if (text && !isError) setTimeout(() => { if (this.status === text) this.status = ''; }, 3000);
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
        async loadData() {
            try {
                const [cfgData, gwData] = await Promise.all([
                    fetch('/dashboard/providers/config').then(response => response.json()),
                    fetch('/api/gateway/providers').then(response => response.json())
                ]);
                const files = cfgData.files || [];
                const selected = files.find(file => file.selected) || files[0];
                this.configPath = selected ? selected.path : '';
                this.providerRows = this.buildRows(cfgData.providers || [], Array.isArray(gwData) ? gwData : []);
            } catch {
                this.providerRows = [];
                this.setStatus('Failed to load providers.', true);
            }
        },
        async loadCodexData() {
            try {
                const data = await fetch('/dashboard/codex/config').then(response => response.json());
                const files = data.files || [];
                const selected = files.find(file => file.selected) || files[0];
                this.codexConfigPath = selected ? selected.path : '';
                this.codexRows = data.providers || [];
            } catch {
                this.codexRows = [];
            }
        },
        async onToggle(provider, enabled) {
            const previous = provider.gateway_enabled;
            provider.updating = true;
            provider.statusText = enabled ? 'Enabling...' : 'Disabling...';
            provider.gateway_enabled = enabled;
            try {
                const response = await fetch('/dashboard/providers/gateway', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ id: provider.id, enabled, path: this.configPath })
                });
                if (!response.ok) {
                    const data = await response.json();
                    throw new Error(data.error || 'failed');
                }
                provider.statusText = enabled ? 'On' : 'Off';
                this.setStatus(`Gateway ${enabled ? 'enabled' : 'disabled'} for ${provider.id}.`);
            } catch (error) {
                provider.gateway_enabled = previous;
                provider.statusText = previous ? 'On' : 'Off';
                this.setStatus(error.message || 'Failed to update.', true);
            } finally {
                provider.updating = false;
            }
        },
        async onCodexToggle(provider, enabled) {
            const previous = provider.gateway_enabled;
            provider.updating = true;
            provider.statusText = enabled ? 'Enabling...' : 'Disabling...';
            provider.gateway_enabled = enabled;
            try {
                const response = await fetch('/dashboard/codex/gateway', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ id: provider.id, enabled, path: this.codexConfigPath })
                });
                if (!response.ok) {
                    const data = await response.json();
                    throw new Error(data.error || 'failed');
                }
                provider.statusText = enabled ? 'On' : 'Off';
                this.setStatus(`Codex gateway ${enabled ? 'enabled' : 'disabled'} for ${provider.id}.`);
                await this.loadData();
            } catch (error) {
                provider.gateway_enabled = previous;
                provider.statusText = previous ? 'On' : 'Off';
                this.setStatus(error.message || 'Failed to update Codex.', true);
            } finally {
                provider.updating = false;
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
