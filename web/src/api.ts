const BASE = "/admin";

async function request<T>(path: string, options?: RequestInit): Promise<T> {
	const res = await fetch(`${BASE}${path}`, {
		headers: { "Content-Type": "application/json" },
		...options,
	});
	if (!res.ok) {
		const body = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(body.error || `HTTP ${res.status}`);
	}
	return res.json();
}

async function createLinkStream(
	link: Partial<import("./types").ProxyLink>,
	onProgress: (progress: import("./types").ValidationProgress) => void,
): Promise<import("./types").ProxyLink> {
	const response = await fetch(`${BASE}/links/stream`, {
		method: "POST",
		headers: {
			Accept: "text/event-stream",
			"Content-Type": "application/json",
		},
		body: JSON.stringify(link),
	});
	if (!response.ok) {
		const body = await response
			.json()
			.catch(() => ({ error: response.statusText }));
		throw new Error(body.error || `HTTP ${response.status}`);
	}
	if (!response.body)
		throw new Error("Validation progress stream is unavailable");

	const reader = response.body.getReader();
	const decoder = new TextDecoder();
	let buffer = "";
	let completedLink: import("./types").ProxyLink | null = null;

	const handleEvent = (event: string, data: string) => {
		if (!data) return;
		let payload: unknown;
		try {
			payload = JSON.parse(data);
		} catch {
			throw new Error("Received invalid validation progress data");
		}
		if (event === "provider") {
			onProgress(payload as import("./types").ValidationProgress);
		} else if (event === "complete") {
			completedLink = payload as import("./types").ProxyLink;
		} else if (event === "error") {
			const message =
				typeof payload === "object" &&
				payload !== null &&
				"error" in payload &&
				typeof payload.error === "string"
					? payload.error
					: "Validation failed";
			throw new Error(message);
		}
	};

	while (true) {
		const { done, value } = await reader.read();
		buffer += decoder.decode(value, { stream: !done });
		const events = buffer.split(/\r?\n\r?\n/);
		buffer = events.pop() || "";
		for (const message of events) {
			const lines = message.split(/\r?\n/);
			const event = lines
				.find((line) => line.startsWith("event:"))
				?.slice(6)
				.trim();
			const data = lines
				.filter((line) => line.startsWith("data:"))
				.map((line) => line.slice(5).trimStart())
				.join("\n");
			if (event) handleEvent(event, data);
		}
		if (done) break;
	}
	if (!completedLink)
		throw new Error("Validation progress stream ended unexpectedly");
	return completedLink;
}

export const api = {
	getTypes: () => request<{ types: string[] }>("/types"),

	listProviders: async () =>
		(await request<import("./types").Provider[] | null>("/providers")) ?? [],
	createProvider: (p: Partial<import("./types").Provider>) =>
		request<import("./types").Provider>("/providers", {
			method: "POST",
			body: JSON.stringify(p),
		}),
	getProvider: (id: string) =>
		request<import("./types").Provider>(`/providers/${id}`),
	updateProviderAPIKey: (id: string, apiKey: string) =>
		request<import("./types").Provider>(`/providers/${id}/api-key`, {
			method: "PUT",
			body: JSON.stringify({ api_key: apiKey }),
		}),
	refreshProviderModels: (id: string) =>
		request<import("./types").Provider>(`/providers/${id}/models/refresh`, {
			method: "POST",
		}),
	deleteProvider: (id: string) =>
		request<void>(`/providers/${id}`, { method: "DELETE" }),

	listLinks: async () =>
		(await request<import("./types").ProxyLink[] | null>("/links")) ?? [],
	createLink: (l: Partial<import("./types").ProxyLink>) =>
		request<import("./types").ProxyLink>("/links", {
			method: "POST",
			body: JSON.stringify(l),
		}),
	createLinkStream,
	getLink: (id: string) => request<import("./types").ProxyLink>(`/links/${id}`),
	testLink: (id: string, model?: string) =>
		request<import("./types").ProxyLink>(`/links/${id}/test`, {
			method: "POST",
			body: JSON.stringify(model ? { model } : {}),
		}),
	deleteLink: (id: string) =>
		request<void>(`/links/${id}`, { method: "DELETE" }),

	listRecentRequests: async () =>
		(await request<import("./types").RequestLog[] | null>("/requests")) ?? [],
	listValidationRequests: async () =>
		(await request<import("./types").RequestLog[] | null>(
			"/validation-requests",
		)) ?? [],

	getStats: (params?: { link_id?: string; from?: string; to?: string }) => {
		const qs = new URLSearchParams();
		if (params?.link_id) qs.set("link_id", params.link_id);
		if (params?.from) qs.set("from", params.from);
		if (params?.to) qs.set("to", params.to);
		const q = qs.toString();
		return request<import("./types").StatsResponse>(
			`/stats${q ? `?${q}` : ""}`,
		);
	},
};
