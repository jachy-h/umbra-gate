<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue';

const navItems = [
  { path: '/dashboard', label: 'Overview' },
  { path: '/dashboard/agents', label: 'Agents' },
  { path: '/dashboard/providers', label: 'Providers' },
  { path: '/dashboard/models', label: 'Models' },
  { path: '/dashboard/analytics', label: 'Analytics' },
  { path: '/dashboard/failures', label: 'Failures' },
  { path: '/dashboard/sessions', label: 'Sessions' }
];
const ranges = ['24h', '7d', '30d', '90d'];
const path = ref(normalizePath(window.location.pathname));
const range = ref('7d');
const stats = ref(null);
const overview = ref(null);
const providers = ref([]);
const models = ref([]);
const modelAnalytics = ref([]);
const timeseries = ref([]);
const agents = ref([]);
const activeAgentId = ref('');
const sessions = ref([]);
const sessionPage = ref(1);
const sessionPageSize = ref(50);
const sessionHasNext = ref(false);
const sessionDetail = ref(null);
const sessionLog = ref(null);
const analyticsRows = ref([]);
const analyticsSections = ref({});
const activeAnalyticsSection = ref('provider');
const failures = ref(null);
const loading = ref(false);
const notice = ref('');
const noticeError = ref(false);
const overviewMotionOpen = ref(false);
const overviewCursorStyle = ref({ '--cursor-x': '50%', '--cursor-y': '50%' });
const dailyTrafficChartEl = ref(null);
const modelUsageChartEl = ref(null);
const providerUsageChartEl = ref(null);
let dailyTrafficChart;
let modelUsageChart;
let providerUsageChart;
const analyticsDimensions = [
  { id: 'provider', label: 'Provider' },
  { id: 'model', label: 'Model' },
  { id: 'agent', label: 'Agent' },
  { id: 'project', label: 'Project' },
  { id: 'endpoint', label: 'Endpoint' },
  { id: 'status', label: 'Status' }
];
const chartPalette = ['#171717', '#0070f3', '#00dfd8', '#7928ca', '#ff0080', '#f5a623', '#10b981', '#ef4444'];
const overviewChartColors = {
  requests: '#171717',
  requestsFill: 'rgba(23, 23, 23, 0.08)',
  tokens: '#06b6d4',
  tokensFill: 'rgba(6, 182, 212, 0.1)',
  model: '#7c3aed',
  provider: '#f97316',
  hoverLine: 'rgba(124, 58, 237, 0.22)'
};

const currentView = computed(() => {
  if (path.value.startsWith('/dashboard/sessions/')) return 'sessionDetail';
  if (path.value === '/dashboard') return 'overview';
  return path.value.replace('/dashboard/', '') || 'overview';
});
const activeAgent = computed(() => agents.value.find((agent) => agent.agent_id === activeAgentId.value) || agents.value[0]);
const dailyTrafficRows = computed(() => {
  const byDate = new Map(timeseries.value.map((row) => [row.date, row]));
  const count = { '24h': 1, '7d': 7, '30d': 30, '90d': 90 }[range.value] || 7;
  return Array.from({ length: count }, (_, index) => {
    const date = isoDateDaysAgo(count - index - 1);
    const row = byDate.get(date) || {};
    return {
      date,
      request_count: row.request_count || 0,
      total_tokens: row.total_tokens || 0
    };
  });
});
const providerTotals = computed(() => analyticsRows.value.reduce((totals, row) => ({
  requests: totals.requests + (row.request_count || 0),
  tokens: totals.tokens + (row.total_tokens || 0),
  errors: totals.errors + (row.error_count || 0)
}), { requests: 0, tokens: 0, errors: 0 }));
const modelTotals = computed(() => modelAnalytics.value.reduce((totals, row) => ({
  requests: totals.requests + (row.request_count || 0),
  tokens: totals.tokens + (row.total_tokens || 0),
  errors: totals.errors + (row.error_count || 0),
  weightedLatency: totals.weightedLatency + (row.avg_duration_ms || 0) * (row.request_count || 0)
}), { requests: 0, tokens: 0, errors: 0, weightedLatency: 0 }));
const fastestModels = computed(() => models.value
  .filter((row) => row.request_count > 0)
  .slice()
  .sort((a, b) => (a.avg_duration_ms || 0) - (b.avg_duration_ms || 0))
  .slice(0, 6));
const heaviestModels = computed(() => models.value
  .slice()
  .sort((a, b) => (b.total_tokens || 0) - (a.total_tokens || 0))
  .slice(0, 6));
const modelAvgLatency = computed(() => modelTotals.value.requests ? modelTotals.value.weightedLatency / modelTotals.value.requests : 0);

function normalizePath(value) {
  if (value === '/dashboard/') return '/dashboard';
  return value.startsWith('/dashboard') ? value : '/dashboard';
}

function go(nextPath) {
  path.value = normalizePath(nextPath);
  window.history.pushState({}, '', path.value);
  loadView();
}

