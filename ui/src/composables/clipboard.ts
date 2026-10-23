import { toast } from 'vue-sonner'
import { i18nGlobal } from '@/config/lang'

function legacyCopy(value: string): void {
  const textarea = document.createElement('textarea')
  textarea.value = value
  textarea.setAttribute('readonly', '')
  textarea.style.position = 'fixed'
  textarea.style.opacity = '0'
  document.body.appendChild(textarea)
  textarea.select()
  // Fallback for insecure contexts (plain HTTP on a public IP), where the Clipboard API is absent.
  document.execCommand('copy')
  textarea.remove()
}

export async function copyText(value: string): Promise<void> {
  try {
    await navigator.clipboard.writeText(value)
  } catch {
    legacyCopy(value)
  }
  toast.success(i18nGlobal().t('copied'), { description: value, duration: 1800 })
}
