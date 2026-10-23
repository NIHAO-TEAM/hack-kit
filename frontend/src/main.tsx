import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { App } from '@app/index';

const root = document.getElementById('root');
if (!root) throw new Error('Нет корневого элемента #root');

async function start() {
  if (import.meta.env.MODE === 'mock') {
    const { worker } = await import('./mocks/browser');
    await worker.start({ quiet: true, onUnhandledFrame: 'bypass' });
  }
  createRoot(root!).render(
    <StrictMode>
      <App />
    </StrictMode>,
  );
}

void start();
