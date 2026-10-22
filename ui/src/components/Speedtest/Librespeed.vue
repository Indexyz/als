<script setup>
import { onUnmounted, ref, toRaw } from 'vue'
import { useAppStore } from '@/stores/app'
import VueApexCharts from 'vue3-apexcharts'
const appStore = useAppStore()

let workerInstance = null
let workerTimer = null
const MAX_CHART_POINTS = 100
const working = ref(false)
const uploadText = ref('...')
const downloadText = ref('...')
const chartDownloadRef = ref()
const chartUploadRef = ref()
const baseChartOptions = {
  chart: {
    height: 200,
    foreColor: '#e8e8e8',
    animations: {
      enabled: true,
      easing: 'linear',
      dynamicAnimation: {
        speed: 300
      }
    },
    zoom: {
      enabled: false
    },
    toolbar: {
      show: false
    }
  },
  tooltip: {
    theme: 'dark'
  },
  xaxis: {
    type: 'category',
    categories: [''],
    labels: {
      show: false
    }
  },
  yaxis: {
    labels: {
      formatter: (value) => {
        return value + ' Mbps'
      }
    }
  },
  dataLabels: {
    enabled: false
  },
  markers: {
    size: 0
  },
  stroke: {
    curve: 'smooth'
  }
}
const baseSeries = {
  type: 'area',
  name: 'Receive',
  data: []
}
const charts = ref({
  download: {
    ref: null,
    options: { ...baseChartOptions },
    series: [{ ...baseSeries }],
    data: [],
    categories: []
  },
  upload: {
    ref: null,
    options: { ...baseChartOptions },
    series: [{ ...baseSeries }],
    data: [],
    categories: []
  }
})

for (var chartId in charts.value) {
  charts.value[chartId].options.chart.id =
    'chart-librespeed-' + (Math.random() + 1).toString(36).substring(2)
}

const resetCharts = () => {
  for (const chart of Object.values(charts.value)) {
    chart.data = []
    chart.categories = []
  }
}

const stopSpeedtest = (force = false) => {
  if (workerTimer !== null) {
    clearInterval(workerTimer)
    workerTimer = null
  }
  if (workerInstance !== null) {
    workerInstance.onmessage = null
    workerInstance.postMessage('abort')
    workerInstance.terminate()
    workerInstance = null
  }
  working.value = false
  if (force) {
    uploadText.value = '...'
    downloadText.value = '...'
  }
}

const appendChartPoint = (chart, value, pointName) => {
  chart.data.push(value)
  chart.categories.push(pointName)
  if (chart.data.length > MAX_CHART_POINTS) {
    chart.data.splice(0, chart.data.length - MAX_CHART_POINTS)
    chart.categories.splice(0, chart.categories.length - MAX_CHART_POINTS)
  }
}

const startOrStopSpeedtest = () => {
  if (workerInstance !== null) {
    stopSpeedtest()
    return
  }
  resetCharts()
  working.value = true
  const worker = new Worker('./speedtest_worker.js')
  workerInstance = worker
  worker.onmessage = (e) => {
    if (workerInstance !== worker) return
    var nowPointName = new Date().toLocaleTimeString('en-GB', { hour12: false })
    let data
    try {
      data = JSON.parse(e.data)
    } catch (error) {
      console.error('Invalid speedtest worker payload', error)
      return
    }
    console.log(data)
    var status = data.testState
    if (status >= 4) {
      stopSpeedtest()
      return
    }

    if (status == 1 && data.dlStatus == 0) {
      downloadText.value = '...'
      uploadText.value = '...'
    }

    if (data.ulStatus) {
      uploadText.value = data.ulStatus
      appendChartPoint(charts.value.upload, data.ulStatus, nowPointName)
      chartUploadRef.value?.updateOptions({
        xaxis: { categories: toRaw(charts.value.upload.categories) }
      })
      chartUploadRef.value?.updateSeries([
        { name: 'Receive', data: toRaw(charts.value.upload.data) }
      ])
      return
    }

    if (data.dlStatus) {
      downloadText.value = data.dlStatus
      appendChartPoint(charts.value.download, data.dlStatus, nowPointName)
      chartDownloadRef.value?.updateOptions({
        xaxis: { categories: toRaw(charts.value.download.categories) }
      })
      chartDownloadRef.value?.updateSeries([
        { name: 'Receive', data: toRaw(charts.value.download.data) }
      ])
    }
  }
  worker.postMessage(
    'start ' +
      JSON.stringify({
        test_order: 'D_U',
        url_dl: './session/' + appStore.sessionId + '/speedtest/download',
        url_ul: './session/' + appStore.sessionId + '/speedtest/upload',
        url_ping: './session/' + appStore.sessionId + '/speedtest/upload'
      })
  )
  workerTimer = setInterval(() => {
    if (workerInstance === worker) worker.postMessage('status')
  }, 200)
}

onUnmounted(() => stopSpeedtest())
</script>

<template>
  <div v-show="working || downloadText !== '...'">
    <n-grid x-gap="12" cols="1 s:1 m:1 l:2" responsive="screen">
      <n-gi span="1">
        <div>
          <h4>{{ $t('librespeed_download') }}</h4>
          <h1>{{ downloadText }} Mbps</h1>
        </div>
        <VueApexCharts
          type="area"
          ref="chartDownloadRef"
          :options="charts.download.options"
          height="200px"
          :series="charts.download.series"
        />
      </n-gi>
      <n-gi span="1">
        <div>
          <h4>{{ $t('librespeed_upload') }}</h4>
          <h1>{{ uploadText }} Mbps</h1>
        </div>
        <VueApexCharts
          type="area"
          ref="chartUploadRef"
          :options="charts.upload.options"
          height="200px"
          :series="charts.upload.series"
        />
      </n-gi>
    </n-grid>
  </div>
  <n-space justify="space-evenly">
    <n-button size="large" @click="startOrStopSpeedtest" style="margin-top: 10px">
      <template v-if="working">
        <n-spin size="small" style="margin-right: 10px" />{{ $t('librespeed_stop') }}
      </template>
      <template v-else>{{ $t('librespeed_begin') }}</template>
    </n-button>
  </n-space>
</template>
