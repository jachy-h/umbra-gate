import { useTranslation } from "../i18n/context";
import type { Locale } from "../i18n/types";

const labels: Record<Locale, string> = {
	en: "EN",
	zh: "中文",
};

export function LanguageSwitch() {
	const { locale, setLocale } = useTranslation();

	const toggle = () => {
		setLocale(locale === "en" ? "zh" : "en");
	};

	return (
		<button
			onClick={toggle}
			className="inline-flex items-center gap-1.5 rounded-full px-3 py-1.5 text-sm font-medium text-[var(--color-muted)] hover:text-[var(--color-ink)] hover:bg-[var(--color-surface-soft)] transition-colors cursor-pointer"
			title={locale === "en" ? "Switch to Chinese" : "切换到英文"}
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
				<circle cx="12" cy="12" r="10" />
				<line x1="2" y1="12" x2="22" y2="12" />
				<path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z" />
			</svg>
			{labels[locale]}
		</button>
	);
}
