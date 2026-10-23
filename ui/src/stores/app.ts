import { onScopeDispose, ref, shallowRef } from 'vue'
import { defineStore } from 'pinia'
import axios, { type AxiosRequestConfig } from 'axios'
import { formatBytes } from '@/helper/unit'

export interface AppConfig {
  location: string
  public_ipv4: string
  public_ipv6: string
  my_ip: string
  speedtest_files: string[] | null
  sponsor_message: string
  filetest_follow_domain?: string
  feature_ping: boolean
  feature_nexttrace: boolean
  feature_librespeed: boolean
  feature_filespeedtest: boolean
  feature_speedtest_dot_net: boolean
  feature_iperf3: boolean
  feature_mtr: boolean
  feature_traceroute: boolean
  feature_iface_traffic: boolean
}

export type FeatureKey = Extract<keyof AppConfig, `feature_${string}`>

export interface MethodResponse {
  success: boolean
  [key: string]: unknown
}

export const useAppStore = defineStore('app', () => {
  const source = shallowRef<EventSource>()
  const sessionId = ref<string>()
  const connecting = ref(true)
  const config = ref<AppConfig>()
  const memoryUsage = ref<string>()
  let timer: ReturnType<typeof setTimeout> | null = null
  let disposed = false

  const clearReconnectTimer = () => {
    if (timer !== null) {
      clearTimeout(timer)
      timer = null
    }
  }

  const reconnectEventSource = () => {
    if (disposed || timer !== null) return
    timer = setTimeout(() => {
      timer = null
      if (!disposed) setupEventSource()
    }, 1000)
  }

  const setupEventSource = () => {
    if (disposed) return
    source.value?.close()
    connecting.value = true
    const eventSource = new EventSource('./session')
    eventSource.addEventListener('SessionId', (e: MessageEvent<string>) => {
      if (source.value !== eventSource) return
      sessionId.value = e.data
    })

    eventSource.addEventListener('Config', (e: MessageEvent<string>) => {
      if (source.value !== eventSource) return
      try {
        config.value = JSON.parse(e.data) as AppConfig
        connecting.value = false
      } catch (error) {
        console.error('Invalid SSE Config payload', error)
      }
    })
    eventSource.addEventListener('MemoryUsage', (e: MessageEvent<string>) => {
      if (source.value !== eventSource) return
      memoryUsage.value = formatBytes(e.data)
    })

    eventSource.onerror = () => {
      if (source.value !== eventSource || disposed) return
      eventSource.close()
      source.value = undefined
      connecting.value = true
      console.warn('SSE disconnected')
      reconnectEventSource()
    }
    source.value = eventSource
  }

  const cleanup = () => {
    if (disposed) return
    disposed = true
    clearReconnectTimer()
    source.value?.close()
    source.value = undefined
  }

  setupEventSource()
  onScopeDispose(cleanup)

  const requestMethod = async (
    method: string,
    data: Record<string, unknown> = {},
    signal: AbortSignal | null = null
  ): Promise<MethodResponse> => {
    const axiosConfig: AxiosRequestConfig = {
      timeout: 1000 * 120,
      headers: { session: sessionId.value },
      params: data
    }
    if (signal) axiosConfig.signal = signal
    const response = await axios.get<MethodResponse>('./method/' + method, axiosConfig)
    if (!response.data.success) throw new Error(`Method ${method} failed`)
    return response.data
  }

  const isEnabled = (feature: FeatureKey): boolean => Boolean(config.value?.[feature])

  return {
    source,
    sessionId,
    connecting,
    config,
    memoryUsage,
    requestMethod,
    isEnabled,
    cleanup
  }
})
