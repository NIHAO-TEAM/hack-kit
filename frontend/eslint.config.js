import js from '@eslint/js';
import query from '@tanstack/eslint-plugin-query';
import router from '@tanstack/eslint-plugin-router';
import reactHooks from 'eslint-plugin-react-hooks';
import globals from 'globals';
import tseslint from 'typescript-eslint';

export default tseslint.config(
  {
    ignores: [
      'dist',
      'public/mockServiceWorker.js',
      'src/app/routeTree.gen.ts',
      'src/shared/api/schema.gen.ts',
    ],
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  ...query.configs['flat/recommended'],
  ...router.configs['flat/recommended'],
  reactHooks.configs.flat.recommended,
  { languageOptions: { globals: { ...globals.browser, ...globals.node } } },
);
