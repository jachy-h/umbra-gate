import { useEffect, useState } from "react";
import { api } from "../api";
import type { Provider, ProxyLink, ChainEntry, ModelPriority } from "../types";
import { Button } from "../components/Button";
import { Input } from "../components/Input";
import { SearchableSelect } from "../components/SearchableSelect";
import { Spinner } from "../components/Spinner";
import { Modal } from "../components/Modal";
import { useTranslation } from "../i18n";

const blankChainEntry = (): ChainEntry => ({
	provider_id: "",
	protocol: "",
	retry_count: 1,
	model_priorities: [{ source: "request_model" }],
});

interface Props {
	link?: ProxyLink | null;
	onSaved: () => void;
	onCancel: () => void;
	onDirtyChange: (dirty: boolean) => void;
}

export function LinkEditor({ link, onSaved, onCancel, onDirtyChange }: Props) {
	const { t } = useTranslation();
	const [providers, setProviders] = useState<Provider[]>([]);
	const [loading, setLoading] = useState(true);
	const [name, setName] = useState("");
	const [path, setPath] = useState("");
	const [chain, setChain] = useState<ChainEntry[]>([]);
	const [enabled, setEnabled] = useState(true);
	const [saving, setSaving] = useState(false);
	const [error, setError] = useState("");
	const [keyProvider, setKeyProvider] = useState<Provider | null>(null);
	const [providerAPIKey, setProviderAPIKey] = useState("");
	const [savingProviderKey, setSavingProviderKey] = useState(false);
	const [providerKeyError, setProviderKeyError] = useState("");
	const [refreshedModelProviderIDs, setRefreshedModelProviderIDs] = useState<
		Set<string>
	>(() => new Set());
	const [refreshingModelProviderIDs, setRefreshingModelProviderIDs] = useState<
		Set<string>
	>(() => new Set());
	const [dirty, setDirty] = useState(false);
	const [validatingChainIndex, setValidatingChainIndex] = useState<
		number | null
	>(null);

	const markDirty = () => {
		if (!dirty) {
			setDirty(true);
			onDirtyChange(true);
		}
	};

	const isEditing = !!link;
	const configuredChain = chain.filter((entry) => entry.provider_id);
	const isValidatingChain = saving;

	useEffect(() => {
		api
			.listProviders()
			.then((p) => {
				setProviders(p);
				if (link) {
					setName(link.name);
					setPath(link.path);
					setEnabled(link.enabled);
					setChain(
						link.chain?.length
							? link.chain.map((entry) => ({
									...entry,
									protocol: entry.protocol || "",
									model_priorities: entry.model_priorities?.length
										? entry.model_priorities
										: [{ source: "request_model" }],
								}))
							: [blankChainEntry()],
					);
				} else {
					setChain([blankChainEntry()]);
				}
			})
			.catch(console.error)
			.finally(() => {
				setDirty(false);
				onDirtyChange(false);
				setLoading(false);
			});
	}, [link, onDirtyChange]);

	useEffect(() => {
		if (!dirty) return;
		const preventUnload = (event: BeforeUnloadEvent) => {
			event.preventDefault();
			event.returnValue = "";
		};
		window.addEventListener("beforeunload", preventUnload);
		return () => window.removeEventListener("beforeunload", preventUnload);
	}, [dirty]);

	const updateChain = (
		i: number,
		field: keyof ChainEntry,
		value: string | number,
	) => {
		markDirty();
		setChain((prev) =>
			prev.map((c, idx) => (idx === i ? { ...c, [field]: value } : c)),
		);
	};

	const addChainEntry = () => {
		markDirty();
		setChain((prev) => [...prev, blankChainEntry()]);
	};

	const updatePriorities = (
		chainIndex: number,
		updater: (priorities: ModelPriority[]) => ModelPriority[],
	) => {
		markDirty();
		setChain((current) =>
			current.map((entry, index) =>
				index === chainIndex
					? { ...entry, model_priorities: updater(entry.model_priorities) }
					: entry,
			),
		);
	};

	const addRequestModel = (chainIndex: number) => {
		updatePriorities(chainIndex, (priorities) => [
			...priorities,
			{ source: "request_model" },
		]);
	};

	const addFixedModel = (chainIndex: number) => {
		updatePriorities(chainIndex, (priorities) => [
			...priorities,
			{ source: "fixed_model", model: "" },
		]);
	};

	const updateFixedModel = (
		chainIndex: number,
		priorityIndex: number,
		model: string,
	) => {
		updatePriorities(chainIndex, (priorities) =>
			priorities.map((priority, index) =>
				index === priorityIndex ? { ...priority, model } : priority,
			),
		);
	};

	const removePriority = (chainIndex: number, priorityIndex: number) => {
		updatePriorities(chainIndex, (priorities) =>
			priorities.filter((_, index) => index !== priorityIndex),
		);
	};

	const movePriority = (
		chainIndex: number,
		priorityIndex: number,
		direction: "up" | "down",
	) => {
		updatePriorities(chainIndex, (priorities) => {
			const nextIndex =
				direction === "up" ? priorityIndex - 1 : priorityIndex + 1;
			if (nextIndex < 0 || nextIndex >= priorities.length) return priorities;
			const reordered = [...priorities];
			const [moved] = reordered.splice(priorityIndex, 1);
			reordered.splice(nextIndex, 0, moved);
			return reordered;
		});
	};

	const openProviderKeyModal = (provider: Provider) => {
		setKeyProvider(provider);
		setProviderAPIKey("");
		setProviderKeyError("");
	};

	const saveProviderKey = async () => {
		if (!keyProvider) return;
		setSavingProviderKey(true);
		setProviderKeyError("");
		try {
			const updated = await api.updateProviderAPIKey(
				keyProvider.id,
				providerAPIKey,
			);
			setProviders((current) =>
				current.map((provider) =>
					provider.id === updated.id ? updated : provider,
				),
			);
			setKeyProvider(null);
		} catch (e: unknown) {
			setProviderKeyError(
				e instanceof Error ? e.message : t.linkEditor.saveFailed,
			);
		} finally {
			setSavingProviderKey(false);
		}
	};

	const refreshProviderModels = async (providerID: string) => {
		if (refreshedModelProviderIDs.has(providerID)) {
			return providers.find((provider) => provider.id === providerID) ?? null;
		}
		if (refreshingModelProviderIDs.has(providerID)) return null;
		setRefreshingModelProviderIDs((current) =>
			new Set(current).add(providerID),
		);
		try {
			const updated = await api.refreshProviderModels(providerID);
			setProviders((current) =>
				current.map((provider) =>
					provider.id === updated.id ? updated : provider,
				),
			);
			setRefreshedModelProviderIDs((current) =>
				new Set(current).add(providerID),
			);
			return updated;
		} catch (e: unknown) {
			setProviders((current) =>
				current.map((provider) =>
					provider.id === providerID ? { ...provider, models: [] } : provider,
				),
			);
			setError(
				e instanceof Error
					? `${t.providers.refreshModelsFailed}: ${e.message}`
					: t.providers.refreshModelsFailed,
			);
			return null;
		} finally {
			setRefreshingModelProviderIDs((current) => {
				const next = new Set(current);
				next.delete(providerID);
				return next;
			});
		}
	};

	const removeChainEntry = (i: number) => {
		markDirty();
		setChain((prev) =>
			prev.length === 1
				? [blankChainEntry()]
				: prev.filter((_, idx) => idx !== i),
		);
	};

	const moveChainEntry = (i: number, direction: "up" | "down") => {
		const newIndex = direction === "up" ? i - 1 : i + 1;
		if (newIndex < 0 || newIndex >= chain.length) return;
		markDirty();
		const reordered = [...chain];
		const [moved] = reordered.splice(i, 1);
		reordered.splice(newIndex, 0, moved);
		setChain(reordered);
	};

	const save = async () => {
		if (!chain[0]?.provider_id) {
			setError(t.linkEditor.selectProviderError);
			return;
		}
		setSaving(true);
		setValidatingChainIndex(null);
		setError("");
		try {
			const payload: Partial<ProxyLink> = {
				name,
				path: path || undefined,
				chain: configuredChain,
				enabled,
			};
			if (link) payload.id = link.id;
			await api.createLinkStream(payload, (progress) => {
				setValidatingChainIndex(progress.chain_index);
			});
			setDirty(false);
			onDirtyChange(false);
			onSaved();
		} catch (e: unknown) {
			setError(e instanceof Error ? e.message : t.linkEditor.saveFailed);
		} finally {
			setSaving(false);
			setValidatingChainIndex(null);
		}
	};

	if (loading) return <Spinner />;

	const fieldCls =
		"h-10 px-3.5 rounded-md border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] text-sm outline-none transition-colors focus:border-[var(--color-ink)] w-full";
	const labelCls =
		"text-[11px] font-medium uppercase tracking-wide text-[var(--color-muted)]";

	const providerName = (id: string) =>
		providers.find((p) => p.id === id)?.name || id;
	const selectProvider = async (index: number, providerID: string) => {
		const providerChanged = chain[index]?.provider_id !== providerID;
		markDirty();
		setChain((current) =>
			current.map((entry, entryIndex) =>
				entryIndex === index
					? { ...entry, provider_id: providerID, protocol: "" }
					: entry,
			),
		);
		const refreshedProvider = await refreshProviderModels(providerID);
		if (!providerChanged) return;
		const availableModels = new Set(refreshedProvider?.models ?? []);
		setChain((current) =>
			current.map((entry, entryIndex) => {
				if (entryIndex !== index || entry.provider_id !== providerID)
					return entry;
				return {
					...entry,
					model_priorities: entry.model_priorities.map((priority) =>
						priority.source === "fixed_model" &&
						priority.model &&
						!availableModels.has(priority.model)
							? { ...priority, model: "" }
							: priority,
					),
				};
			}),
		);
	};

	return (
		<div className="animate-fade-in">
			<div className="mb-8">
				<h1 className="text-[28px] font-semibold leading-[1.2] tracking-[-0.5px] text-[var(--color-ink)]">
					{isEditing ? t.linkEditor.editTitle : t.linkEditor.newTitle}
				</h1>
				<p className="mt-2 text-[var(--color-muted)] text-base">
					{t.linkEditor.subtitle}
				</p>
			</div>

			<div className="flex gap-8 items-start">
				{/* Left column: Basic info */}
				<div className="w-[360px] shrink-0 flex flex-col gap-6">
					<div className="rounded-xl border border-[var(--color-hairline)] bg-[var(--color-canvas)] p-6 space-y-5">
						<Input
							label={t.linkEditor.nameLabel}
							value={name}
							onChange={(e) => {
								markDirty();
								setName(e.target.value);
							}}
							placeholder={t.linkEditor.namePlaceholder}
						/>

						<Input
							label={t.linkEditor.pathLabel}
							value={path}
							onChange={(e) => {
								markDirty();
								setPath(e.target.value);
							}}
							placeholder={t.linkEditor.pathPlaceholder}
							disabled={isEditing}
						/>

						<div className="flex items-center justify-between">
							<span className="text-sm font-medium text-[var(--color-ink)]">
								{t.linkEditor.enabled}
							</span>
							<button
								type="button"
								onClick={() => {
									markDirty();
									setEnabled(!enabled);
								}}
								className={`relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none ${
									enabled
										? "bg-[var(--color-primary)]"
										: "bg-[var(--color-hairline)]"
								}`}
							>
								<span
									className={`pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out ${
										enabled ? "translate-x-5" : "translate-x-0"
									}`}
								/>
							</button>
						</div>
					</div>

					<div
						className={`rounded-xl border bg-[var(--color-canvas)] p-6 transition-[border-color,box-shadow,background-color] duration-300 ${
							isValidatingChain
								? "border-[var(--color-primary)] bg-[var(--color-primary)]/[0.04] ring-4 ring-[var(--color-primary)]/10 shadow-[0_10px_30px_rgba(0,0,0,0.08)]"
								: "border-[var(--color-hairline)]"
						}`}
					>
						<div className="mb-4 rounded-lg border border-[var(--color-hairline)] bg-[var(--color-surface-soft)] px-4 py-3">
							<p className="text-[10px] font-bold uppercase tracking-[0.18em] text-[var(--color-muted)]">
								{t.linkEditor.capabilityTitle}
							</p>
							<p className="mt-1 text-sm font-semibold text-[var(--color-ink)]">
								{t.linkEditor.capabilityDesc}
							</p>
						</div>
						<div className="mb-3 flex items-center justify-between gap-3">
							<h3 className="text-sm font-semibold text-[var(--color-ink)]">
								{t.linkEditor.chainPreview}
							</h3>
							{validatingChainIndex !== null && (
								<span
									className="inline-flex items-center gap-1.5 rounded-full bg-[var(--color-primary)] px-2.5 py-1 text-[10px] font-bold uppercase tracking-wide text-[var(--color-on-primary)] animate-pulse"
									aria-live="polite"
								>
									<span className="h-1.5 w-1.5 rounded-full bg-current" />
									{t.linkEditor.validatingStep
										.replace("%d", String(validatingChainIndex + 1))
										.replace("%d", String(configuredChain.length))}
								</span>
							)}
						</div>
						{configuredChain.length === 0 ? (
							<p className="text-sm text-[var(--color-muted)]">
								{t.linkEditor.noProvidersInChain}
							</p>
						) : (
							<div className="space-y-2.5">
								{configuredChain.map((c, i, arr) => {
									const isCurrentValidation =
										isValidatingChain && validatingChainIndex === i;
									return (
										<div
											key={c.provider_id + i}
											className="relative flex gap-3 transition-all duration-500"
										>
											<div className="flex w-6 shrink-0 flex-col items-center">
												<span
													className={`flex h-6 w-6 items-center justify-center rounded-full text-[10px] font-bold transition-all duration-300 ${
														isCurrentValidation
															? "scale-110 bg-[var(--color-primary)] text-[var(--color-on-primary)] shadow-[0_0_0_5px_var(--color-primary)]/20 animate-pulse"
															: i === 0
																? "bg-[var(--color-badge-violet)] text-white"
																: i === arr.length - 1
																	? "bg-[var(--color-badge-emerald)] text-white"
																	: "bg-[var(--color-badge-orange)] text-white"
													}`}
												>
													{i + 1}
												</span>
												{i < arr.length - 1 && (
													<span
														className={`mt-1 h-full min-h-5 w-px transition-colors duration-300 ${
															isCurrentValidation
																? "bg-[var(--color-primary)] animate-pulse"
																: "bg-[var(--color-hairline)]"
														}`}
													/>
												)}
											</div>
											<div
												className={`min-w-0 flex-1 rounded-lg border bg-[var(--color-surface-soft)] px-3 py-2.5 transition-all duration-300 ${
													isCurrentValidation
														? "border-[var(--color-primary)] bg-[var(--color-primary)]/[0.07] shadow-[0_0_0_3px_var(--color-primary)]/10"
														: "border-[var(--color-hairline-soft)]"
												}`}
											>
												<div className="flex items-center justify-between gap-2">
													<p className="truncate text-sm font-semibold text-[var(--color-ink)]">
														{providerName(c.provider_id)}
													</p>
													{isCurrentValidation && (
														<span className="shrink-0 text-[10px] font-bold uppercase tracking-wide text-[var(--color-primary)] animate-pulse">
															{t.linkEditor.validating}
														</span>
													)}
												</div>
												<div className="mt-2 flex flex-wrap gap-1.5">
													{c.model_priorities.map((priority, priorityIndex) => (
														<span
															key={`${priority.source}-${priority.model}-${priorityIndex}`}
															className={`rounded px-2 py-0.5 text-[11px] font-medium ${
																priority.source === "request_model"
																	? "bg-[var(--color-canvas)] text-[var(--color-ink)]"
																	: "bg-[var(--color-hairline)] text-[var(--color-muted)]"
															}`}
														>
															{priority.source === "request_model"
																? t.linkEditor.requestModel
																: priority.model ||
																	t.linkEditor.fixedModelPlaceholder}
														</span>
													))}
												</div>
											</div>
										</div>
									);
								})}
							</div>
						)}
					</div>

					<div className="flex gap-3">
						<Button variant="secondary" onClick={onCancel} className="flex-1">
							{t.linkEditor.cancel}
						</Button>
						<Button
							onClick={save}
							disabled={
								saving ||
								!name ||
								chain.filter((c) => c.provider_id).length === 0
							}
							className="flex-1"
						>
							{saving ? t.linkEditor.saving : t.linkEditor.saveLink}
						</Button>
					</div>

					{error && (
						<p className="text-sm text-[var(--color-error)]">{error}</p>
					)}
				</div>

				{/* Right column: Provider Chain */}
				<div className="flex-1 min-w-0">
					<div className="flex items-center justify-between mb-5">
						<h2 className="text-base font-semibold text-[var(--color-ink)]">
							{t.linkEditor.providerChain}
						</h2>
						<Button variant="secondary" size="sm" onClick={addChainEntry}>
							{t.linkEditor.addStep}
						</Button>
					</div>

					<div className="space-y-3">
						{chain.map((entry, i) => {
							const provider = providers.find(
								(p) => p.id === entry.provider_id,
							);
							const isFirst = i === 0;
							const isLast = i === chain.length - 1;
							return (
								<div
									key={i}
									className="relative rounded-xl border border-[var(--color-hairline)] bg-[var(--color-canvas)] transition-colors"
								>
									{/* Header */}
									<div className="flex items-center justify-between px-5 py-3 border-b border-[var(--color-hairline-soft)]">
										<div className="flex items-center gap-3">
											<span
												className={`flex items-center justify-center w-7 h-7 rounded-full text-xs font-bold text-white ${
													isFirst
														? "bg-[var(--color-badge-violet)]"
														: isLast
															? "bg-[var(--color-badge-emerald)]"
															: "bg-[var(--color-badge-orange)]"
												}`}
											>
												{i + 1}
											</span>
											<span className="text-sm font-semibold text-[var(--color-ink)]">
												{isFirst
													? t.linkEditor.primary
													: isLast
														? t.linkEditor.finalFallback
														: t.linkEditor.fallbackNum.replace("%d", String(i))}
											</span>
										</div>
										<div className="flex items-center gap-2">
											{!isFirst && (
												<button
													onClick={() => moveChainEntry(i, "up")}
													className="inline-flex items-center justify-center w-7 h-7 rounded-md text-[var(--color-muted)] hover:text-[var(--color-ink)] hover:bg-[var(--color-surface-soft)] transition-colors cursor-pointer"
													title={t.linkEditor.moveUp}
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
														<path d="M18 15l-6-6-6 6" />
													</svg>
												</button>
											)}
											{!isLast && (
												<button
													onClick={() => moveChainEntry(i, "down")}
													className="inline-flex items-center justify-center w-7 h-7 rounded-md text-[var(--color-muted)] hover:text-[var(--color-ink)] hover:bg-[var(--color-surface-soft)] transition-colors cursor-pointer"
													title={t.linkEditor.moveDown}
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
														<path d="M6 9l6 6 6-6" />
													</svg>
												</button>
											)}
											<button
												onClick={() => removeChainEntry(i)}
												className="inline-flex items-center justify-center w-7 h-7 rounded-md text-[var(--color-error)] hover:bg-[var(--color-error)]/10 transition-colors cursor-pointer"
												title={t.linkEditor.remove}
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
													<path d="M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2m3 0v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6h14" />
												</svg>
											</button>
										</div>
									</div>

									{/* Body */}
									<div className="grid grid-cols-1 gap-4 p-5 lg:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]">
										<div className="space-y-4">
											<div className="space-y-1.5">
												<label className={labelCls}>
													{t.linkEditor.providerLabel}
												</label>
												<SearchableSelect
													options={providers.map((p) => ({
														label: p.name,
														value: p.id,
														hasAPIKey: p.has_api_key,
													}))}
													value={entry.provider_id}
													onChange={(v) => selectProvider(i, v)}
													placeholder={t.linkEditor.searchProvider}
												/>
											</div>
											<div className="space-y-1.5">
												<label className={labelCls}>
													{t.linkEditor.retryLabel}
												</label>
												<input
													type="number"
													value={String(entry.retry_count)}
													onChange={(e) =>
														updateChain(
															i,
															"retry_count",
															parseInt(e.target.value) || 0,
														)
													}
													className={fieldCls}
													min={0}
												/>
											</div>
										</div>

										<div className="rounded-lg border border-[var(--color-hairline)] bg-[var(--color-surface-soft)] p-4 space-y-3">
											<div className="flex items-center justify-between gap-3">
												<label className={labelCls}>
													{t.linkEditor.modelPriorities}
												</label>
												<span className="text-[11px] text-[var(--color-muted)]">
													{entry.model_priorities.length}
												</span>
											</div>
											{entry.model_priorities.map((priority, priorityIndex) => (
												<div
													key={priorityIndex}
													className="flex items-center gap-2 rounded-md border border-[var(--color-hairline-soft)] bg-[var(--color-canvas)] p-2.5"
												>
													<span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-[var(--color-surface-soft)] text-[11px] font-semibold text-[var(--color-muted)]">
														{priorityIndex + 1}
													</span>
													{priority.source === "request_model" ? (
														<span className="flex-1 text-sm font-medium text-[var(--color-ink)]">
															{t.linkEditor.requestModel}
														</span>
													) : (
														<div className="flex-1">
															<SearchableSelect
																options={(provider?.models || []).map(
																	(model) => ({
																		label: model,
																		value: model,
																	}),
																)}
																value={priority.model || ""}
																onChange={(model) =>
																	updateFixedModel(i, priorityIndex, model)
																}
																placeholder={t.linkEditor.fixedModelPlaceholder}
																onOpen={() =>
																	provider && refreshProviderModels(provider.id)
																}
																allowCustomValue
																className="h-8"
															/>
														</div>
													)}
													<button
														type="button"
														disabled={priorityIndex === 0}
														onClick={() => movePriority(i, priorityIndex, "up")}
														className="text-[11px] text-[var(--color-muted)] disabled:opacity-30 hover:text-[var(--color-ink)]"
													>
														↑
													</button>
													<button
														type="button"
														disabled={
															priorityIndex ===
															entry.model_priorities.length - 1
														}
														onClick={() =>
															movePriority(i, priorityIndex, "down")
														}
														className="text-[11px] text-[var(--color-muted)] disabled:opacity-30 hover:text-[var(--color-ink)]"
													>
														↓
													</button>
													<button
														type="button"
														disabled={entry.model_priorities.length === 1}
														onClick={() => removePriority(i, priorityIndex)}
														className="text-[11px] text-[var(--color-error)] hover:underline disabled:cursor-not-allowed disabled:opacity-30"
													>
														{t.linkEditor.remove}
													</button>
												</div>
											))}
											<div className="flex flex-wrap gap-2 pt-1">
												{!entry.model_priorities.some(
													(priority) => priority.source === "request_model",
												) && (
													<Button
														variant="secondary"
														size="sm"
														onClick={() => addRequestModel(i)}
													>
														{t.linkEditor.addRequestModel}
													</Button>
												)}
												<Button
													variant="secondary"
													size="sm"
													onClick={() => addFixedModel(i)}
												>
													{t.linkEditor.addFixedModel}
												</Button>
											</div>
										</div>

										{provider && !provider.has_api_key && (
											<div className="flex flex-col items-start gap-2 rounded-lg border border-[var(--color-hairline)] px-4 py-3">
												<span className="text-xs text-[var(--color-muted)]">
													{t.linkEditor.providerKeyMissing}
												</span>
												<button
													type="button"
													onClick={() => openProviderKeyModal(provider)}
													className="text-xs font-semibold text-[var(--color-ink)] underline underline-offset-4"
												>
													{t.linkEditor.configureProvider.replace(
														"%s",
														provider.name,
													)}
												</button>
											</div>
										)}
									</div>
								</div>
							);
						})}
					</div>
				</div>
			</div>
			<Modal
				open={keyProvider !== null}
				title={t.linkEditor.providerKeyModalTitle.replace(
					"%s",
					keyProvider?.name || "",
				)}
				onClose={() => !savingProviderKey && setKeyProvider(null)}
			>
				<div className="space-y-5">
					<Input
						label={t.providers.apiKeyLabel}
						type="text"
						autoComplete="off"
						name="provider-api-key"
						data-1p-ignore="true"
						data-lpignore="true"
						spellCheck={false}
						value={providerAPIKey}
						onChange={(event) => setProviderAPIKey(event.target.value)}
						placeholder={t.providers.apiKeyPlaceholder}
					/>
					{providerKeyError && (
						<p className="text-sm text-[var(--color-error)]">
							{providerKeyError}
						</p>
					)}
					<div className="flex justify-end gap-3">
						<Button
							variant="secondary"
							onClick={() => setKeyProvider(null)}
							disabled={savingProviderKey}
						>
							{t.linkEditor.cancel}
						</Button>
						<Button
							onClick={saveProviderKey}
							disabled={savingProviderKey || !providerAPIKey}
						>
							{savingProviderKey ? t.linkEditor.saving : t.providers.save}
						</Button>
					</div>
				</div>
			</Modal>
		</div>
	);
}
