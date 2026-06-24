export const translations = {
    en: {
        home: 'Home', agents: 'Agents', sessions: 'Sessions', models: 'Models', providers: 'Providers', analytics: 'Analytics', failures: 'Failures', dashboard: 'Dashboard', timeRange: 'Time Range', performance: 'Performance', usage: 'Usage', successRate: 'Success Rate', avgLatency: 'Avg Latency', p95Latency: 'P95 Latency', errors: 'Errors', successRateDesc: '% of requests that completed without errors', avgLatencyDesc: 'Average response time across all successful requests', p95LatencyDesc: '95th percentile - 95% of requests are faster than this', errorsDesc: 'Total failed requests in selected time range', todayRequests: 'Today Requests', todayRequestsDesc: 'Total LLM API calls made today', monthRequests: 'Month Requests', monthRequestsDesc: 'Total LLM API calls this calendar month', todayTokens: 'Today Tokens', todayTokensDesc: 'Input + output tokens consumed today', monthTokens: 'Month Tokens', monthTokensDesc: 'Input + output tokens consumed this calendar month', last7Days: 'Last 7 Days', tokensByProvider: 'Tokens by Provider', tokensByModel: 'Tokens by Model', loading: 'Loading...', failedToLoadUsage: 'Failed to load usage', noUsageYet: 'No usage yet', tokens: 'tokens', requests: 'Requests', chartUnavailable: 'Chart library failed to load', switchLang: '中文'
    },
    zh: {
        home: '首页', agents: 'Agent', sessions: '会话', models: '模型', providers: '提供方', analytics: '分析', failures: '失败记录', dashboard: '仪表盘', timeRange: '时间范围', performance: '性能', usage: '用量', successRate: '成功率', avgLatency: '平均延迟', p95Latency: 'P95 延迟', errors: '错误数', successRateDesc: '成功完成（未出错）的请求占比', avgLatencyDesc: '所有成功请求的平均响应时间', p95LatencyDesc: '第95百分位 - 95% 的请求快于此值', errorsDesc: '所选时间范围内的失败请求总数', todayRequests: '今日请求', todayRequestsDesc: '今日发起的 LLM API 调用总数', monthRequests: '本月请求', monthRequestsDesc: '本月发起的 LLM API 调用总数', todayTokens: '今日 Tokens', todayTokensDesc: '今日消耗的输入 + 输出 Tokens', monthTokens: '本月 Tokens', monthTokensDesc: '本月消耗的输入 + 输出 Tokens', last7Days: '最近 7 天', tokensByProvider: '按 Provider 统计 Tokens', tokensByModel: '按模型统计 Tokens', loading: '加载中...', failedToLoadUsage: '加载用量失败', noUsageYet: '暂无用量', tokens: 'tokens', requests: '请求数', chartUnavailable: '图表库加载失败', switchLang: 'EN'
    }
};

export function getLang() {
    const lang = localStorage.getItem('dashboard_lang');
    return lang === 'zh' ? 'zh' : 'en';
}

export function setLang(lang) {
    localStorage.setItem('dashboard_lang', lang);
}

export function t(key) {
    return translations[getLang()][key] || translations.en[key] || key;
}
