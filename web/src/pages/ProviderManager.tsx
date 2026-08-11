import { useEffect, useState } from "react";
import { api } from "../api";
import type {
	EndpointFormat,
	Provider,
	ProviderEndpoint,
	ProviderProtocol,
} from "../types";
import { Card } from "../components/Card";
import { Button } from "../components/Button";
import { Input, Select } from "../components/Input";
import { Modal } from "../components/Modal";
import { Spinner } from "../components/Spinner";
import { formatOptions, protocolLabel, protocolOptions } from "../protocols";
import { useTranslation } from "../i18n";

const defaultEndpoint = (): ProviderEndpoint => ({
	protocol: "openai",
	request_format: "chat_completions",
	response_format: "chat_completions",
	base_url: "",
});

export function ProviderManager() {
	const { t } = useTranslation();
	const [providers, setProviders] = useState<Provider[]>([]);
	const [types, setTypes] = useState<string[]>([]);
	const [loading, setLoading] = useState(true);
	const [modalOpen, setModalOpen] = useState(false);
	const [editing, setEditing] = useState<Provider | null>(null);
	const [form, setForm] = useState<{
		name: string;
		type: string;
		endpoints: ProviderEndpoint[];
		api_key: string;
		models: string;
	}>({
		name: "",
		type: "custom",
		endpoints: [defaultEndpoint()],
		api_key: "",
		models: "",
	});
	const [saving, setSaving] = useState(false);
	const [error, setError] = useState("");
	const [keyEditingId, setKeyEditingId] = useState<string | null>(null);
	const [keyValue, setKeyValue] = useState("");
	const [modelsProvider, setModelsProvider] = useState<Provider | null>(null);
	const [modelSearch, setModelSearch] = useState("");

	const fetchAll = () => {
		setLoading(true);
		Promise.all([api.listProviders(), api.getTypes()])
			.then(([p, t]) => {
				setProviders(p);
				setTypes(t.types);
			})
			.catch(console.error)
			.finally(() => setLoading(false));
	};

	useEffect(() => {
		fetchAll();
	}, []);

	const openCreate = () => {
		setEditing(null);
		setForm({
			name: "",
			type: "custom",
			endpoints: [defaultEndpoint()],
			api_key: "",
			models: "",
		});
		setError("");
		setModalOpen(true);
	};

	const openEdit = (p: Provider) => {
		setEditing(p);
		setForm({
			name: p.name,
			type: p.type,
			endpoints: p.endpoints?.length
				? p.endpoints
				: [{ ...defaultEndpoint(), base_url: p.base_url }],
			api_key: "",
			models: (p.models || []).join(", "),
		});
		setError("");
		setModalOpen(true);
	};

	const save = async () => {
		setSaving(true);
		setError("");
		try {
			const payload: Partial<Provider> = {
				name: form.name,
				type: form.type,
				base_url: form.endpoints[0]?.base_url || "",
				endpoints: form.endpoints,
				models: form.models
					.split(",")
					.map((s) => s.trim())
					.filter(Boolean),
				enabled: true,
			};
			if (form.api_key) payload.api_key = form.api_key;
			if (editing) {
				payload.id = editing.id;
			}
			await api.createProvider(payload);
			setModalOpen(false);
			fetchAll();
		} catch (e: unknown) {
			setError(e instanceof Error ? e.message : t.providers.saveFailed);
		} finally {
			setSaving(false);
		}
	};

	const startKeyEdit = (p: Provider) => {
		setKeyEditingId(p.id);
		setKeyValue("");
	};

	const saveKey = async (p: Provider) => {
		setSaving(true);
		try {
			await api.updateProviderAPIKey(p.id, keyValue);
			setKeyEditingId(null);
			fetchAll();
		} catch (e: unknown) {
			alert(e instanceof Error ? e.message : t.providers.saveFailed);
		} finally {
			setSaving(false);
		}
	};

	const remove = async (id: string) => {
		if (!confirm(t.providers.deleteConfirm)) return;
		try {
			await api.deleteProvider(id);
			fetchAll();
		} catch (e: unknown) {
			alert(e instanceof Error ? e.message : t.providers.deleteFailed);
		}
	};

	const typeOptions = types.map((t) => ({ label: t, value: t }));
	const endpointProtocolOptions = protocolOptions.map((option) => ({
		label:
			option.value === "openai" ? t.providers.openai : t.providers.anthropic,
		value: option.value,
	}));
	const endpointFormatOptions = (protocol: ProviderProtocol) =>
		formatOptions
			.filter((option) =>
				protocol === "anthropic"
					? option.value === "messages"
					: option.value !== "messages",
			)
			.map((option) => ({
				label:
					option.value === "chat_completions" ? t.providers.chat : option.label,
				value: option.value,
			}));

	const updateEndpoint = (
		index: number,
		field: keyof ProviderEndpoint,
		value: string,
	) => {
		setForm((current) => ({
			...current,
			endpoints: current.endpoints.map((endpoint, endpointIndex) =>
				endpointIndex === index
					? field === "protocol"
						? value === "anthropic"
							? {
									...endpoint,
									protocol: value as ProviderProtocol,
									request_format: "messages",
									response_format: "messages",
								}
							: {
									...endpoint,
									protocol: value as ProviderProtocol,
									request_format: "chat_completions",
									response_format: "chat_completions",
								}
						: field === "request_format" || field === "response_format"
							? { ...endpoint, [field]: value as EndpointFormat }
							: { ...endpoint, base_url: value }
					: endpoint,
			),
		}));
	};

	const addEndpoint = () => {
		const candidates: ProviderEndpoint[] = [
			defaultEndpoint(),
			{
				protocol: "openai",
				request_format: "responses",
				response_format: "responses",
				base_url: "",
			},
			{
				protocol: "anthropic",
				request_format: "messages",
				response_format: "messages",
				base_url: "",
			},
		];
		const unused = candidates.find(
			(candidate) =>
				!form.endpoints.some(
					(endpoint) =>
						endpoint.protocol === candidate.protocol &&
						endpoint.response_format === candidate.response_format,
				),
		);
		if (!unused) return;
		setForm((current) => ({
			...current,
			endpoints: [...current.endpoints, unused],
		}));
	};

	const removeEndpoint = (index: number) => {
		if (form.endpoints.length === 1) return;
		setForm((current) => ({
			...current,
			endpoints: current.endpoints.filter(
				(_, endpointIndex) => endpointIndex !== index,
			),
		}));
	};

	const matchingModels = (modelsProvider?.models ?? []).filter((model) =>
		model.toLowerCase().includes(modelSearch.trim().toLowerCase()),
	);

	return (
		<div className="space-y-8 animate-fade-in">
			<div className="flex items-center justify-between">
				<div>
					<h1 className="text-[28px] font-semibold leading-[1.2] tracking-[-0.5px] text-[var(--color-ink)]">
						{t.providers.title}
					</h1>
					<p className="mt-2 text-[var(--color-muted)] text-base">
						{t.providers.subtitle}
					</p>
				</div>
				<Button onClick={openCreate}>{t.providers.addProvider}</Button>
			</div>

			{loading ? (
				<Spinner />
			) : providers.length === 0 ? (
				<Card className="text-center text-[var(--color-muted)] py-16">
					{t.providers.noProviders}
				</Card>
			) : (
				<div className="overflow-hidden rounded-xl border border-[var(--color-hairline)] bg-[var(--color-canvas)]">
					<table className="w-full">
						<thead>
							<tr className="border-b border-[var(--color-hairline-soft)] text-left text-sm font-medium text-[var(--color-muted)]">
								<th className="px-8 py-3 font-medium">
									{t.providers.tableName}
								</th>
								<th className="px-8 py-3 font-medium">
									{t.providers.tableEndpoints}
								</th>
								<th className="px-8 py-3 font-medium">
									{t.providers.tableApiKey}
								</th>
								<th className="px-8 py-3 font-medium w-24" />
							</tr>
						</thead>
						<tbody>
							{providers.map((p) => (
								<tr
									key={p.id}
									className="border-b border-[var(--color-hairline-soft)] last:border-b-0 hover:bg-[var(--color-surface-soft)] transition-colors"
								>
									<td className="px-8 py-4">
										<span className="text-sm font-semibold text-[var(--color-ink)]">
											{p.name}
										</span>
										{p.builtin && (
											<span className="ml-2">
												<span className="inline-flex items-center rounded-full px-2.5 py-0.5 text-[13px] font-medium bg-[var(--color-surface-card)] text-[var(--color-muted)]">
													{t.providers.builtin}
												</span>
											</span>
										)}
									</td>
									<td className="px-8 py-4">
										<div className="space-y-1.5">
											{(p.endpoints || []).map((endpoint) => (
												<div
													key={`${endpoint.protocol}-${endpoint.response_format}`}
													className="flex items-center gap-2 min-w-0"
												>
													<span className="shrink-0 rounded border border-[var(--color-hairline)] bg-[var(--color-surface-soft)] px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-[var(--color-muted)]">
														{protocolLabel(endpoint.protocol)}
													</span>
													<span
														className="max-w-[360px] truncate font-mono text-xs text-[var(--color-muted)]"
														title={endpoint.base_url}
													>
														{endpoint.base_url}
													</span>
												</div>
											))}
										</div>
									</td>
									<td className="px-8 py-4">
										{keyEditingId === p.id ? (
											<div className="flex items-center gap-2">
												<input
													type="text"
													autoComplete="off"
													name={`provider-api-key-${p.id}`}
													data-1p-ignore="true"
													data-lpignore="true"
													spellCheck={false}
													value={keyValue}
													onChange={(e) => setKeyValue(e.target.value)}
													className="h-8 w-48 px-2.5 rounded-md border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-sm outline-none focus:border-[var(--color-ink)]"
													placeholder="sk-..."
													autoFocus
													onKeyDown={(e) => {
														if (e.key === "Enter") saveKey(p);
														if (e.key === "Escape") setKeyEditingId(null);
													}}
												/>
												<Button
													size="sm"
													onClick={() => saveKey(p)}
													disabled={saving}
												>
													{t.providers.save}
												</Button>
												<Button
													size="sm"
													variant="ghost"
													onClick={() => setKeyEditingId(null)}
												>
													{t.providers.cancel}
												</Button>
											</div>
										) : (
											<button
												onClick={() => startKeyEdit(p)}
												className="text-sm text-[var(--color-muted)] cursor-pointer hover:text-[var(--color-ink)] transition-colors"
												title={
													p.has_api_key
														? t.providers.changeKey
														: t.providers.setKey
												}
											>
												{p.has_api_key
													? t.providers.maskedKey
													: t.providers.notSet}
											</button>
										)}
									</td>
									<td className="px-8 py-4">
										<div className="flex gap-2">
											<Button
												variant="ghost"
												size="sm"
												onClick={() => {
													setModelSearch("");
													setModelsProvider(p);
												}}
												className="min-w-[76px]"
											>
												{t.providers.showModels}
											</Button>
											{!p.builtin && (
												<Button
													variant="ghost"
													size="sm"
													onClick={() => openEdit(p)}
													className="min-w-[60px]"
												>
													{t.providers.edit}
												</Button>
											)}
											{!p.builtin && (
												<Button
													variant="ghost"
													size="sm"
													onClick={() => remove(p.id)}
													className="min-w-[60px] !text-[var(--color-error)]"
												>
													{t.providers.del}
												</Button>
											)}
										</div>
									</td>
								</tr>
							))}
						</tbody>
					</table>
				</div>
			)}

			<Modal
				open={modelsProvider !== null}
				title={`${modelsProvider?.name ?? ""} · ${t.providers.modelsLabel}`}
				onClose={() => setModelsProvider(null)}
				className="!max-w-xl !h-[70vh] flex flex-col"
			>
				<div className="flex min-h-0 flex-1 flex-col gap-3">
					<Input
						label={t.providers.searchModels}
						value={modelSearch}
						onChange={(event) => setModelSearch(event.target.value)}
						placeholder={t.providers.searchModels}
						autoFocus
					/>
					<div className="min-h-0 flex-1">
						{modelsProvider?.models?.length ? (
							matchingModels.length ? (
								<ul className="h-full space-y-1 overflow-y-auto rounded-lg border border-[var(--color-hairline)] bg-[var(--color-surface-soft)] p-2">
									{matchingModels.map((model) => (
										<li
											key={model}
											className="rounded-md bg-[var(--color-canvas)] px-3 py-2 font-mono text-sm text-[var(--color-ink)]"
										>
											{model}
										</li>
									))}
								</ul>
							) : (
								<p className="flex h-full items-center text-sm text-[var(--color-muted)]">
									{t.providers.noMatchingModels}
								</p>
							)
						) : (
							<p className="flex h-full items-center text-sm text-[var(--color-muted)]">
								{t.providers.noModels}
							</p>
						)}
					</div>
				</div>
				<div className="mt-5 flex justify-end">
					<Button variant="secondary" onClick={() => setModelsProvider(null)}>
						{t.providers.cancel}
					</Button>
				</div>
			</Modal>

			<Modal
				open={modalOpen}
				title={editing ? t.providers.editProvider : t.providers.newProvider}
				onClose={() => setModalOpen(false)}
			>
				<div className="space-y-4">
					<Input
						label={t.providers.nameLabel}
						value={form.name}
						onChange={(e) => setForm({ ...form, name: e.target.value })}
						placeholder={t.providers.namePlaceholder}
					/>
					<Select
						label={t.providers.typeLabel}
						options={typeOptions}
						value={form.type}
						onChange={(e) => setForm({ ...form, type: e.target.value })}
					/>
					<div className="rounded-lg border border-[var(--color-hairline)] bg-[var(--color-surface-soft)] p-4 space-y-3">
						<div className="flex items-center justify-between">
							<div>
								<p className="text-sm font-semibold text-[var(--color-ink)]">
									{t.providers.endpointsLabel}
								</p>
								<p className="mt-0.5 text-xs text-[var(--color-muted)]">
									{t.providers.endpointsDesc}
								</p>
							</div>
							<Button
								variant="secondary"
								size="sm"
								onClick={addEndpoint}
								disabled={form.endpoints.length >= 3}
							>
								{t.providers.addEndpoint}
							</Button>
						</div>
						{form.endpoints.map((endpoint, index) => (
							<div
								key={index}
								className="relative rounded-lg border border-[var(--color-hairline)] bg-[var(--color-canvas)] p-3 pr-11"
							>
								<div className="grid grid-cols-3 items-end gap-2">
									<Select
										label={t.providers.styleLabel}
										options={endpointProtocolOptions}
										value={endpoint.protocol}
										onChange={(event) =>
											updateEndpoint(index, "protocol", event.target.value)
										}
									/>
									<Select
										label={t.providers.requestLabel}
										options={endpointFormatOptions(endpoint.protocol)}
										value={endpoint.request_format}
										onChange={(event) =>
											updateEndpoint(
												index,
												"request_format",
												event.target.value,
											)
										}
									/>
									<Select
										label={t.providers.responseLabel}
										options={endpointFormatOptions(endpoint.protocol)}
										value={endpoint.response_format}
										onChange={(event) =>
											updateEndpoint(
												index,
												"response_format",
												event.target.value,
											)
										}
									/>
								</div>
								<div className="mt-3">
									<Input
										label={t.providers.baseUrlLabel}
										value={endpoint.base_url}
										onChange={(event) =>
											updateEndpoint(index, "base_url", event.target.value)
										}
										placeholder={
											endpoint.response_format === "responses"
												? t.providers.baseUrlPlaceholderResponses
												: t.providers.baseUrlPlaceholderDefault
										}
									/>
								</div>
								<button
									type="button"
									onClick={() => removeEndpoint(index)}
									disabled={form.endpoints.length === 1}
									className="absolute right-2 top-2 inline-flex h-8 w-8 items-center justify-center rounded-md text-[var(--color-error)] transition-colors hover:bg-red-50 disabled:cursor-not-allowed disabled:opacity-25"
									title={t.providers.removeEndpoint}
								>
									×
								</button>
							</div>
						))}
					</div>
					<Input
						label={t.providers.apiKeyLabel}
						type="text"
						autoComplete="off"
						name="provider-api-key-form"
						data-1p-ignore="true"
						data-lpignore="true"
						spellCheck={false}
						value={form.api_key}
						onChange={(e) => setForm({ ...form, api_key: e.target.value })}
						placeholder={t.providers.apiKeyPlaceholder}
					/>
					<Input
						label={t.providers.modelsLabel}
						value={form.models}
						onChange={(e) => setForm({ ...form, models: e.target.value })}
						placeholder={t.providers.modelsPlaceholder}
					/>
					{error && (
						<p className="text-sm text-[var(--color-error)]">{error}</p>
					)}
					<div className="flex justify-end gap-3 pt-2">
						<Button variant="secondary" onClick={() => setModalOpen(false)}>
							{t.providers.cancel}
						</Button>
						<Button
							onClick={save}
							disabled={
								saving ||
								!form.name ||
								form.endpoints.length === 0 ||
								form.endpoints.some((endpoint) => !endpoint.base_url)
							}
						>
							{saving ? t.providers.saving : t.providers.save}
						</Button>
					</div>
				</div>
			</Modal>
		</div>
	);
}
