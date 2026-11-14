// @ts-check
import withNuxt from './.nuxt/eslint.config.mjs'
import tsParser from '@typescript-eslint/parser'

export default withNuxt(
  {
    files: ['**/*.ts'],
    languageOptions: {
      parser: tsParser,
    },
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
