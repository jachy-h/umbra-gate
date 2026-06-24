import { createApp } from 'https://unpkg.com/vue@3/dist/vue.esm-browser.prod.js';

createApp({
    data() {
        return {
            agents: [],
            activeAgentId: '',
            loading: true,
            statusMessage: '',
            statusError: false
        };
    },
    mounted() {
        document.documentElement.classList.add('vue-ready');
        this.loadAgents();
    },
    computed: {
        activeAgent() {
            return this.agents.find(agent => agent.agent_id === this.activeAgentId) || this.agents[0] || null;
        }
    },
    methods: {
        async loadAgents() {
            this.loading = true;
            try {
                const response = await fetch('/api/agents');
                if (!response.ok) throw new Error('failed to load agents');
                this.agents = await response.json();
                if (!this.agents.some(agent => agent.agent_id === this.activeAgentId)) {
                    this.activeAgentId = this.agents[0]?.agent_id || '';
                }
            } catch (error) {
                this.status(error.message || 'Failed to load agents.', true);
                this.agents = [];
                this.activeAgentId = '';
            } finally {
                this.loading = false;
            }
        },
        agentGatewayEnabled(agent) {
            return Array.isArray(agent.bindings) && agent.bindings.some(binding => binding.gateway_enabled);
        },
        replaceAgent(updatedAgent) {
            const index = this.agents.findIndex(agent => agent.agent_id === updatedAgent.agent_id);
            if (index === -1) {
                this.agents.push(updatedAgent);
                return;
            }
            this.agents.splice(index, 1, updatedAgent);
        },
        async refreshAgent(agentId) {
            const response = await fetch(`/api/agents/${encodeURIComponent(agentId)}/status`);
            if (!response.ok) throw new Error('failed to refresh agent status');
            this.replaceAgent(await response.json());
        },
        selectedConfig(agent) {
            const files = agent.config_files || [];
            const selected = files.find(file => file.selected) || files[0];
            return selected ? selected.path : '';
        },
        status(message, isError = false) {
            this.statusMessage = message;
            this.statusError = isError;
            if (message && !isError) {
                setTimeout(() => {
                    if (this.statusMessage === message) this.statusMessage = '';
                }, 3000);
            }
        },
        async toggle(agent, binding) {
            const nextEnabled = !binding.gateway_enabled;
            const previousEnabled = binding.gateway_enabled;
            const previousLiveBaseURL = binding.live_base_url;
            binding.updating = true;
            try {
                const planResponse = await fetch(`/api/agents/${encodeURIComponent(agent.agent_id)}/plan`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        provider_id: binding.provider_id,
                        enabled: nextEnabled,
                        config_path: binding.config_path
                    })
                });
                if (!planResponse.ok) throw new Error('failed to plan change');
                const plan = await planResponse.json();
                const applyResponse = await fetch(`/api/agents/${encodeURIComponent(agent.agent_id)}/apply`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        provider_id: binding.provider_id,
                        enabled: nextEnabled,
                        config_path: binding.config_path,
                        base_checksum: plan.base_checksum
                    })
                });
                if (!applyResponse.ok) {
                    const data = await applyResponse.json().catch(() => ({}));
                    throw new Error(data.error || 'failed to apply change');
                }
                binding.gateway_enabled = nextEnabled;
                binding.live_base_url = nextEnabled ? binding.gateway_base_url : '';
                this.status(`${agent.display_name} gateway ${nextEnabled ? 'enabled' : 'disabled'} for ${binding.provider_id}.`);
                await this.refreshAgent(agent.agent_id);
            } catch (error) {
                binding.gateway_enabled = previousEnabled;
                binding.live_base_url = previousLiveBaseURL;
                this.status(error.message || 'Failed to update agent.', true);
            } finally {
                binding.updating = false;
            }
        }
    }
}).mount('#agentsApp');
