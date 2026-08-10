import { useEffect, useState, useMemo } from "react";
import { api } from "../api";
import type { StatsRow, ProxyLink, RequestLog } from "../types";
import { Card } from "../components/Card";
import { Badge } from "../components/Badge";
import { Spinner } from "../components/Spinner";
import { SearchableSelect } from "../components/SearchableSelect";
import { RequestDetailsModal } from "../components/RequestDetailsModal";
import { useTranslation } from "../i18n";

function fmtNum(n: number) {
	if (n >= 1e6) return (n / 1e6).toFixed(1) + "M";
	if (n >= 1e3) return (n / 1e3).toFixed(1) + "K";
	return String(n);
}

function fmtMs(ms: number, count: number) {
	if (count === 0) return "—";
	return (ms / count).toFixed(0) + "ms";
}

function fmtRate(success: number, total: number) {
	if (total === 0) return "—";
	return ((success / total) * 100).toFixed(1) + "%";
}

function rangeStart(dateRange: string) {
	const hours: Record<string, number> = {
		"24h": 24,
		"7d": 24 * 7,
		"30d": 24 * 30,
	};
	if (!hours[dateRange]) return undefined;
	return new Date(Date.now() - hours[dateRange] * 60 * 60 * 1000).toISOString();
}

export function StatsDashboard() {
	const { t } = useTranslation();
	const [stats, setStats] = useState<StatsRow[]>([]);
	const [links, setLinks] = useState<ProxyLink[]>([]);
	const [requests, setRequests] = useState<RequestLog[]>([]);
	const [selectedRequest, setSelectedRequest] = useState<RequestLog | null>(
		null,
	);
	const [loading, setLoading] = useState(true);
	const [linkFilter, setLinkFilter] = useState("");
	const [dateRange, setDateRange] = useState("all");

	useEffect(() => {
		let cancelled = false;
		setLoading(true);

		Promise.all([
			api.getStats({
				link_id: linkFilter || undefined,
				from: rangeStart(dateRange),
			}),
			api.listLinks(),
			api.listRecentRequests(),
		])
			.then(([s, l, r]) => {
				if (cancelled) return;
				setStats(s.stats || []);
				setLinks(l);
				setRequests(r);
			})
			.catch((error) => {
				if (!cancelled) console.error(error);
			})
			.finally(() => {
				if (!cancelled) setLoading(false);
			});

		return () => {
			cancelled = true;
		};
	}, [linkFilter, dateRange]);

	const aggregated = useMemo(() => {
		const byLink: Record<
			string,
			{
				total: number;
				success: number;
				failure: number;
				lat: number;
				name: string;
			}
		> = {};
		let grandTotal = 0;
		let grandSuccess = 0;
		let grandFailure = 0;
		let grandLat = 0;

		for (const s of stats) {
			if (s.AttrKey !== "") continue; // skip attribute-level rows, use link-level aggregates
			grandTotal += s.Total;
			grandSuccess += s.Success;
			grandFailure += s.Failure;
			grandLat += s.Lat;

			if (!byLink[s.LinkID]) {
				const link = links.find((l) => l.id === s.LinkID);
				byLink[s.LinkID] = {
					total: 0,
					success: 0,
					failure: 0,
					lat: 0,
					name: link?.name || s.LinkID,
				};
			}
			byLink[s.LinkID].total += s.Total;
			byLink[s.LinkID].success += s.Success;
			byLink[s.LinkID].failure += s.Failure;
			byLink[s.LinkID].lat += s.Lat;
		}

		return { byLink, grandTotal, grandSuccess, grandFailure, grandLat };
	}, [stats, links]);

	return (
		<div className="space-y-8 animate-fade-in">
			<div>
				<h1 className="text-[28px] font-semibold leading-[1.2] tracking-[-0.5px] text-[var(--color-ink)]">
					{t.stats.title}
				</h1>
				<p className="mt-2 text-[var(--color-muted)] text-base">
					{t.stats.subtitle}
				</p>
			</div>

			{/* Filters */}
			<div className="flex flex-wrap items-end gap-4">
				<div className="w-64 space-y-1.5">
					<label className="text-[11px] font-medium uppercase tracking-wide text-[var(--color-muted)]">
						{t.stats.filterLink}
					</label>
					<SearchableSelect
						options={[
							{ label: t.stats.allLinks, value: "" },
							...links.map((link) => ({ label: link.name, value: link.id })),
						]}
						value={linkFilter}
						onChange={setLinkFilter}
						placeholder={t.stats.searchLinks}
					/>
				</div>
				<div className="w-48 space-y-1.5">
					<label className="text-[11px] font-medium uppercase tracking-wide text-[var(--color-muted)]">
						{t.stats.filterDateRange}
					</label>
					<SearchableSelect
						options={[
							{ label: t.stats.last24h, value: "24h" },
							{ label: t.stats.last7d, value: "7d" },
							{ label: t.stats.last30d, value: "30d" },
							{ label: t.stats.allTime, value: "all" },
						]}
						value={dateRange}
						onChange={setDateRange}
						placeholder={t.stats.selectRange}
					/>
				</div>
			</div>

			{loading ? (
				<Spinner />
			) : (
				<>
					{/* Overview cards */}
					<div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
						<Card>
							<p className="text-sm font-medium text-[var(--color-muted)] uppercase tracking-wide">
								{t.stats.totalRequests}
							</p>
							<p className="mt-2 text-[28px] font-semibold leading-[1.2] tracking-[-0.5px] text-[var(--color-ink)]">
								{fmtNum(aggregated.grandTotal)}
							</p>
						</Card>
						<Card>
							<p className="text-sm font-medium text-[var(--color-muted)] uppercase tracking-wide">
								{t.stats.successRate}
							</p>
							<p className="mt-2 text-[28px] font-semibold leading-[1.2] tracking-[-0.5px] text-[var(--color-success)]">
								{fmtRate(aggregated.grandSuccess, aggregated.grandTotal)}
							</p>
						</Card>
						<Card>
							<p className="text-sm font-medium text-[var(--color-muted)] uppercase tracking-wide">
								{t.stats.failures}
							</p>
							<p className="mt-2 text-[28px] font-semibold leading-[1.2] tracking-[-0.5px] text-[var(--color-error)]">
								{fmtNum(aggregated.grandFailure)}
							</p>
						</Card>
						<Card>
							<p className="text-sm font-medium text-[var(--color-muted)] uppercase tracking-wide">
								{t.stats.avgLatency}
							</p>
							<p className="mt-2 text-[28px] font-semibold leading-[1.2] tracking-[-0.5px] text-[var(--color-ink)]">
								{fmtMs(aggregated.grandLat, aggregated.grandTotal)}
							</p>
						</Card>
					</div>

					{/* Per-link table */}
					{Object.keys(aggregated.byLink).length > 0 && (
						<div className="overflow-hidden rounded-xl border border-[var(--color-hairline)] bg-[var(--color-canvas)]">
							<div className="px-8 py-5 border-b border-[var(--color-hairline-soft)]">
								<h3 className="text-lg font-semibold text-[var(--color-ink)]">
									{t.stats.byProxyLink}
								</h3>
							</div>
							<table className="w-full">
								<thead>
									<tr className="border-b border-[var(--color-hairline-soft)] text-left text-sm font-medium text-[var(--color-muted)]">
										<th className="px-8 py-3 font-medium">
											{t.stats.tableLink}
										</th>
										<th className="px-8 py-3 font-medium">
											{t.stats.tableTotal}
										</th>
										<th className="px-8 py-3 font-medium">
											{t.stats.tableSuccess}
										</th>
										<th className="px-8 py-3 font-medium">
											{t.stats.tableFailure}
										</th>
										<th className="px-8 py-3 font-medium">
											{t.stats.tableAvgLatency}
										</th>
										<th className="px-8 py-3 font-medium">
											{t.stats.tableSuccessRate}
										</th>
									</tr>
								</thead>
								<tbody>
									{Object.entries(aggregated.byLink).map(([id, row]) => (
										<tr
											key={id}
											className="border-b border-[var(--color-hairline-soft)] last:border-b-0 hover:bg-[var(--color-surface-soft)] transition-colors"
										>
											<td className="px-8 py-4 text-sm font-semibold text-[var(--color-ink)]">
												{row.name}
											</td>
											<td className="px-8 py-4 text-sm">{fmtNum(row.total)}</td>
											<td className="px-8 py-4 text-sm text-[var(--color-success)]">
												{fmtNum(row.success)}
											</td>
											<td className="px-8 py-4 text-sm text-[var(--color-error)]">
												{fmtNum(row.failure)}
											</td>
											<td className="px-8 py-4 text-sm">
												{fmtMs(row.lat, row.total)}
											</td>
											<td className="px-8 py-4 text-sm">
												<Badge
													color={
														row.success / Math.max(row.total, 1) >= 0.95
															? "success"
															: row.success / Math.max(row.total, 1) >= 0.8
																? "warning"
																: "error"
													}
												>
													{fmtRate(row.success, row.total)}
												</Badge>
											</td>
										</tr>
									))}
								</tbody>
							</table>
						</div>
					)}

					<div className="overflow-hidden rounded-xl border border-[var(--color-hairline)] bg-[var(--color-canvas)]">
						<div className="px-8 py-5 border-b border-[var(--color-hairline-soft)]">
							<h3 className="text-lg font-semibold text-[var(--color-ink)]">
								{t.stats.latestRequests}
							</h3>
							<p className="mt-1 text-sm text-[var(--color-muted)]">
								{t.stats.latestRequestsDesc}
							</p>
						</div>
						{requests.length === 0 ? (
							<p className="px-8 py-10 text-sm text-[var(--color-muted)]">
								{t.stats.noRequests}
							</p>
						) : (
							<div className="overflow-x-auto">
								<table className="w-full">
									<thead>
										<tr className="border-b border-[var(--color-hairline-soft)] text-left text-sm font-medium text-[var(--color-muted)]">
											<th className="px-6 py-3 font-medium">
												{t.stats.tableTime}
											</th>
											<th className="px-6 py-3 font-medium">
												{t.stats.tableLink}
											</th>
											<th className="px-6 py-3 font-medium">
												{t.stats.tableProvider}
											</th>
											<th className="px-6 py-3 font-medium">
												{t.stats.tableModel}
											</th>
											<th className="px-6 py-3 font-medium">
												{t.stats.tableStatus}
											</th>
											<th className="px-6 py-3 font-medium">
												{t.stats.tableLatency}
											</th>
										</tr>
									</thead>
									<tbody>
										{requests.map((request) => {
											const link = links.find(
												(item) => item.id === request.link_id,
											);
											const isLinkTest =
												request.attributes?._request_type === "link_validation";
											return (
												<tr
													key={request.id}
													className="border-b border-[var(--color-hairline-soft)] last:border-b-0 cursor-pointer hover:bg-[var(--color-surface-soft)] transition-colors focus:outline-none focus:bg-[var(--color-surface-soft)]"
													title={t.stats.viewRequest}
													tabIndex={0}
													onClick={() => setSelectedRequest(request)}
													onKeyDown={(event) => {
														if (event.key === "Enter" || event.key === " ")
															setSelectedRequest(request);
													}}
												>
													<td className="px-6 py-3 text-xs whitespace-nowrap text-[var(--color-muted)]">
														{new Date(request.created_at).toLocaleString()}
													</td>
													<td className="px-6 py-3 text-sm font-medium text-[var(--color-ink)]">
														{link?.name || request.path}
													</td>
													<td className="px-6 py-3 text-sm">
														<span className="flex items-center gap-2">
															{request.provider_name}
															{isLinkTest && (
																<Badge color="default">
																	{t.stats.linkTestBadge}
																</Badge>
															)}
														</span>
													</td>
													<td className="px-6 py-3 text-sm font-mono">
														{request.model || "—"}
													</td>
													<td className="px-6 py-3">
														<Badge
															color={
																request.status_code >= 200 &&
																request.status_code < 300
																	? "success"
																	: "error"
															}
														>
															{request.status_code || t.stats.errStatus}
														</Badge>
													</td>
													<td className="px-6 py-3 text-sm">
														{request.latency_ms}ms
													</td>
												</tr>
											);
										})}
									</tbody>
								</table>
							</div>
						)}
					</div>

					<RequestDetailsModal
						request={selectedRequest}
						onClose={() => setSelectedRequest(null)}
					/>
				</>
			)}
		</div>
	);
}
