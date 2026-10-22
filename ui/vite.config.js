import { fileURLToPath, URL } from 'node:url'
import { copyFileSync, existsSync, lstatSync } from 'node:fs'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import { NaiveUiResolver } from 'unplugin-vue-components/resolvers'

export default defineConfig({
  base: './',
  server: {
    proxy: {
      '/session': {
        target: 'http://127.0.0.1:8080',
        ws: true
      },
      '/method': {
        target: 'http://127.0.0.1:8080',
        ws: true
      }
    }
  },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  build: {
    rolldownOptions: {
      output: {
        codeSplitting: {
          groups: [
            {
              name: 'apexcharts',
              test: /node_modules[\\/]?(apexcharts|vue3-apexcharts)[\\/]/
            }
          ]
        }
      }
    }
  },
  plugins: [
    vue(),
    AutoImport({
      dts: false,
      imports: [
        'vue',
        {
          'naive-ui': ['useDialog', 'useMessage', 'useNotification', 'useLoadingBar']
        }
      ]
    }),
    {
      name: 'copy-speedtest-worker',
      buildStart() {
        const destination = new URL('./public/speedtest_worker.js', import.meta.url)
        if (!existsSync(destination) || !lstatSync(destination).isSymbolicLink()) {
          copyFileSync(new URL('./speedtest/speedtest_worker.js', import.meta.url), destination)
        }
      }
    },
    Components({
      dts: false,
      resolvers: [NaiveUiResolver()]
    })
  ]
})
