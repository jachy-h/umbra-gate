import { useState, useRef, useEffect, useMemo, useCallback } from "react";
import { createPortal } from "react-dom";
import { useTranslation } from "../i18n";

interface Option {
	label: string;
	value: string;
	hasAPIKey?: boolean;
}

interface SearchableSelectProps {
	options: Option[];
	value: string;
	onChange: (value: string) => void;
	onOpen?: () => void;
	placeholder?: string;
	disabled?: boolean;
	allowCustomValue?: boolean;
	className?: string;
}

export function SearchableSelect({
	options,
	value,
	onChange,
	onOpen,
	placeholder,
	disabled,
	allowCustomValue = false,
	className = "",
}: SearchableSelectProps) {
	const { t } = useTranslation();
	const [open, setOpen] = useState(false);
	const [query, setQuery] = useState("");
	const [highlightIndex, setHighlightIndex] = useState(-1);
	const containerRef = useRef<HTMLDivElement>(null);
	const listRef = useRef<HTMLDivElement>(null);
	const [menuPosition, setMenuPosition] = useState<{
		left: number;
		top: number;
		width: number;
	} | null>(null);

	const selectedLabel = useMemo(
		() => options.find((option) => option.value === value)?.label || value,
		[options, value],
	);

	const filtered = useMemo(() => {
		if (!query) return options;
		const normalizedQuery = query.toLowerCase();
		return options.filter((option) =>
			option.label.toLowerCase().includes(normalizedQuery),
		);
	}, [options, query]);

	useEffect(() => {
		setHighlightIndex(filtered.length > 0 ? 0 : -1);
	}, [filtered]);

	useEffect(() => {
		function handleOutsideClick(event: MouseEvent) {
			const target = event.target as Node;
			if (
				!containerRef.current?.contains(target) &&
				!listRef.current?.contains(target)
			) {
				setOpen(false);
				setQuery("");
			}
		}
		document.addEventListener("mousedown", handleOutsideClick);
		return () => document.removeEventListener("mousedown", handleOutsideClick);
	}, []);

	useEffect(() => {
		if (highlightIndex >= 0 && listRef.current?.children[highlightIndex]) {
			listRef.current.children[highlightIndex].scrollIntoView({
				block: "nearest",
			});
		}
	}, [highlightIndex]);

	useEffect(() => {
		if (!open) {
			setMenuPosition(null);
			return;
		}
		const updateMenuPosition = () => {
			const rect = containerRef.current?.getBoundingClientRect();
			if (rect)
				setMenuPosition({
					left: rect.left,
					top: rect.bottom + 4,
					width: rect.width,
				});
		};
		updateMenuPosition();
		window.addEventListener("resize", updateMenuPosition);
		window.addEventListener("scroll", updateMenuPosition, true);
		return () => {
			window.removeEventListener("resize", updateMenuPosition);
			window.removeEventListener("scroll", updateMenuPosition, true);
		};
	}, [open]);

	const choose = useCallback(
		(nextValue: string) => {
			onChange(nextValue);
			setOpen(false);
			setQuery("");
		},
		[onChange],
	);

	const handleKeyDown = useCallback(
		(event: React.KeyboardEvent<HTMLInputElement>) => {
			if (event.key === "ArrowDown") {
				event.preventDefault();
				setOpen(true);
				setHighlightIndex((previous) =>
					previous >= filtered.length - 1 ? 0 : previous + 1,
				);
			} else if (event.key === "ArrowUp") {
				event.preventDefault();
				setOpen(true);
				setHighlightIndex((previous) =>
					previous <= 0 ? filtered.length - 1 : previous - 1,
				);
			} else if (event.key === "Enter" && open) {
				event.preventDefault();
				if (highlightIndex >= 0) {
					choose(filtered[highlightIndex].value);
				} else if (allowCustomValue && query.trim()) {
					choose(query.trim());
				}
			} else if (event.key === "Escape") {
				event.preventDefault();
				setOpen(false);
				setQuery("");
				event.currentTarget.blur();
			}
		},
		[allowCustomValue, choose, filtered, highlightIndex, open, query],
	);

	const inputClass = `h-10 w-full rounded-md border border-[var(--color-hairline)] bg-[var(--color-canvas)] pl-3.5 pr-9 text-sm text-[var(--color-ink)] outline-none transition-colors placeholder:text-[var(--color-muted-soft)] focus:border-[var(--color-ink)] ${
		disabled ? "cursor-not-allowed opacity-50" : "cursor-text"
	} ${className}`;

	return (
		<div ref={containerRef} className="relative">
			<input
				type="text"
				role="combobox"
				aria-expanded={open}
				aria-autocomplete="list"
				autoComplete="off"
				value={open ? query : selectedLabel}
				onFocus={() => {
					if (!disabled) {
						setQuery("");
						setOpen(true);
						onOpen?.();
					}
				}}
				onClick={() => !disabled && setOpen(true)}
				onChange={(event) => {
					setOpen(true);
					setQuery(event.target.value);
				}}
				onKeyDown={handleKeyDown}
				onBlur={() => {
					if (allowCustomValue && query.trim()) choose(query.trim());
				}}
				disabled={disabled}
				title={selectedLabel || undefined}
				placeholder={placeholder}
				className={inputClass}
			/>
			<svg
				width="14"
				height="14"
				viewBox="0 0 24 24"
				fill="none"
				stroke="var(--color-muted)"
				strokeWidth="2"
				strokeLinecap="round"
				strokeLinejoin="round"
				className={`pointer-events-none absolute right-3 top-3 transition-transform duration-150 ${open ? "rotate-180" : ""}`}
			>
				<path d="M6 9l6 6 6-6" />
			</svg>

			{open &&
				menuPosition &&
				createPortal(
					<div
						className="fixed z-[100] overflow-hidden rounded-lg border border-[var(--color-hairline)] bg-[var(--color-canvas)] shadow-[0_8px_24px_rgba(0,0,0,0.12)] animate-fade-in"
						style={{
							left: menuPosition.left,
							top: menuPosition.top,
							width: menuPosition.width,
						}}
					>
						<div
							ref={listRef}
							role="listbox"
							className="max-h-52 overflow-y-auto"
						>
							{filtered.length === 0 ? (
								<div className="px-3 py-4 text-center text-sm text-[var(--color-muted)]">
									{t.searchableSelect.noResults}
								</div>
							) : (
								filtered.map((option, index) => (
									<button
										key={option.value}
										type="button"
										role="option"
										aria-selected={option.value === value}
										title={option.label}
										onMouseDown={(event) => event.preventDefault()}
										onClick={() => choose(option.value)}
										className={`w-full px-3 py-2.5 text-left text-sm transition-colors ${
											index === highlightIndex
												? "bg-blue-50 font-medium text-[var(--color-ink)] shadow-[inset_3px_0_0_var(--color-brand-accent)]"
												: option.value === value
													? "bg-[var(--color-surface-soft)] font-medium text-[var(--color-ink)]"
													: "text-[var(--color-ink)] hover:bg-[var(--color-surface-soft)]"
										}`}
									>
										<span>{option.label}</span>
										{option.hasAPIKey && (
											<svg
												aria-hidden="true"
												width="13"
												height="13"
												viewBox="0 0 24 24"
												fill="none"
												stroke="currentColor"
												strokeWidth="2.5"
												strokeLinecap="round"
												strokeLinejoin="round"
												className="ml-1 inline-block text-[var(--color-success)]"
											>
												<path d="m5 12 4 4L19 6" />
											</svg>
										)}
									</button>
								))
							)}
						</div>
					</div>,
					document.body,
				)}
		</div>
	);
}
