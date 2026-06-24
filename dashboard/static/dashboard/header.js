import { getLang, setLang, t } from './i18n.js';

const lang = getLang();
document.documentElement.lang = lang;

const languageToggle = document.getElementById('languageToggle');
if (languageToggle) {
    languageToggle.textContent = t('switchLang');
    languageToggle.addEventListener('click', () => {
        setLang(lang === 'en' ? 'zh' : 'en');
        window.location.reload();
    });
}