function isoDateDaysAgo(days) {
  const date = new Date();
  date.setHours(0, 0, 0, 0);
  date.setDate(date.getDate() - days);
  return date.toISOString().slice(0, 10);
}

function setNotice(message, isError = false) {
  notice.value = message;
  noticeError.value = isError;
  if (message && !isError) {
    setTimeout(() => {
      if (notice.value === message) notice.value = '';
    }, 3000);
  }
}

async function api(pathname, options) {
  const response = await fetch(pathname, options);
  if (!response.ok) {
    const body = await response.json().catch(() => ({}));
    throw new Error(body.error || `request failed: ${response.status}`);
  }
  if (response.status === 204) return null;
  return response.json();
}

function formatNum(value) {
  const number = Number(value || 0);
  if (number >= 1000000) return `${trim(number / 1000000)}M`;
  if (number >= 1000) return `${trim(number / 1000)}K`;
  return `${number}`;
}

function trim(value) {
  return value.toFixed(1).replace(/\.0$/, '');
}

function pct(value) {
  return `${Math.round(Number(value || 0) * 100)}%`;
}

function duration(ms) {
  const value = Number(ms || 0);
  if (value >= 1000) return `${trim(value / 1000)}s`;
  return `${Math.round(value)}ms`;
}

function metricDigits(value) {
  return String(value ?? '—').split('').map((char, index, chars) => ({
    char,
    stagger: index === chars.length - 2 ? '1' : index === chars.length - 1 ? '2' : undefined
  }));
}

function time(t) {
  if (!t) return '—';
  return new Date(`${t}Z`).toLocaleString();
}

function replayOverviewMotion() {
  overviewMotionOpen.value = false;
  nextTick(() => {
    const page = document.querySelector('.overview-page');
    if (page) void page.offsetHeight;
    overviewMotionOpen.value = true;
  });
}

function moveOverviewCursor(event) {
  const rect = event.currentTarget.getBoundingClientRect();
  overviewCursorStyle.value = {
    '--cursor-x': `${event.clientX - rect.left}px`,
    '--cursor-y': `${event.clientY - rect.top}px`
  };
}

function resetOverviewCursor() {
  overviewCursorStyle.value = { '--cursor-x': '50%', '--cursor-y': '50%' };
}

function tiltOverviewCard(event) {
  const rect = event.currentTarget.getBoundingClientRect();
  event.currentTarget.style.setProperty('--cursor-x', `${event.clientX - rect.left}px`);
  event.currentTarget.style.setProperty('--cursor-y', `${event.clientY - rect.top}px`);
}

function resetOverviewCard(event) {
  event.currentTarget.style.removeProperty('--cursor-x');
  event.currentTarget.style.removeProperty('--cursor-y');
}

function sectionRows(id) {
  return analyticsSections.value[id] || [];
}

function chartRows(id) {
  return sectionRows(id)
    .slice()
    .sort((a, b) => (b.request_count || 0) - (a.request_count || 0))
    .slice(0, 8);
}

function chartMax(id) {
  return Math.max(...chartRows(id).map((row) => row.request_count || 0), 1);
}

function successWidth(row) {
  const success = row.success_count || 0;
  const errors = row.error_count || 0;
  const total = Math.max(success + errors, 1);
  return `${Math.round(success / total * 100)}%`;
}

function errorWidth(row) {
  const success = row.success_count || 0;
  const errors = row.error_count || 0;
  const total = Math.max(success + errors, 1);
  return `${Math.round(errors / total * 100)}%`;
}

function chartColor(index) {
  return chartPalette[index % chartPalette.length];
}

function chartGradient(index) {
  const colors = [chartColor(index), chartColor(index + 1)];
  return `linear-gradient(90deg, ${colors[0]}, ${colors[1]})`;
}

function chartDefaults() {
  return {
    responsive: true,
    maintainAspectRatio: false,
    animation: { duration: 520, easing: 'easeOutQuart' },
    plugins: {
      legend: { labels: { boxWidth: 8, boxHeight: 8, color: '#525252', font: { size: 12 } } },
      tooltip: { callbacks: { label: (context) => `${context.dataset.label}: ${formatNum(context.raw)}` } }
    },
    scales: {
      x: { grid: { display: false }, ticks: { color: '#525252', maxRotation: 0, autoSkip: true } },
      y: { beginAtZero: true, grid: { color: '#ebebeb' }, ticks: { color: '#525252', callback: (value) => formatNum(value) } }
    }
  };
}

const verticalHoverLinePlugin = {
  id: 'verticalHoverLine',
  afterDatasetsDraw(chart) {
    const active = chart.tooltip?.getActiveElements?.() || [];
    if (!active.length) return;
    const { ctx, chartArea } = chart;
    const x = active[0].element.x;
    ctx.save();
    ctx.beginPath();
    ctx.moveTo(x, chartArea.top);
    ctx.lineTo(x, chartArea.bottom);
    ctx.lineWidth = 1;
    ctx.strokeStyle = overviewChartColors.hoverLine;
    ctx.setLineDash([4, 4]);
    ctx.stroke();
    ctx.restore();
  }
};

