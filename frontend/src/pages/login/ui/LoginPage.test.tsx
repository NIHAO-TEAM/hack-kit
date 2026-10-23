import { MantineProvider } from '@mantine/core';
import { render, screen } from '@testing-library/react';
import { LoginPage } from './LoginPage';

describe('LoginPage', () => {
  it('рядом с входом по паролю есть кнопка SSO', () => {
    render(
      <MantineProvider>
        <LoginPage onSubmit={async () => {}} />
      </MantineProvider>,
    );
    expect(screen.getByRole('button', { name: 'Войти' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Войти через SSO' })).toHaveAttribute(
      'href',
      '/api/v1/auth/oidc/login',
    );
  });
});
