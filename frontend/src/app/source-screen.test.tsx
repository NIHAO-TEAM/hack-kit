import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { HttpResponse, http } from 'msw';
import { renderApp } from './render-app';
import { resetSourceMock } from '../mocks/source';
import { server } from '../mocks/server';

function signedInAs(role: 'analyst' | 'marketer') {
  server.use(http.get('/api/v1/auth/me', () => HttpResponse.json({ id: 2, login: role, role })));
}

beforeEach(resetSourceMock);

describe('экран «Источник»', () => {
  it('шапка показывает версию и источник', async () => {
    signedInAs('analyst');
    renderApp('/source');
    const header = await screen.findByRole('region', { name: 'Текущий снимок' });
    expect(within(header).getByText('1')).toBeInTheDocument();
    expect(within(header).getByText('Набор организаторов')).toBeInTheDocument();
  });

  it.each([
    ['emulator', 'Эмулятор'],
    ['future-origin', 'future-origin'],
  ])('источник «%s»: подпись «%s», неизвестное значение — как есть', async (source, label) => {
    signedInAs('analyst');
    server.use(
      http.get('/api/v1/source/status', () =>
        HttpResponse.json({
          version: 3,
          source,
          loadedAt: '2026-10-19T15:00:00Z',
          ageSeconds: 1,
          checks: [],
        }),
      ),
    );
    renderApp('/source');
    const header = await screen.findByRole('region', { name: 'Текущий снимок' });
    expect(within(header).getByText(label)).toBeInTheDocument();
    if (source === 'emulator') {
      expect(within(header).getByText(/а не REST API биллинга/)).toBeInTheDocument();
    }
  });

  it('проверки качества из сервера: текст, услуги и ссылки на клиентов', async () => {
    signedInAs('analyst');
    renderApp('/source');
    const quality = await screen.findByRole('region', { name: 'Проверки качества данных' });
    expect(
      within(quality).getByText('2 услуги в ожидании оплаты дольше 5 дней'),
    ).toBeInTheDocument();
    expect(within(quality).getByText(/Нет услуг в статусе «заблокирована»/)).toBeInTheDocument();
    expect(within(quality).getByText(/Услуга 11/)).toBeInTheDocument();
    expect(within(quality).getByRole('link', { name: '3' })).toHaveAttribute('href', '/queue/3');
  });

  it('пока пересчёт не готов — «считаются», после сброса ключа source (его сбрасывает recalculated) проверки появляются', async () => {
    signedInAs('analyst');
    let ready = false;
    server.use(
      http.get('/api/v1/source/status', () =>
        HttpResponse.json({
          version: 2,
          loadedAt: '2026-10-19T15:00:00Z',
          ageSeconds: 1,
          checks: ready
            ? [
                {
                  id: 'stale_status',
                  ok: true,
                  message: 'Нет услуг в ожидании оплаты дольше 5 дней',
                },
              ]
            : [],
        }),
      ),
    );
    const { queryClient } = renderApp('/source');
    expect(await screen.findByText(/Проверки считаются/)).toBeInTheDocument();
    ready = true;
    await queryClient.invalidateQueries({ queryKey: ['source'] });
    expect(await screen.findByText('Противоречий не найдено.')).toBeInTheDocument();
  });

  it('все проверки прошли — «Противоречий не найдено»', async () => {
    signedInAs('analyst');
    server.use(
      http.get('/api/v1/source/status', () =>
        HttpResponse.json({
          version: 1,
          loadedAt: '2026-10-19T15:00:00Z',
          ageSeconds: 1,
          checks: [
            { id: 'stale_status', ok: true, message: 'Нет услуг в ожидании оплаты дольше 5 дней' },
          ],
        }),
      ),
    );
    renderApp('/source');
    expect(await screen.findByText('Противоречий не найдено.')).toBeInTheDocument();
  });

  it('эмулятор подписан как имитация, «+24 часа» поднимает версию', async () => {
    signedInAs('analyst');
    const user = userEvent.setup();
    renderApp('/source');
    const emulator = await screen.findByRole('region', { name: 'Эмулятор биллинга' });
    expect(within(emulator).getByText(/Имитация биллинга/)).toBeInTheDocument();
    await user.click(within(emulator).getByRole('button', { name: '+24 часа' }));
    await waitFor(() =>
      expect(
        within(screen.getByRole('region', { name: 'Текущий снимок' })).getByText('2'),
      ).toBeInTheDocument(),
    );
  });

  it.each([
    ['529,84', 52984],
    ['529', 52900],
    ['0,05', 5],
  ])('пополнение на «%s» € уходит в запрос как %d центов', async (typed, cents) => {
    signedInAs('analyst');
    const bodies: unknown[] = [];
    server.use(
      http.post('/api/v1/source/emulator/actions', async ({ request }) => {
        bodies.push(await request.json());
        return HttpResponse.json({ version: 2, source: 'emulator' });
      }),
    );
    const user = userEvent.setup();
    renderApp('/source');
    const emulator = await screen.findByRole('region', { name: 'Эмулятор биллинга' });
    await user.type(within(emulator).getByLabelText('Номер клиента'), '9');
    await user.type(within(emulator).getByLabelText('Сумма, €'), typed);
    await user.click(within(emulator).getByRole('button', { name: 'Пополнить' }));
    await waitFor(() => expect(bodies).toHaveLength(1));
    expect(JSON.stringify(bodies[0])).toContain(`"cents":${cents}`);
  });

  it('плохой файл: ошибка по полю человеческим текстом', async () => {
    signedInAs('analyst');
    const user = userEvent.setup();
    renderApp('/source');
    const panel = await screen.findByRole('region', { name: 'Загрузка снимка' });
    const file = new File([JSON.stringify({ services: [{ ownerId: 999 }] })], 'bad.json', {
      type: 'application/json',
    });
    await user.upload(panel.querySelector('input[type="file"]')!, file);
    await user.click(within(panel).getByRole('button', { name: 'Загрузить' }));
    expect(await within(panel).findByText(/владелец 999 не найден/)).toBeInTheDocument();
    expect(within(panel).getByText('Снимок не принят, данные прежние')).toBeInTheDocument();
  });

  it('неизвестный клиент при правке счёта: ошибка у поля', async () => {
    signedInAs('analyst');
    const user = userEvent.setup();
    renderApp('/source');
    const panel = await screen.findByRole('region', { name: 'Правка данных' });
    await user.type(within(panel).getByLabelText('Номер клиента'), '99');
    await user.click(within(panel).getByRole('button', { name: 'Изменить счёт' }));
    expect(await within(panel).findByText('Такого клиента нет')).toBeInTheDocument();
  });
});
