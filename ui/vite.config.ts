import { fileURLToPath, URL } from 'node:url'
import { copyFileSync, existsSync, lstatSync } from 'node:fs'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

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
              test: /node_modules[\\/](apexcharts|vue3-apexcharts)[\\/]/
            },
            {
              name: 'markdown',
              test: /node_modules[\\/](highlight\.js|markdown-it[^\\/]*|linkify-it|mdurl|uc\.micro|entities|punycode\.js)[\\/]/
            }
          ]
        }
      }
    }
  },
  plugins: [
    vue(),
    tailwindcss(),
    {
      name: 'copy-speedtest-worker',
      buildStart() {
        const destination = new URL('./public/speedtest_worker.js', import.meta.url)
        if (!existsSync(destination) || !lstatSync(destination).isSymbolicLink()) {
          copyFileSync(new URL('./speedtest/speedtest_worker.js', import.meta.url), destination)
        }
      }
    }
  ]
})
