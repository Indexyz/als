import js from '@eslint/js'
import { globalIgnores } from 'eslint/config'
import prettier from 'eslint-config-prettier/flat'
import pluginVue from 'eslint-plugin-vue'
import { vueTsConfigs, withVueTs } from '@vue/eslint-config-typescript'
import globals from 'globals'

export default withVueTs(
  { rootDir: import.meta.dirname },
  globalIgnores([
    'dist/**',
    'speedtest/**',
    'public/speedtest_worker.js',
    // Generated shadcn-vue primitives; keep them identical to upstream.
    'src/components/ui/**'
  ]),
  js.configs.recommended,
  pluginVue.configs['flat/essential'],
  vueTsConfigs.recommendedTypeChecked,
  {
    files: ['**/*.{ts,vue}'],
    languageOptions: {
      globals: globals.browser
    },
    rules: {
      'no-empty': ['error', { allowEmptyCatch: true }],
      'vue/multi-word-component-names': 'off'
    }
  },
  {
    files: ['*.config.ts', 'src/**/*.test.ts'],
    languageOptions: {
      globals: globals.node
    }
  },
  {
    // node:test's test() returns a promise that the runner itself tracks.
    files: ['src/**/*.test.ts'],
    rules: {
      '@typescript-eslint/no-floating-promises': [
        'error',
        { allowForKnownSafeCalls: [{ from: 'package', package: 'node:test', name: 'test' }] }
      ]
    }
  },
  prettier
)
