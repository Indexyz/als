import { computed, ref, watchEffect } from 'vue'

export type ThemePreference = 'light' | 'dark' | 'system'

const STORAGE_KEY = 'als-theme'
const media = window.matchMedia('(prefers-color-scheme: dark)')
const systemDark = ref(media.matches)
media.addEventListener('change', (event) => (systemDark.value = event.matches))

let saved: string | null = null
try {
  saved = localStorage.getItem(STORAGE_KEY)
} catch {}

const preference = ref<ThemePreference>(saved === 'light' || saved === 'dark' ? saved : 'system')
const isDark = computed(() =>
  preference.value === 'system' ? systemDark.value : preference.value === 'dark'
)

watchEffect(() => {
  document.documentElement.classList.toggle('dark', isDark.value)
})

function setPreference(value: ThemePreference): void {
  preference.value = value
  try {
    if (value === 'system') localStorage.removeItem(STORAGE_KEY)
    else localStorage.setItem(STORAGE_KEY, value)
  } catch {}
}

export function useTheme() {
  return { preference, isDark, setPreference }
}
