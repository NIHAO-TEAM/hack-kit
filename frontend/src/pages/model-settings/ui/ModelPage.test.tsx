import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { HttpResponse, http } from 'msw';
import { renderApp } from '../../../app/render-app';
import { resetModelSettingsMock } from '../../../mocks/model-settings';
import { server } from '../../../mocks/server';

beforeEach(() => {
  resetModelSettingsMock();
  server.use(
    http.get('/api/v1/auth/me', () =>
      HttpResponse.json({ id: 2, login: 'analyst', role: 'analyst' }),
    ),
  );
});

async function noautoThumb() {
  const group = await screen.findByText('Автопродление выключено');
  expect(group).toBeInTheDocument();
  return screen.getByRole('slider', { name: 'Автопродление выключено' });
}

describe('экран «Модель»', () => {
  it('сдвиг noauto до 0,85 показывает «сменят уровень 1 клиент, потеря +200,69 €»', async () => {
    const user = userEvent.setup();
    renderApp('/model');
    const thumb = await noautoThumb();
    thumb.focus();
    await user.keyboard('{ArrowRight>6/}');
    expect(thumb).toHaveAttribute('aria-valuenow', '0.85');
    expect(await screen.findByText(/сменят уровень 1 клиент, потеря \+200,69/)).toBeInTheDocument();
  });
});
