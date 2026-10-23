<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, ref } from 'vue'
import { Languages, MemoryStick, Monitor, Moon, Radar, Sun } from '@lucide/vue'
import 'vue-sonner/style.css'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu'
import { Toaster } from '@/components/ui/sonner'
import { TooltipProvider } from '@/components/ui/tooltip'
import { autoLang, changeLanguage, list as langList, type LocaleCode } from '@/config/lang'
import { useTheme, type ThemePreference } from '@/composables/theme'
import { useAppStore } from '@/stores/app'
import GithubIcon from '@/components/GithubIcon.vue'
import LoadingCard from '@/components/Loading.vue'
import InfoCard from '@/components/Information.vue'
import UtilitiesCard from '@/components/Utilities.vue'
import Diagnostics from '@/components/Utilities/Diagnostics.vue'

const SpeedtestCard = defineAsyncComponent(() => import('@/components/Speedtest.vue'))
const TrafficCard = defineAsyncComponent(() => import('@/components/TrafficDisplay.vue'))

const appStore = useAppStore()
const { preference, isDark, setPreference } = useTheme()

const currentLangCode = ref<LocaleCode>('zh-CN')
const currentLang = computed(() => langList.find((item) => item.value === currentLangCode.value))

const themeModel = computed<string>({
  get: () => preference.value,
  set: (value) => setPreference(value as ThemePreference)
})
const langModel = computed<string>({
  get: () => currentLangCode.value,
  set: (value) => {
    currentLangCode.value = value as LocaleCode
    void changeLanguage(value as LocaleCode)
  }
})

const themeOptions = [
  { value: 'light', icon: Sun },
  { value: 'dark', icon: Moon },
  { value: 'system', icon: Monitor }
] as const

const hasDiagnostics = computed(() =>
  (['feature_ping', 'feature_traceroute', 'feature_mtr', 'feature_nexttrace'] as const).some(
    (feature) => appStore.isEnabled(feature)
  )
)

onMounted(async () => {
  currentLangCode.value = await autoLang()
})
</script>

<template>
  <TooltipProvider :delay-duration="200">
    <div class="flex min-h-screen flex-col">
      <header
        class="bg-background/80 supports-[backdrop-filter]:bg-background/60 sticky top-0 z-40 border-b backdrop-blur"
      >
        <div class="mx-auto flex h-14 max-w-7xl items-center gap-3 px-4 sm:px-6">
          <div class="flex items-center gap-2.5">
            <div
              class="bg-primary/10 text-primary ring-primary/20 flex size-8 items-center justify-center rounded-lg ring-1"
            >
              <Radar class="size-4.5" />
            </div>
            <span class="text-[15px] font-semibold tracking-tight">{{ $t('app_title') }}</span>
          </div>

          <div
            class="text-muted-foreground ml-2 hidden items-center gap-1.5 text-xs sm:flex"
            aria-live="polite"
          >
            <span class="relative flex size-2">
              <span
                v-if="!appStore.connecting"
                class="bg-success absolute inline-flex size-full animate-ping rounded-full opacity-60"
              />
              <span
                class="relative inline-flex size-2 rounded-full"
                :class="appStore.connecting ? 'bg-warning' : 'bg-success'"
              />
            </span>
            {{ appStore.connecting ? $t('offline') : $t('online') }}
          </div>

          <div class="ml-auto flex items-center gap-1">
            <DropdownMenu>
              <DropdownMenuTrigger as-child>
                <Button variant="ghost" size="sm" class="gap-1.5" :aria-label="$t('language')">
                  <Languages />
                  <span class="text-xs">{{ currentLang?.short }}</span>
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" class="w-40">
                <DropdownMenuLabel>{{ $t('language') }}</DropdownMenuLabel>
                <DropdownMenuRadioGroup v-model="langModel">
                  <DropdownMenuRadioItem
                    v-for="lang in langList"
                    :key="lang.value"
                    :value="lang.value"
                  >
                    {{ lang.label }}
                  </DropdownMenuRadioItem>
                </DropdownMenuRadioGroup>
              </DropdownMenuContent>
            </DropdownMenu>

            <DropdownMenu>
              <DropdownMenuTrigger as-child>
                <Button variant="ghost" size="icon-sm" :aria-label="$t('theme.toggle')">
                  <Moon v-if="isDark" />
                  <Sun v-else />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" class="w-36">
                <DropdownMenuRadioGroup v-model="themeModel">
                  <DropdownMenuRadioItem
                    v-for="option in themeOptions"
                    :key="option.value"
                    :value="option.value"
                  >
                    <component :is="option.icon" class="text-muted-foreground" />
                    {{ $t(`theme.${option.value}`) }}
                  </DropdownMenuRadioItem>
                </DropdownMenuRadioGroup>
              </DropdownMenuContent>
            </DropdownMenu>

            <Button
              variant="ghost"
              size="icon-sm"
              as="a"
              href="https://github.com/Indexyz/als"
              target="_blank"
              rel="noopener"
              aria-label="GitHub"
            >
              <GithubIcon />
            </Button>
          </div>
        </div>
      </header>

      <main class="mx-auto w-full max-w-7xl flex-1 px-4 py-6 sm:px-6 sm:py-8">
        <LoadingCard v-if="appStore.connecting || !appStore.config" />
        <div v-else class="grid items-start gap-6 lg:grid-cols-[340px_minmax(0,1fr)]">
          <aside class="order-2 min-w-0 space-y-6 lg:sticky lg:top-20 lg:order-none">
            <InfoCard />
            <UtilitiesCard />
          </aside>
          <div class="order-1 min-w-0 space-y-6 lg:order-none">
            <Diagnostics v-if="hasDiagnostics" />
            <SpeedtestCard />
            <TrafficCard v-if="appStore.config.feature_iface_traffic" />
          </div>
        </div>
      </main>

      <footer class="border-t">
        <div
          class="text-muted-foreground mx-auto flex max-w-7xl flex-col items-center justify-between gap-2 px-4 py-5 text-xs sm:flex-row sm:px-6"
        >
          <div>
            {{ $t('powered_by') }}
            <a
              class="text-foreground/80 hover:text-foreground underline decoration-dotted underline-offset-4"
              href="https://github.com/Indexyz/als"
              target="_blank"
              rel="noopener"
              >ALS</a
            >
          </div>
          <div v-if="appStore.memoryUsage" class="flex items-center gap-1.5 font-mono">
            <MemoryStick class="size-3.5" />
            {{ $t('memory_usage') }}: {{ appStore.memoryUsage }}
          </div>
        </div>
      </footer>
    </div>
    <Toaster position="bottom-right" :theme="isDark ? 'dark' : 'light'" />
  </TooltipProvider>
</template>
