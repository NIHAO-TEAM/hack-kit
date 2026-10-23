import { MantineProvider } from '@mantine/core';
import { render, screen, waitFor } from '@testing-library/react';
import { afterEach, vi } from 'vitest';
import { LoginPage } from './LoginPage';

function renderPage() {
  render(
    <MantineProvider>
      <LoginPage onSubmit={async () => {}} />
    </MantineProvider>,
  );
}

describe('LoginPage', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('рядом с входом по паролю есть кнопка SSO', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ status: 0, type: 'opaqueredirect' }));
    renderPage();
    expect(screen.getByRole('button', { name: 'Войти' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Войти через SSO' })).toHaveAttribute(
      'href',
      '/api/v1/auth/oidc/login',
    );
  });

  it('SSO выключен — кнопки нет, вход по паролю остаётся', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ status: 501 }));
    renderPage();
    await waitFor(() =>
      expect(screen.queryByRole('link', { name: 'Войти через SSO' })).not.toBeInTheDocument(),
    );
    expect(screen.getByRole('button', { name: 'Войти' })).toBeInTheDocument();
  });

  it('сбой проверки SSO кнопку не прячет', async () => {
    const fetchMock = vi.fn().mockRejectedValue(new TypeError('network'));
    vi.stubGlobal('fetch', fetchMock);
    renderPage();
    await waitFor(() => expect(fetchMock).toHaveBeenCalled());
    expect(screen.getByRole('link', { name: 'Войти через SSO' })).toBeInTheDocument();
  });
});