function renderChart(canvas, existingChart, config) {
  if (!canvas.value || !window.Chart) return existingChart;
  if (existingChart) existingChart.destroy();
  return new window.Chart(canvas.value, config);
}

function destroyOverviewCharts() {
  dailyTrafficChart?.destroy();
  modelUsageChart?.destroy();
  providerUsageChart?.destroy();
  dailyTrafficChart = undefined;
  modelUsageChart = undefined;
  providerUsageChart = undefined;
}

function renderOverviewCharts() {
  if (currentView.value !== 'overview') return;
  dailyTrafficChart = renderChart(dailyTrafficChartEl, dailyTrafficChart, {
    type: 'line',
    data: {
      labels: dailyTrafficRows.value.map((row) => row.date.slice(5)),
      datasets: [
        { label: 'Requests', data: dailyTrafficRows.value.map((row) => row.request_count || 0), borderColor: overviewChartColors.requests, backgroundColor: overviewChartColors.requestsFill, tension: 0.32, pointRadius: 2, pointHoverRadius: 4, borderWidth: 2 },
        { label: 'Tokens', data: dailyTrafficRows.value.map((row) => row.total_tokens || 0), borderColor: overviewChartColors.tokens, backgroundColor: overviewChartColors.tokensFill, tension: 0.32, pointRadius: 2, pointHoverRadius: 4, borderWidth: 2 }
      ]
    },
    options: {
      ...chartDefaults(),
      interaction: { mode: 'index', intersect: false },
      plugins: {
        ...chartDefaults().plugins,
        tooltip: {
          mode: 'index',
          intersect: false,
          backgroundColor: '#171717',
          titleColor: '#ffffff',
          bodyColor: '#ffffff',
          borderColor: 'rgba(255, 255, 255, 0.16)',
          borderWidth: 1,
          padding: 10,
          callbacks: { label: (context) => `${context.dataset.label}: ${formatNum(context.raw)}` }
        }
      }
    },
    plugins: [verticalHoverLinePlugin]
  });
  modelUsageChart = renderChart(modelUsageChartEl, modelUsageChart, {
    type: 'bar',
    data: {
      labels: models.value.slice(0, 8).map((row) => row.model || '-'),
      datasets: [{ label: 'Tokens', data: models.value.slice(0, 8).map((row) => row.total_tokens || 0), backgroundColor: overviewChartColors.model, borderRadius: 5, barPercentage: 0.72, categoryPercentage: 0.72 }]
    },
    options: {
      ...chartDefaults(),
      indexAxis: 'y',
      plugins: { ...chartDefaults().plugins, tooltip: { callbacks: { label: (context) => `Tokens: ${formatNum(context.raw)}` } } },
      scales: {
        x: { ...chartDefaults().scales.x, beginAtZero: true, grid: { color: '#ebebeb' }, ticks: { color: '#525252', callback: (value) => formatNum(value) } },
        y: { ...chartDefaults().scales.y, grid: { display: false }, ticks: { color: '#525252' } }
      }
    }
  });
  providerUsageChart = renderChart(providerUsageChartEl, providerUsageChart, {
    type: 'polarArea',
    data: {
      labels: providers.value.slice(0, 8).map((row) => row.provider_name || '-'),
      datasets: [{ label: 'Tokens', data: providers.value.slice(0, 8).map((row) => row.total_tokens || 0), backgroundColor: providers.value.slice(0, 8).map((_, i) => chartPalette[i % chartPalette.length] + 'CC'), borderColor: providers.value.slice(0, 8).map((_, i) => chartPalette[i % chartPalette.length]), borderWidth: 1 }]
    },
    options: {
      ...chartDefaults(),
      scales: {
        r: { grid: { color: '#ebebeb' }, ticks: { display: false } }
      },
      plugins: {
        ...chartDefaults().plugins,
        tooltip: { callbacks: { label: (context) => `${context.label}: ${formatNum(context.raw)}` } }
      }
    }
  });
}

function modelAnalyticsFor(model) {
  return modelAnalytics.value.find((row) => row.model === model) || {};
}

function modelShare(row) {
  const total = Math.max(modelTotals.value.tokens || 0, 1);
  return `${Math.max(3, (row.total_tokens || 0) / total * 100)}%`;
}

function modelSuccessRate(row) {
  const analytics = modelAnalyticsFor(row.model);
  return analytics.success_rate ?? 1;
}

function analyticsSectionId(id) {
  return `analytics-${id}`;
}

function setActiveAnalyticsSection(id) {
  activeAnalyticsSection.value = id;
}

