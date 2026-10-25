import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { HttpResponse, http } from 'msw';
import { renderApp } from './render-app';
import { resetModelSettingsMock } from '../mocks/model-settings';
import { server } from '../mocks/server';

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
  it('признак bonus_locked подписан словами, без технического id', async () => {
    renderApp('/model');
    expect(await screen.findByText('Бонусы заблокированы')).toBeInTheDocument();
    expect(screen.queryByText('bonus_locked')).not.toBeInTheDocument();
  });

  it('сдвиг noauto до 0,85 показывает «сменят уровень 1 клиент, потеря +200,69 €»', async () => {
    const user = userEvent.setup();
    renderApp('/model');
    const thumb = await noautoThumb();
    thumb.focus();
    await user.keyboard('{ArrowRight>6/}');
    expect(thumb).toHaveAttribute('aria-valuenow', '0.85');
    expect(await screen.findByText(/сменят уровень 1 клиент, потеря \+200,69/)).toBeInTheDocument();
  });

  it('«Сохранить» создаёт новую версию, показанную в шапке', async () => {
    const user = userEvent.setup();
    renderApp('/model');
    const thumb = await noautoThumb();
    expect(screen.getByLabelText('Версия настроек')).toHaveTextContent('Версия 1');
    thumb.focus();
    await user.keyboard('{ArrowRight}');
    await user.click(screen.getByRole('button', { name: 'Сохранить' }));
    await waitFor(() =>
      expect(screen.getByLabelText('Версия настроек')).toHaveTextContent('Версия 2'),
    );
    expect(screen.getByRole('slider', { name: 'Автопродление выключено' })).toHaveAttribute(
      'aria-valuenow',
      '0.6',
    );
  });

  it('ошибка 422 показана у поля, версия не растёт', async () => {
    const user = userEvent.setup();
    renderApp('/model');
    await noautoThumb();
    const high = screen.getByLabelText('Порог высокого уровня');
    await user.clear(high);
    await user.type(high, '40');
    await user.click(screen.getByRole('button', { name: 'Сохранить' }));
    expect(
      await screen.findByText(/Порог высокого уровня должен быть выше среднего/),
    ).toBeInTheDocument();
    expect(screen.getByLabelText('Версия настроек')).toHaveTextContent('Версия 1');
  });

  it('«Сбросить к умолчанию» возвращает вес и пороги', async () => {
    const user = userEvent.setup();
    renderApp('/model');
    const thumb = await noautoThumb();
    thumb.focus();
    await user.keyboard('{ArrowRight>3/}');
    expect(thumb).toHaveAttribute('aria-valuenow', '0.7');
    await user.click(screen.getByRole('button', { name: 'Сбросить к умолчанию' }));
    expect(screen.getByRole('slider', { name: 'Автопродление выключено' })).toHaveAttribute(
      'aria-valuenow',
      '0.55',
    );
  });
});
