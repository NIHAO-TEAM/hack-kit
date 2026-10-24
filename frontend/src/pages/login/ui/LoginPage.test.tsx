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

  const config = (ssoEnabled: boolean) =>
    vi.fn().mockResolvedValue({ ok: true, status: 200, json: async () => ({ ssoEnabled }) });

  it('SSO включён — рядом с входом по паролю есть кнопка SSO', async () => {
    const fetchMock = config(true);
    vi.stubGlobal('fetch', fetchMock);
    renderPage();
    expect(screen.getByRole('button', { name: 'Войти' })).toBeInTheDocument();
    expect(await screen.findByRole('link', { name: 'Войти через SSO' })).toHaveAttribute(
      'href',
      '/api/v1/auth/oidc/login',
    );
    expect(fetchMock.mock.calls[0]?.[0]).toBe('/api/v1/auth/config');
  });

  it('SSO выключен — кнопки нет, вход по паролю остаётся', async () => {
    const fetchMock = config(false);
    vi.stubGlobal('fetch', fetchMock);
    renderPage();
    await waitFor(() => expect(fetchMock).toHaveBeenCalled());
    await new Promise((r) => setTimeout(r, 0));
    expect(screen.queryByRole('link', { name: 'Войти через SSO' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Войти' })).toBeInTheDocument();
  });

  it('сбой запроса настроек — кнопки нет, вход по паролю остаётся', async () => {
    const fetchMock = vi.fn().mockRejectedValue(new TypeError('network'));
    vi.stubGlobal('fetch', fetchMock);
    renderPage();
    await waitFor(() => expect(fetchMock).toHaveBeenCalled());
    await new Promise((r) => setTimeout(r, 0));
    expect(screen.queryByRole('link', { name: 'Войти через SSO' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Войти' })).toBeInTheDocument();
  });

  it('фон скрыт от читалок и с орбитой, названия и заголовок на месте, значка нет', () => {
    vi.stubGlobal('fetch', config(false));
    const { container } = render(
      <MantineProvider>
        <LoginPage onSubmit={async () => {}} />
      </MantineProvider>,
    );
    const backdrop = container.querySelector('[aria-hidden="true"]');
    expect(backdrop).not.toBeNull();
    expect(backdrop?.querySelectorAll('div').length).toBeGreaterThanOrEqual(3);
    expect(backdrop?.querySelector('svg ellipse')).not.toBeNull();
    expect(container.querySelector('main img')).toBeNull();
    expect(screen.getByRole('heading', { level: 1, name: 'Личный кабинет' })).toBeInTheDocument();
    expect(screen.getByText('Контур удержания')).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Контур удержания' })).not.toBeInTheDocument();
    expect(screen.getByLabelText('Логин')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Войти' })).toBeInTheDocument();
  });
});
