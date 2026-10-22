import js from '@eslint/js'
import { defineConfig, globalIgnores } from 'eslint/config'
import prettier from 'eslint-config-prettier/flat'
import vue from 'eslint-plugin-vue'
import globals from 'globals'

export default defineConfig([
  globalIgnores(['dist/**', 'speedtest/**', 'public/speedtest_worker.js']),
  js.configs.recommended,
  ...vue.configs['flat/essential'],
  {
    files: ['**/*.{js,vue}'],
    languageOptions: {
      globals: {
        ...globals.browser,
        // Vue helpers provided by unplugin-auto-import in vite.config.js.
        computed: 'readonly',
        h: 'readonly',
        markRaw: 'readonly',
        onMounted: 'readonly',
        ref: 'readonly',
        toRaw: 'readonly'
      }
    },
    rules: {
      'no-empty': ['error', { allowEmptyCatch: true }],
      'vue/multi-word-component-names': 'off'
    }
  },
  {
    files: ['*.config.js'],
    languageOptions: {
      globals: globals.node
    }
  },
  prettier
])