function updateActiveAnalyticsSection() {
  if (currentView.value !== 'analytics') return;
  const viewportAnchor = 110;
  const visibleSections = analyticsDimensions
    .map((dimension) => ({ dimension, element: document.getElementById(analyticsSectionId(dimension.id)) }))
    .filter((item) => item.element)
    .map((item) => ({ id: item.dimension.id, top: item.element.getBoundingClientRect().top }));
  const current = visibleSections.filter((item) => item.top <= viewportAnchor).at(-1) || visibleSections[0];
  if (current) activeAnalyticsSection.value = current.id;
}

function scrollToAnalyticsSection(id) {
  setActiveAnalyticsSection(id);
  document.getElementById(analyticsSectionId(id))?.scrollIntoView({ block: 'start', behavior: 'smooth' });
}

async function loadOverview() {
  const [statsData, overviewData, providerData, modelData, timeData] = await Promise.all([
    api('/api/stats'),
    api(`/api/overview?range=${range.value}`),
    api('/api/providers'),
    api('/api/models'),
    api(`/api/timeseries?range=${range.value}`)
  ]);
  stats.value = statsData;
  overview.value = overviewData;
  providers.value = providerData;
  models.value = modelData;
  timeseries.value = timeData;
  replayOverviewMotion();
  await nextTick();
  renderOverviewCharts();
}

async function loadAgents() {
  agents.value = await api('/api/agents');
  if (!agents.value.some((agent) => agent.agent_id === activeAgentId.value)) {
    activeAgentId.value = agents.value[0]?.agent_id || '';
  }
}

async function loadProviders() {
  analyticsRows.value = await api(`/api/providers/analytics?range=${range.value}`);
}

async function loadModels() {
  const [modelData, analyticsData] = await Promise.all([
    api('/api/models'),
    api(`/api/models/analytics?range=${range.value}`)
  ]);
  models.value = modelData;
  modelAnalytics.value = analyticsData;
}

async function loadAnalytics() {
  activeAnalyticsSection.value = analyticsDimensions[0].id;
  const entries = await Promise.all(analyticsDimensions.map(async (dimension) => [
    dimension.id,
    await api(`/api/analytics/breakdown?range=${range.value}&dimension=${dimension.id}`)
  ]));
  analyticsSections.value = Object.fromEntries(entries);
  await nextTick();
  updateActiveAnalyticsSection();
}

async function loadFailures() {
  failures.value = await api(`/api/failures?range=${range.value}`);
}

async function loadSessions() {
  const limit = sessionPageSize.value + 1;
  const offset = (sessionPage.value - 1) * sessionPageSize.value;
  const rows = await api(`/api/sessions?limit=${limit}&offset=${offset}`);
  sessionHasNext.value = rows.length > sessionPageSize.value;
  sessions.value = rows.slice(0, sessionPageSize.value);
}

function sessionPrevPage() {
  if (sessionPage.value > 1) { sessionPage.value--; loadSessions(); }
}

function sessionNextPage() {
  if (sessionHasNext.value) { sessionPage.value++; loadSessions(); }
}

function changeSessionPageSize(e) {
  sessionPageSize.value = Number(e.target.value);
  sessionPage.value = 1;
  loadSessions();
}

async function loadSessionDetail() {
  const id = path.value.split('/').pop();
  sessionDetail.value = await api(`/api/sessions/${id}`);
  sessionLog.value = await api(`/api/sessions/${id}/log`).catch(() => null);
}

async function loadView() {
  loading.value = true;
  setNotice('');
  try {
    if (currentView.value === 'overview') await loadOverview();
    if (currentView.value === 'agents') await loadAgents();
    if (currentView.value === 'providers') await loadProviders();
    if (currentView.value === 'models') await loadModels();
    if (currentView.value === 'analytics') await loadAnalytics();
    if (currentView.value === 'failures') await loadFailures();
    if (currentView.value === 'sessions') await loadSessions();
    if (currentView.value === 'sessionDetail') await loadSessionDetail();
  } catch (error) {
    setNotice(error.message || 'Failed to load dashboard data.', true);
  } finally {
    loading.value = false;
  }
}

async function toggleAgent(agent, binding) {
  const enabled = !binding.gateway_enabled;
  const plan = await api(`/api/agents/${encodeURIComponent(agent.agent_id)}/plan`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ provider_id: binding.provider_id, enabled, config_path: binding.config_path })
  });
  await api(`/api/agents/${encodeURIComponent(agent.agent_id)}/apply`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ provider_id: binding.provider_id, enabled, config_path: binding.config_path, base_checksum: plan.base_checksum })
  });
  setNotice(`${agent.display_name} ${enabled ? 'enabled' : 'disabled'} ${binding.provider_id}.`);
  await loadAgents();
}

function onScroll() {
  updateActiveAnalyticsSection();
}

window.addEventListener('popstate', () => {
  path.value = normalizePath(window.location.pathname);
  loadView();
});

onMounted(() => {
  window.addEventListener('scroll', onScroll, { passive: true });
  loadView();
});

onBeforeUnmount(() => {
  window.removeEventListener('scroll', onScroll);
  destroyOverviewCharts();
});
</script>

