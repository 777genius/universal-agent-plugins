// @ts-check
import withNuxt from './.nuxt/eslint.config.mjs'
import tsParser from '@typescript-eslint/parser'
import { protectedRules } from './scripts/quality-policy.mjs'

export default withNuxt(
  {
    files: ['**/*.ts'],
    languageOptions: {
      parser: tsParser,
    },
  },
  {
    files: ['**/*.{js,mjs,cjs,ts,vue}'],
    rules: protectedRules,
  },
  {
    files: ['**/*.vue'],
    languageOptions: {
      parserOptions: {
        parser: tsParser,
      },
    },
  },
)
