import { defineConfig, globalIgnores } from 'eslint/config';
import grafanaConfig from '@grafana/eslint-config/flat.js';

export default defineConfig([
  globalIgnores(['dist/**', 'node_modules/**', '.config/**', 'scripts/**']),
  ...grafanaConfig,
  {
    rules: {
      'react/prop-types': 'off',
      // The editor components use the standard async-fetch-in-effect pattern
      // (synchronous loading-state reset + cancellation-guarded promise
      // callbacks). The newer react-hooks rule flags the synchronous setState;
      // keep it as a warning rather than refactoring working data-loading code.
      'react-hooks/set-state-in-effect': 'warn',
    },
  },
  {
    // Type-aware linting for non-test source files. Test files are excluded
    // from tsconfig.json (so `tsc --noEmit` stays clean) and are linted
    // without type information below.
    files: ['src/**/*.{ts,tsx}'],
    ignores: ['src/**/*.test.{ts,tsx}'],
    languageOptions: {
      parserOptions: {
        project: './tsconfig.json',
        tsconfigRootDir: import.meta.dirname,
      },
    },
    rules: {
      '@typescript-eslint/no-deprecated': 'warn',
    },
  },
  {
    files: ['src/**/*.test.{ts,tsx}'],
    languageOptions: {
      parserOptions: {
        project: null,
        projectService: false,
      },
    },
    rules: {
      'react-hooks/rules-of-hooks': 'off',
    },
  },
]);
