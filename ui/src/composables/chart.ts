import { computed } from 'vue'
import type { ApexOptions } from 'apexcharts'
import { useTheme } from './theme'

// ApexCharts manipulates colours itself (gradients, shades), so it needs concrete hex values
// rather than CSS variables. These mirror --chart-1 / --chart-2 in base.css.
const palette = {
  light: { primary: '#0f9f75', secondary: '#3b82f6', fore: '#6b7a74', grid: '#e5ebe8' },
  dark: { primary: '#34d399', secondary: '#60a5fa', fore: '#94a3a0', grid: '#28302f' }
} as const

export function useChartTheme() {
  const { isDark } = useTheme()
  const colors = computed(() => (isDark.value ? palette.dark : palette.light))

  const baseOptions = computed<ApexOptions>(() => ({
    chart: {
      background: 'transparent',
      foreColor: colors.value.fore,
      fontFamily: 'inherit',
      toolbar: { show: false },
      zoom: { enabled: false },
      animations: { enabled: true, dynamicAnimation: { speed: 350 } },
      parentHeightOffset: 0
    },
    theme: { mode: isDark.value ? 'dark' : 'light' },
    colors: [colors.value.primary, colors.value.secondary],
    dataLabels: { enabled: false },
    stroke: { curve: 'smooth', width: 2 },
    fill: {
      type: 'gradient',
      gradient: { shadeIntensity: 1, opacityFrom: 0.35, opacityTo: 0.02, stops: [0, 95] }
    },
    grid: {
      borderColor: colors.value.grid,
      strokeDashArray: 4,
      padding: { left: 8, right: 8, top: 0, bottom: 0 }
    },
    markers: { size: 0 },
    legend: { show: false },
    tooltip: { theme: isDark.value ? 'dark' : 'light' }
  }))

  return { colors, baseOptions }
}
