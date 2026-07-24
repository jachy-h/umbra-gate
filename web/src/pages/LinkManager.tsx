import { useEffect, useState } from "react";
import { api } from "../api";
import type { ProxyLink, Provider } from "../types";
import { useHash } from "../hooks/useHash";
import { Card } from "../components/Card";
import { Badge } from "../components/Badge";
import { Button } from "../components/Button";
import { Spinner } from "../components/Spinner";
import { formatLabel } from "../protocols";

export function LinkManager() {
	const [links, setLinks] = useState<ProxyLink[]>([]);
	const [providers, setProviders] = useState<Provider[]>([]);
	const [loading, setLoading] = useState(true);
	const [testingLinkID, setTestingLinkID] = useState<string | null>(null);
	const { navigate } = useHash();

	const fetchAll = () => {
		setLoading(true);
		return Promise.all([api.listLinks(), api.listProviders()])
			.then(([l, p]) => {
				setLinks(l);
				setProviders(p);
			})
			.catch(console.error)
			.finally(() => setLoading(false));
	};

	useEffect(() => {
		fetchAll();
	}, []);

	const remove = async (id: string) => {
		if (!confirm("Delete this proxy link?")) return;
		try {
			await api.deleteLink(id);
			fetchAll();
		} catch (e: unknown) {
			alert(e instanceof Error ? e.message : "Delete failed");
		}
	};

	const testLink = async (id: string) => {
		setTestingLinkID(id);
		try {
			await api.testLink(id);
			await fetchAll();
		} catch (e: unknown) {
			alert(e instanceof Error ? e.message : "Link test failed");
		} finally {
			setTestingLinkID(null);
		}
	};

	const gatewayBase =
		(import.meta.env.VITE_GATEWAY_BASE as string | undefined)?.replace(
			/\/$/,
			"",
		) || `http://${window.location.hostname}:8787`;

	const proxyUrl = (path: string) => `${gatewayBase}/llm-gateway-lite/${path}`;

	const copyUrl = async (path: string) => {
		const url = proxyUrl(path);
		try {
			await navigator.clipboard.writeText(url);
		} catch {
			const el = document.createElement("textarea");
			el.value = url;
			document.body.appendChild(el);
			el.select();
			document.execCommand("copy");
			document.body.removeChild(el);
		}
	};

	const providerName = (id: string) =>
		providers.find((p) => p.id === id)?.name || id;

	return (
		<div className="space-y-8 animate-fade-in">
			<div className="flex items-center justify-between">
				<div>
					<h1 className="text-[28px] font-semibold leading-[1.2] tracking-[-0.5px] text-[var(--color-ink)]">
						Proxy Links
					</h1>
					<p className="mt-2 text-[var(--color-muted)] text-base">
						Configure proxy routes with provider chaining for fallback.
					</p>
				</div>
				<Button onClick={() => navigate("/links/new")}>+ New Link</Button>
			</div>

			{loading ? (
				<Spinner />
			) : links.length === 0 ? (
				<Card className="text-center text-[var(--color-muted)] py-16">
					No proxy links configured. Create one to start routing requests.
				</Card>
			) : (
				<div className="overflow-x-auto rounded-xl border border-[var(--color-hairline)] bg-[var(--color-canvas)]">
					<table className="w-full min-w-[900px]">
						<thead>
							<tr className="border-b border-[var(--color-hairline-soft)] text-left text-sm font-medium text-[var(--color-muted)]">
								<th className="px-6 py-3 font-medium w-[140px]">Name</th>
								<th className="px-6 py-3 font-medium">Capability check</th>
								<th className="px-6 py-3 font-medium w-[220px]">Proxy URL</th>
								<th className="px-6 py-3 font-medium">Chain</th>
								<th className="px-6 py-3 font-medium w-[180px] sticky right-0 bg-[var(--color-canvas)]">
									Actions
								</th>
							</tr>
						</thead>
						<tbody>
							{links.map((l) => (
								<tr
									key={l.id}
									className="border-b border-[var(--color-hairline-soft)] last:border-b-0 hover:bg-[var(--color-surface-soft)] transition-colors"
								>
									<td className="px-6 py-4">
										<div className="text-sm font-semibold text-[var(--color-ink)]">
											{l.name}
										</div>
									</td>
									<td className="px-6 py-4">
										<div className="space-y-1.5">
											{l.supported_formats && l.supported_formats.length > 0 ? (
												<div className="flex flex-wrap gap-1">
													{l.supported_formats.map((format) => (
														<span
															key={format}
															className="inline-flex items-center gap-1 rounded border border-[var(--color-hairline)] bg-[var(--color-surface-soft)] px-1.5 py-0.5 text-[11px] font-semibold text-[var(--color-ink)]"
														>
															<svg
																width="10"
																height="10"
																viewBox="0 0 24 24"
																fill="none"
																stroke="var(--color-success)"
																strokeWidth="3"
																strokeLinecap="round"
																strokeLinejoin="round"
															>
																<path d="M20 6L9 17l-5-5" />
															</svg>
															{formatLabel(format)}
														</span>
													))}
												</div>
											) : (
												<span className="text-xs text-[var(--color-error)] font-medium">
													No shared capability — providers in chain have no
													common format
												</span>
											)}
										</div>
									</td>
									<td className="px-6 py-4 text-sm">
										<div className="flex items-center gap-2">
											<code
												className="font-mono text-xs text-[var(--color-muted)] bg-[var(--color-surface-soft)] px-2 py-1 rounded truncate block max-w-[180px]"
												title={proxyUrl(l.path)}
											>
												{proxyUrl(l.path)}
											</code>
											<button
												onClick={() => copyUrl(l.path)}
												className="inline-flex items-center justify-center w-6 h-6 rounded-md cursor-pointer hover:bg-[var(--color-surface-soft)] text-[var(--color-muted)] hover:text-[var(--color-ink)] transition-colors shrink-0"
												title="Copy URL"
											>
												<svg
													width="14"
													height="14"
													viewBox="0 0 24 24"
													fill="none"
													stroke="currentColor"
													strokeWidth="2"
													strokeLinecap="round"
													strokeLinejoin="round"
												>
													<rect
														x="9"
														y="9"
														width="13"
														height="13"
														rx="2"
														ry="2"
													/>
													<path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
												</svg>
											</button>
										</div>
									</td>
									<td className="px-6 py-4 text-sm">
										<div className="flex items-center flex-wrap gap-1.5">
											{l.chain?.map((c, i) => {
												const hasOverride = !!c.api_key;
												const failed = c.validation_ok === false;
												return (
													<span key={i} className="flex items-center gap-1.5">
														{i > 0 && (
															<svg
																width="14"
																height="14"
																viewBox="0 0 24 24"
																fill="none"
																stroke="var(--color-muted)"
																strokeWidth="2.5"
																strokeLinecap="round"
																strokeLinejoin="round"
																className="shrink-0"
															>
																<path d="M5 12h14M13 5l7 7-7 7" />
															</svg>
														)}
														<span
															className={`${failed ? "grayscale opacity-40" : ""}`}
														>
															<Badge
																color={
																	failed
																		? "error"
																		: i === 0
																			? "violet"
																			: i === l.chain!.length - 1
																				? "emerald"
																				: "orange"
																}
															>
																{providerName(c.provider_id)}
																{hasOverride && " *"}
															</Badge>
														</span>
													</span>
												);
											})}
										</div>
									</td>
									<td className="px-6 py-4 sticky right-0 bg-[var(--color-canvas)]">
										<div className="flex gap-2">
											<Button
												variant="secondary"
												size="sm"
												onClick={() => testLink(l.id)}
												disabled={testingLinkID === l.id}
												title="Run a Link Test against every provider in this chain"
											>
												{testingLinkID === l.id ? "Testing…" : "Test"}
											</Button>
											<Button
												variant="ghost"
												size="sm"
												onClick={() => navigate(`/links/edit/${l.id}`)}
											>
												Edit
											</Button>
											<Button
												variant="ghost"
												size="sm"
												onClick={() => remove(l.id)}
												className="!text-[var(--color-error)]"
											>
												Del
											</Button>
										</div>
									</td>
								</tr>
							))}
						</tbody>
					</table>
				</div>
			)}
		</div>
	);
}
