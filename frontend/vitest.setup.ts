import '@testing-library/jest-dom/vitest';
import { notifications } from '@mantine/notifications';
import { cleanup } from '@testing-library/react';
import { configure } from '@testing-library/dom';
import { server } from './src/mocks/server';
import { conflictOn, resetTasks } from './src/mocks/tasks';

// Mantine требует matchMedia и ResizeObserver, которых нет в jsdom.
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    addListener: () => undefined,
    removeListener: () => undefined,
    dispatchEvent: () => false,
  }),
});
globalThis.ResizeObserver = class {
  observe() {}
  unobserve() {}
  disconnect() {}
};
window.HTMLElement.prototype.scrollIntoView = () => undefined;

// Первый рендер маршрута грузит код экрана; на загруженном раннере 1 с по умолчанию мало.
configure({ asyncUtilTimeout: 5000 });

beforeAll(() => server.listen({ onUnhandledFrame: 'error' }));
afterEach(() => {
  cleanup();
  notifications.clean();
  server.resetHandlers();
  sessionStorage.clear();
  resetTasks();
  conflictOn.clear();
});
afterAll(() => server.close());
