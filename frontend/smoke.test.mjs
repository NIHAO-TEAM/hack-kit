// Заглушка фронта: настоящий pnpm check описан в frontend/AGENTS.md.
import { test } from 'node:test';
import assert from 'node:assert/strict';

test('заглушка фронта собирается', () => {
  assert.equal(1 + 1, 2);
});