<template>
  <header class="topbar">
    <nav class="shell">
      <button class="brand" type="button" @click="go('/dashboard')"><span class="brand-mark">U</span><span>Umbragate</span></button>
      <button v-for="item in navItems" :key="item.path" type="button" :class="{ active: path === item.path || (item.path !== '/dashboard' && path.startsWith(item.path)) }" @click="go(item.path)">
        {{ item.label }}
      </button>
    </nav>
  </header>

  <main class="shell">
    <div v-if="notice" :class="['notice', { error: noticeError }]">{{ notice }}</div>

    <section v-if="currentView === 'overview'" class="page overview-page" :data-open="overviewMotionOpen ? 'true' : 'false'" :style="overviewCursorStyle" @pointermove="moveOverviewCursor" @pointerleave="resetOverviewCursor">
      <div class="overview-cursor-glow" aria-hidden="true"></div>
      <div class="overview-hero t-panel-slide overview-interactive-card" :data-open="overviewMotionOpen ? 'true' : 'false'" @pointermove="tiltOverviewCard" @pointerleave="resetOverviewCard">
        <div>
          <p class="eyebrow">Gateway Command Center</p>
          <h1>Overview</h1>
          <p class="hero-copy">Live traffic, token pressure, reliability, and latency in one operational snapshot.</p>
        </div>
        <div class="hero-actions">
          <select v-model="range" @change="loadOverview"><option v-for="item in ranges" :key="item">{{ item }}</option></select>
          <button class="secondary" type="button" @click="loadOverview">Refresh</button>
        </div>
      </div>
      <div class="metrics overview-metrics t-panel-slide" :data-open="overviewMotionOpen ? 'true' : 'false'">
        <article class="t-resize overview-interactive-card" @pointermove="tiltOverviewCard" @pointerleave="resetOverviewCard"><span>Today Requests</span><strong class="t-digit-group is-animating"><span v-for="(digit, index) in metricDigits(formatNum(stats?.today_requests))" :key="`today-requests-${digit.char}-${index}-${stats?.today_requests}`" class="t-digit" :data-stagger="digit.stagger">{{ digit.char }}</span></strong><small>{{ formatNum(stats?.month_requests) }} this month</small></article>
        <article class="t-resize overview-interactive-card" @pointermove="tiltOverviewCard" @pointerleave="resetOverviewCard"><span>Today Tokens</span><strong class="t-digit-group is-animating"><span v-for="(digit, index) in metricDigits(formatNum(stats?.today_tokens))" :key="`today-tokens-${digit.char}-${index}-${stats?.today_tokens}`" class="t-digit" :data-stagger="digit.stagger">{{ digit.char }}</span></strong><small>{{ formatNum(stats?.month_tokens) }} this month</small></article>
        <article class="t-resize overview-interactive-card" @pointermove="tiltOverviewCard" @pointerleave="resetOverviewCard"><span>Success Rate</span><strong class="t-digit-group is-animating"><span v-for="(digit, index) in metricDigits(pct(overview?.success_rate))" :key="`success-rate-${digit.char}-${index}-${overview?.success_rate}`" class="t-digit" :data-stagger="digit.stagger">{{ digit.char }}</span></strong><small>{{ formatNum(overview?.error_count) }} errors in {{ range }}</small></article>
        <article class="t-resize overview-interactive-card" @pointermove="tiltOverviewCard" @pointerleave="resetOverviewCard"><span>P95 Latency</span><strong class="t-digit-group is-animating"><span v-for="(digit, index) in metricDigits(duration(overview?.p95_duration_ms))" :key="`p95-${digit.char}-${index}-${overview?.p95_duration_ms}`" class="t-digit" :data-stagger="digit.stagger">{{ digit.char }}</span></strong><small>{{ duration(overview?.avg_duration_ms) }} average</small></article>
      </div>
      <div class="overview-layout t-panel-slide" :data-open="overviewMotionOpen ? 'true' : 'false'">
        <section class="panel trend-panel overview-interactive-card" @pointermove="tiltOverviewCard" @pointerleave="resetOverviewCard">
          <div class="panel-head"><h2>Daily Traffic</h2><span class="muted">{{ range }}</span></div>
          <div class="overview-chart"><canvas ref="dailyTrafficChartEl"></canvas></div>
        </section>
        <section class="panel reliability-panel overview-interactive-card" @pointermove="tiltOverviewCard" @pointerleave="resetOverviewCard">
          <div class="panel-head"><h2>Reliability</h2><span class="muted">{{ formatNum(overview?.total_requests) }} requests</span></div>
          <div class="radial-card"><div class="radial-meter" :style="{ '--value': `${Math.round(Number(overview?.success_rate || 0) * 100)}%` }"><strong>{{ pct(overview?.success_rate) }}</strong><span>success</span></div></div>
          <div class="mini-stats"><div><span>Successful</span><strong>{{ formatNum(overview?.success_count) }}</strong></div><div><span>Failed</span><strong>{{ formatNum(overview?.error_count) }}</strong></div></div>
        </section>
        <section class="panel overview-interactive-card" @pointermove="tiltOverviewCard" @pointerleave="resetOverviewCard">
          <div class="panel-head"><h2>Model Usage</h2><span class="muted">Top {{ models.slice(0, 8).length }}</span></div>
          <div v-if="!models.length" class="empty-state">No model usage yet.</div>
          <div v-show="models.length" class="overview-chart"><canvas ref="modelUsageChartEl"></canvas></div>
        </section>
        <section class="panel overview-interactive-card" @pointermove="tiltOverviewCard" @pointerleave="resetOverviewCard">
          <div class="panel-head"><h2>Provider Usage</h2><span class="muted">Top {{ providers.slice(0, 8).length }}</span></div>
          <div v-if="!providers.length" class="empty-state">No provider usage yet.</div>
          <div v-show="providers.length" class="overview-chart"><canvas ref="providerUsageChartEl"></canvas></div>
        </section>
      </div>
    </section>

    <section v-else-if="currentView === 'agents'" class="page">
      <div class="page-head"><div><p class="eyebrow">Configuration</p><h1>Agents</h1></div><button @click="loadAgents">Refresh</button></div>
      <div class="tabs"><button v-for="agent in agents" :key="agent.agent_id" :class="{ active: activeAgentId === agent.agent_id }" @click="activeAgentId = agent.agent_id">{{ agent.display_name }}</button></div>
      <section v-if="activeAgent" class="panel">
        <h2>{{ activeAgent.display_name }}</h2>
        <p class="muted">Gateway URL: <code>{{ activeAgent.gateway_base_url || 'http://127.0.0.1:4141' }}</code></p>
        <p class="muted">Proxy Method: <strong>{{ activeAgent.proxy_method || 'Direct Passthrough' }}</strong></p>
        <p v-if="!activeAgent.gateway_capable" class="muted">{{ activeAgent.gateway_disabled_reason || 'Gateway proxy is temporarily unavailable for this agent.' }}</p>
        <table><thead><tr><th>Provider</th><th>Live Base URL</th><th>Gateway</th><th></th></tr></thead>
          <tbody><tr v-for="binding in activeAgent.bindings" :key="binding.provider_id">
            <td>{{ binding.provider_id }}</td><td><code>{{ binding.live_base_url || '-' }}</code></td><td><span :class="['badge', binding.gateway_enabled ? 'ok' : 'muted']">{{ binding.gateway_enabled ? 'Enabled' : 'Disabled' }}</span></td>
            <td><button class="secondary" :disabled="!activeAgent.gateway_capable && !binding.gateway_enabled" @click="toggleAgent(activeAgent, binding)">{{ binding.gateway_enabled ? 'Disable' : (activeAgent.gateway_capable ? 'Enable' : 'Unavailable') }}</button></td>
          </tr></tbody>
        </table>
      </section>
    </section>

    <section v-else-if="currentView === 'providers'" class="page provider-page">
      <div class="page-head">
        <div><p class="eyebrow">Usage</p><h1>Providers</h1></div>
        <div class="filters"><select v-model="range" @change="loadProviders"><option v-for="item in ranges" :key="item">{{ item }}</option></select><button class="secondary" @click="loadProviders">Refresh</button></div>
      </div>
      <div class="metrics provider-metrics">
        <article><span>Active Providers</span><strong>{{ formatNum(analyticsRows.length) }}</strong></article>
        <article><span>Requests</span><strong>{{ formatNum(providerTotals.requests) }}</strong></article>
        <article><span>Tokens</span><strong>{{ formatNum(providerTotals.tokens) }}</strong></article>
        <article><span>Errors</span><strong>{{ formatNum(providerTotals.errors) }}</strong></article>
      </div>
      <section class="panel provider-analytics">
        <div class="panel-head"><h2>Provider Analytics</h2><span class="muted">{{ range }}</span></div>
        <div class="analytics-grid">
          <div v-if="!analyticsRows.length" class="empty-state">No provider usage yet.</div>
          <article v-for="row in analyticsRows" :key="row.provider_name" class="provider-card">
            <div><strong>{{ row.provider_name }}</strong><span>{{ formatNum(row.total_tokens) }} tokens</span></div>
            <div class="provider-card-metrics"><span>{{ formatNum(row.request_count) }} req</span><span>{{ pct(row.success_rate) }} success</span><span>{{ duration(row.avg_duration_ms) }} avg</span></div>
          </article>
        </div>
      </section>
    </section>

    <section v-else-if="currentView === 'models'" class="page models-page">
      <div class="page-head">
        <div><p class="eyebrow">Model Portfolio</p><h1>Models</h1><p class="hero-copy">Compare model demand, token share, reliability, and latency.</p></div>
        <div class="filters"><select v-model="range" @change="loadModels"><option v-for="item in ranges" :key="item">{{ item }}</option></select><button class="secondary" type="button" @click="loadModels">Refresh</button></div>
      </div>
      <div class="metrics model-metrics">
        <article><span>Active Models</span><strong>{{ formatNum(models.length) }}</strong><small>{{ modelAnalytics.length }} in {{ range }}</small></article>
        <article><span>Requests</span><strong>{{ formatNum(modelTotals.requests) }}</strong><small>all model calls</small></article>
        <article><span>Tokens</span><strong>{{ formatNum(modelTotals.tokens) }}</strong><small>prompt + completion</small></article>
        <article><span>Avg Latency</span><strong>{{ duration(modelAvgLatency) }}</strong><small>{{ formatNum(modelTotals.errors) }} errors</small></article>
      </div>
      <div class="models-layout">
        <section class="panel model-rank-panel">
          <div class="panel-head"><h2>Token Share</h2><span class="muted">Top {{ heaviestModels.length }}</span></div>
          <div v-if="!heaviestModels.length" class="empty-state">No model usage yet.</div>
          <article v-for="(row, index) in heaviestModels" :key="row.model" class="model-card">
            <div class="model-card-head"><strong :title="row.model">{{ row.model || '-' }}</strong><span>{{ formatNum(row.total_tokens) }} tokens</span></div>
            <div class="model-meter"><i :style="{ width: modelShare(row), background: chartGradient(index) }"></i></div>
            <div class="model-card-foot"><span>{{ formatNum(row.request_count) }} req</span><span>{{ duration(row.avg_duration_ms) }} avg</span><span>{{ pct(modelSuccessRate(row)) }} success</span></div>
          </article>
        </section>
        <section class="panel model-speed-panel">
          <div class="panel-head"><h2>Fastest Models</h2><span class="muted">Avg latency</span></div>
          <div v-if="!fastestModels.length" class="empty-state">No latency data yet.</div>
          <div v-for="(row, index) in fastestModels" :key="row.model" class="speed-row">
            <span :title="row.model">{{ row.model || '-' }}</span>
            <div><i :style="{ width: `${Math.max(8, 100 - (row.avg_duration_ms || 0) / Math.max(...fastestModels.map((item) => item.avg_duration_ms || 0), 1) * 82)}%`, background: chartGradient(index + 2) }"></i></div>
            <strong>{{ duration(row.avg_duration_ms) }}</strong>
          </div>
        </section>
        <section class="panel models-table-panel">
          <div class="panel-head"><h2>Model Inventory</h2><span class="muted">{{ models.length }} rows</span></div>
          <div class="model-table">
            <div class="model-table-head"><span>Model</span><span>Requests</span><span>Tokens</span><span>Share</span><span>Success</span><span>Avg Latency</span></div>
            <div v-if="!models.length" class="empty-state">No models yet.</div>
            <div v-for="row in models" :key="row.model" class="model-table-row">
              <span :title="row.model">{{ row.model || '-' }}</span>
              <strong>{{ formatNum(row.request_count) }}</strong>
              <span>{{ formatNum(row.total_tokens) }}</span>
              <span>{{ Math.round((row.total_tokens || 0) / Math.max(modelTotals.tokens, 1) * 100) }}%</span>
              <span>{{ pct(modelSuccessRate(row)) }}</span>
              <span>{{ duration(row.avg_duration_ms) }}</span>
            </div>
          </div>
        </section>
      </div>
    </section>

    <section v-else-if="currentView === 'analytics'" class="page analytics-page">
      <div class="page-head">
        <div><p class="eyebrow">Dimensions</p><h1>Analytics</h1></div>
        <div class="filters"><select v-model="range" @change="loadAnalytics"><option v-for="item in ranges" :key="item">{{ item }}</option></select></div>
      </div>
      <div class="analytics-layout">
        <aside class="analytics-menu" aria-label="Analytics sections">
          <a
            v-for="dimension in analyticsDimensions"
            :key="dimension.id"
            :href="`#${analyticsSectionId(dimension.id)}`"
            :class="{ active: activeAnalyticsSection === dimension.id }"
            :aria-current="activeAnalyticsSection === dimension.id ? 'true' : undefined"
            @click.prevent="scrollToAnalyticsSection(dimension.id)"
          >{{ dimension.label }}</a>
        </aside>
        <div class="analytics-sections">
          <section v-for="dimension in analyticsDimensions" :id="analyticsSectionId(dimension.id)" :key="dimension.id" class="panel analytics-section">
            <div class="panel-head">
              <h2>{{ dimension.label }}</h2>
              <span class="muted">{{ sectionRows(dimension.id).length }} rows</span>
            </div>
            <div v-if="chartRows(dimension.id).length" class="analytics-charts">
              <div class="analytics-chart">
                <div class="chart-title"><span>Requests</span><span>Top {{ chartRows(dimension.id).length }}</span></div>
                <div v-for="(row, index) in chartRows(dimension.id)" :key="`${row.name}-requests`" class="chart-row">
                  <span :title="row.name">{{ row.name || '-' }}</span>
                  <div class="chart-track"><i :style="{ width: `${Math.max(3, (row.request_count || 0) / chartMax(dimension.id) * 100)}%`, background: chartGradient(index) }"></i></div>
                  <strong>{{ formatNum(row.request_count) }}</strong>
                </div>
              </div>
              <div class="analytics-chart">
                <div class="chart-title"><span>Outcome</span><span>Success / Error</span></div>
                <div v-for="row in chartRows(dimension.id)" :key="`${row.name}-outcome`" class="chart-row outcome-row">
                  <span :title="row.name">{{ row.name || '-' }}</span>
                  <div class="stack-track">
                    <i class="stack-success" :style="{ width: successWidth(row) }"></i>
                    <i class="stack-error" :style="{ width: errorWidth(row) }"></i>
                  </div>
                  <strong :class="{ warn: (row.error_count || 0) > 0 }">{{ pct(row.success_rate) }}</strong>
                </div>
              </div>
            </div>
            <div class="analytics-table">
              <div class="analytics-table-head"><span>Name</span><span>Requests</span><span>Tokens</span><span>Success</span><span>Avg Latency</span></div>
              <div v-if="!sectionRows(dimension.id).length" class="empty-state">No {{ dimension.label.toLowerCase() }} analytics yet.</div>
              <div v-for="row in sectionRows(dimension.id)" :key="row.name" class="analytics-table-row">
                <span :title="row.name">{{ row.name || '-' }}</span>
                <strong>{{ formatNum(row.request_count) }}</strong>
                <span>{{ formatNum(row.total_tokens) }}</span>
                <span>{{ pct(row.success_rate) }}</span>
                <span>{{ duration(row.avg_duration_ms) }}</span>
              </div>
            </div>
          </section>
        </div>
      </div>
    </section>

    <section v-else-if="currentView === 'failures'" class="page">
      <div class="page-head"><div><p class="eyebrow">Reliability</p><h1>Failures</h1></div><select v-model="range" @change="loadFailures"><option v-for="item in ranges" :key="item">{{ item }}</option></select></div>
      <div class="metrics"><article><span>Total Failures</span><strong>{{ formatNum(failures?.total_failures) }}</strong></article><article><span>Recent</span><strong>{{ formatNum(failures?.recent?.length) }}</strong></article></div>
      <section class="panel"><table><thead><tr><th>Time</th><th>Provider</th><th>Model</th><th>Error</th></tr></thead><tbody><tr v-for="row in failures?.recent || []" :key="row.id"><td>{{ time(row.started_at) }}</td><td>{{ row.provider_name }}</td><td>{{ row.model }}</td><td>{{ row.error_message }}</td></tr></tbody></table></section>
    </section>

    <section v-else-if="currentView === 'sessions'" class="page">
      <div class="page-head"><div><p class="eyebrow">Requests</p><h1>Sessions</h1></div><div class="page-head-actions"><select :value="sessionPageSize" @change="changeSessionPageSize"><option :value="25">25 / page</option><option :value="50">50 / page</option><option :value="100">100 / page</option></select><button @click="loadSessions">Refresh</button></div></div>
      <section class="panel"><table><thead><tr><th>Time</th><th>Agent</th><th>Provider</th><th>Model</th><th>Tokens</th><th>Status</th><th></th></tr></thead><tbody><tr v-for="session in sessions" :key="session.id"><td>{{ time(session.started_at) }}</td><td>{{ session.agent_id || '-' }}</td><td>{{ session.provider_name }}</td><td>{{ session.model }}</td><td>{{ formatNum((session.prompt_tokens || 0) + (session.completion_tokens || 0)) }}</td><td><span class="badge">{{ session.status }}</span></td><td><button class="secondary" @click="go(`/dashboard/sessions/${session.id}`)">View</button></td></tr></tbody></table></section>
      <div class="pagination-bar"><span class="pagination-info">Page {{ sessionPage }} · {{ sessions.length }} rows</span><div class="pagination-controls"><button class="secondary" :disabled="sessionPage <= 1" @click="sessionPrevPage">Previous</button><button class="secondary" :disabled="!sessionHasNext" @click="sessionNextPage">Next</button></div></div>
    </section>

    <section v-else-if="currentView === 'sessionDetail'" class="page">
      <div class="page-head"><div><p class="eyebrow">Session</p><h1>#{{ sessionDetail?.id }}</h1></div><button class="secondary" @click="go('/dashboard/sessions')">Back</button></div>
      <section class="panel detail-grid"><div><span>Provider</span><strong>{{ sessionDetail?.provider_name }}</strong></div><div><span>Model</span><strong>{{ sessionDetail?.model }}</strong></div><div><span>Duration</span><strong>{{ duration(sessionDetail?.duration_ms) }}</strong></div><div><span>Status</span><strong>{{ sessionDetail?.status }}</strong></div></section>
      <section class="panel"><h2>Request Log</h2><pre>{{ JSON.stringify(sessionLog, null, 2) }}</pre></section>
    </section>

    <div v-if="loading" class="loading">Loading...</div>
  </main>
</template>
