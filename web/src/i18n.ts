import { createI18n } from 'vue-i18n'
import en from './locales/en'
import ru from './locales/ru'

const STORAGE_KEY = 'ovpn-dash.locale'

function loadLocale(): 'ru' | 'en' {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved === 'ru' || saved === 'en') return saved
  } catch {
    /* ignore */
  }
  return 'ru'
}

export const i18n = createI18n({
  legacy: false,
  locale: loadLocale(),
  fallbackLocale: 'en',
  messages: { en, ru },
})

export function setLocale(locale: 'ru' | 'en') {
  i18n.global.locale.value = locale
  try {
    localStorage.setItem(STORAGE_KEY, locale)
  } catch {
    /* ignore */
  }
  if (typeof document !== 'undefined') {
    document.documentElement.lang = locale
  }
}

if (typeof document !== 'undefined') {
  document.documentElement.lang = String(i18n.global.locale.value)
}
