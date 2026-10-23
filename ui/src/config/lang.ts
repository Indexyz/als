import { nextTick } from 'vue'
import { createI18n } from 'vue-i18n'
import type enUS from '@/locales/en-US.json'

export type MessageSchema = typeof enUS
export type LocaleCode = 'zh-CN' | 'en-US'

export interface LanguageOption {
  label: string
  short: string
  value: LocaleCode
  autoChangeMap: string[]
}

export const list: LanguageOption[] = [
  { label: '简体中文', short: '中', value: 'zh-CN', autoChangeMap: ['zh-CN', 'zh'] },
  { label: 'English', short: 'EN', value: 'en-US', autoChangeMap: ['en-US', 'en'] }
]

const STORAGE_KEY = 'als-lang'
const DEFAULT_LOCALE: LocaleCode = 'zh-CN'

const i18n = createI18n<[MessageSchema], LocaleCode, false>({
  locale: DEFAULT_LOCALE,
  legacy: false
})

export const i18nGlobal = () => i18n.global

// Lazy loading, see https://vue-i18n.intlify.dev/guide/advanced/lazy.html
export function setupI18n() {
  void loadLocaleMessages(DEFAULT_LOCALE)
  setI18nLanguage(DEFAULT_LOCALE)
  return i18n
}

export function setI18nLanguage(locale: LocaleCode): void {
  i18n.global.locale.value = locale
  document.documentElement.setAttribute('lang', locale)
}

export async function loadLocaleMessages(locale: LocaleCode): Promise<void> {
  const messages = (await import(`../locales/${locale}.json`)) as { default: MessageSchema }
  i18n.global.setLocaleMessage(locale, messages.default)
  await nextTick()
}

export async function changeLanguage(locale: LocaleCode): Promise<LocaleCode> {
  await loadLocaleMessages(locale)
  setI18nLanguage(locale)
  try {
    localStorage.setItem(STORAGE_KEY, locale)
  } catch {}
  return locale
}

export async function autoLang(): Promise<LocaleCode> {
  let saved: string | null = null
  try {
    saved = localStorage.getItem(STORAGE_KEY)
  } catch {}
  const candidates = [saved, ...navigator.languages, navigator.language].filter(
    (value): value is string => Boolean(value)
  )
  for (const candidate of candidates) {
    const lang = list.find(
      (item) => item.value === candidate || item.autoChangeMap.includes(candidate)
    )
    if (lang) {
      await loadLocaleMessages(lang.value)
      setI18nLanguage(lang.value)
      return lang.value
    }
  }
  return DEFAULT_LOCALE
}
