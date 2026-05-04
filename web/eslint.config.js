// Flat ESLint config (ESM) for the Vite + React + TypeScript SPA.
//
// Scope: this is the authoritative *style and correctness* gate. The
// authoritative *type* gate is `npm run build` (which runs `tsc -b`).
// We deliberately don't enable type-aware rules here — typescript-eslint's
// recommended (non-type-aware) ruleset plus the React hooks plugin cover
// the foot-guns `tsc` can't see (exhaustive-deps, unused imports, etc.).
import js from '@eslint/js'
import globals from 'globals'
import tseslint from 'typescript-eslint'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'

export default tseslint.config(
  {
    // Generated files and build artefacts: never lint.
    ignores: ['dist', 'src/lib/schema.d.ts', 'playwright-report', 'test-results'],
  },
  {
    files: ['**/*.{ts,tsx}'],
    extends: [js.configs.recommended, ...tseslint.configs.recommended],
    languageOptions: {
      ecmaVersion: 2022,
      globals: { ...globals.browser, ...globals.node },
    },
    plugins: {
      'react-hooks': reactHooks,
      'react-refresh': reactRefresh,
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      'react-refresh/only-export-components': [
        'warn',
        { allowConstantExport: true },
      ],
    },
  },
)
