export type Locale = "en" | "zh";

export interface Translations {
	// App shell
	app: {
		title: string;
		footerBuiltBy: string;
	};

	// Nav tabs
	nav: {
		statistics: string;
		providers: string;
		links: string;
	};

	// Stats dashboard
	stats: {
		title: string;
		subtitle: string;
		filterLink: string;
		allLinks: string;
		searchLinks: string;
		filterDateRange: string;
		last24h: string;
		last7d: string;
		last30d: string;
		allTime: string;
		selectRange: string;
		apply: string;
		totalRequests: string;
		successRate: string;
		failures: string;
		avgLatency: string;
		byProxyLink: string;
		tableLink: string;
		tableTotal: string;
		tableSuccess: string;
		tableFailure: string;
		tableAvgLatency: string;
		tableSuccessRate: string;
		latestRequests: string;
		latestRequestsDesc: string;
		noRequests: string;
		tableTime: string;
		tableProvider: string;
		tableModel: string;
		tableStatus: string;
		tableLatency: string;
		linkTestBadge: string;
		viewRequest: string;
		errStatus: string;
	};

	// Provider manager
	providers: {
		title: string;
		subtitle: string;
		addProvider: string;
		noProviders: string;
		builtin: string;
		tableName: string;
		tableEndpoints: string;
		tableApiKey: string;
		save: string;
		cancel: string;
		notSet: string;
		maskedKey: string;
		changeKey: string;
		setKey: string;
		edit: string;
		del: string;
		deleteConfirm: string;
		deleteFailed: string;
		saveFailed: string;
		editProvider: string;
		newProvider: string;
		nameLabel: string;
		namePlaceholder: string;
		typeLabel: string;
		endpointsLabel: string;
		endpointsDesc: string;
		addEndpoint: string;
		styleLabel: string;
		requestLabel: string;
		responseLabel: string;
		baseUrlLabel: string;
		baseUrlPlaceholderResponses: string;
		baseUrlPlaceholderDefault: string;
		removeEndpoint: string;
		apiKeyLabel: string;
		apiKeyPlaceholder: string;
		modelsLabel: string;
		modelsPlaceholder: string;
		saving: string;
		openai: string;
		anthropic: string;
		chat: string;
	};

	// Link manager
	links: {
		title: string;
		subtitle: string;
		newLink: string;
		noLinks: string;
		tableName: string;
		tableCapabilityCheck: string;
		tableProxyUrl: string;
		tableChain: string;
		tableActions: string;
		noCommonFormat: string;
		copyUrl: string;
		test: string;
		testing: string;
		edit: string;
		del: string;
		testTitle: string;
		deleteConfirm: string;
		deleteFailed: string;
		testFailed: string;
	};

	// Link editor
	linkEditor: {
		newTitle: string;
		editTitle: string;
		subtitle: string;
		nameLabel: string;
		namePlaceholder: string;
		pathLabel: string;
		pathPlaceholder: string;
		enabled: string;
		capabilityTitle: string;
		capabilityDesc: string;
		chainPreview: string;
		noProvidersInChain: string;
		cancel: string;
		saveLink: string;
		saving: string;
		providerChain: string;
		addStep: string;
		primary: string;
		finalFallback: string;
		fallbackNum: string;
		moveUp: string;
		moveDown: string;
		remove: string;
		autoDetectTitle: string;
		autoDetectDesc: string;
		providerLabel: string;
		searchProvider: string;
		retryLabel: string;
		fallbackModelLabel: string;
		fallbackModelPlaceholder: string;
		apiKeyLabel: string;
		apiKeyOverrideLabel: string;
		apiKeyPlaceholder: string;
		apiKeyPlaceholderHasGlobal: string;
		selectProviderError: string;
		saveFailed: string;
	};

	// Request details modal
	requestDetails: {
		title: string;
		noContent: string;
		noHeaders: string;
		provider: string;
		model: string;
		status: string;
		latency: string;
		request: string;
		response: string;
		agentToGateway: string;
		gatewayToProvider: string;
		headers: string;
		body: string;
		noUpstreamUrl: string;
	};

	// Searchable select
	searchableSelect: {
		noResults: string;
		search: string;
	};

	// Protocols
	protocols: {
		openaiStyle: string;
		anthropicStyle: string;
		chatCompletions: string;
		responses: string;
		messages: string;
		formatNotSelected: string;
		protocolNotSelected: string;
	};
}
