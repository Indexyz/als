import { onScopeDispose, ref } from 'vue'
import { defineStore } from 'pinia'
import axios from 'axios'
import { formatBytes } from '@/helper/unit'
export const useAppStore = defineStore('app', () => {
  const source = ref()
  const sessionId = ref()
  const connecting = ref(true)
  const config = ref()
  const drawerWidth = ref()
  const memoryUsage = ref()
  let timer = null
  let disposed = false

  const handleResize = () => {
    let width = window.innerWidth
    if (width > 800) {
      drawerWidth.value = 800
    } else {
      drawerWidth.value = width
    }
  }
  window.addEventListener('resize', handleResize)
  handleResize()

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
    eventSource.addEventListener('SessionId', (e) => {
      if (source.value !== eventSource) return
      sessionId.value = e.data
      console.log('session', e.data)
    })

    eventSource.addEventListener('Config', (e) => {
      if (source.value !== eventSource) return
      try {
        config.value = JSON.parse(e.data)
        connecting.value = false
      } catch (error) {
        console.error('Invalid SSE Config payload', error)
      }
    })
    eventSource.addEventListener('MemoryUsage', (e) => {
      if (source.value !== eventSource) return
      memoryUsage.value = formatBytes(e.data)
    })

    eventSource.onerror = function () {
      if (source.value !== eventSource || disposed) return
      eventSource.close()
      source.value = undefined
      connecting.value = true
      console.log('SSE disconnected')
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
    window.removeEventListener('resize', handleResize)
  }

  setupEventSource()
  onScopeDispose(cleanup)

  const requestMethod = (method, data = {}, signal = null) => {
    let axiosConfig = {
      timeout: 1000 * 120, // 请求超时时间
      headers: {
        session: sessionId.value
      }
    }

    if (signal != null) {
      axiosConfig.signal = signal
    }

    const _axios = axios.create(axiosConfig)

    return new Promise((resolve, reject) => {
      _axios
        .get('./method/' + method, { params: data })
        .then((response) => {
          if (response.data.success) {
            resolve(response.data)
            return
          }
          reject(response)
        })
        .catch((error) => {
          if (error.code == 'ERR_CANCELED') {
            reject(error)
            return
          }
          console.error(error)
          reject(error)
        })
    })
  }

  return {
    //vars
    source,
    sessionId,
    connecting,
    config,
    drawerWidth,
    memoryUsage,

    //methods
    requestMethod,
    cleanup
  }
})
