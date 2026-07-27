import {
	createContext,
	useContext,
	useState,
	useCallback,
	useEffect,
	type ReactNode,
} from "react";
import type { Locale, Translations } from "./types";
import en from "./locales/en";
import zh from "./locales/zh";

const locales: Record<Locale, Translations> = { en, zh };

const STORAGE_KEY = "umbra-gate-locale";

function detectLocale(): Locale {
	try {
		const stored = localStorage.getItem(STORAGE_KEY) as Locale | null;
		if (stored && (stored === "en" || stored === "zh")) return stored;
	} catch {
		// localStorage unavailable
	}
	const navLang = navigator.language.toLowerCase();
	if (navLang.startsWith("zh")) return "zh";
	return "en";
}

interface I18nContextValue {
	locale: Locale;
	t: Translations;
	setLocale: (locale: Locale) => void;
}

const I18nContext = createContext<I18nContextValue | null>(null);

export function I18nProvider({ children }: { children: ReactNode }) {
	const [locale, setLocaleState] = useState<Locale>(detectLocale);

	const setLocale = useCallback((next: Locale) => {
		setLocaleState(next);
		try {
			localStorage.setItem(STORAGE_KEY, next);
		} catch {
			// localStorage unavailable
		}
	}, []);

	// Sync when another tab changes locale
	useEffect(() => {
		const handler = (e: StorageEvent) => {
			if (
				e.key === STORAGE_KEY &&
				(e.newValue === "en" || e.newValue === "zh")
			) {
				setLocaleState(e.newValue as Locale);
			}
		};
		window.addEventListener("storage", handler);
		return () => window.removeEventListener("storage", handler);
	}, []);

	const value: I18nContextValue = {
		locale,
		t: locales[locale],
		setLocale,
	};

	return <I18nContext value={value}>{children}</I18nContext>;
}

export function useTranslation(): I18nContextValue {
	const ctx = useContext(I18nContext);
	if (!ctx) {
		throw new Error("useTranslation must be used within an I18nProvider");
	}
	return ctx;
}
