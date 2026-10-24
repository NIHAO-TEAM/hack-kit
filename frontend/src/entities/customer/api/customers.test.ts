import { QueryClient } from '@tanstack/react-query';
import { HttpResponse, http } from 'msw';
import { server } from '../../../mocks/server';
import { customersExportHref, customersOptions } from './customers';

describe('customersOptions', () => {
  it('ключ содержит параметры: смена сортировки — другой запрос', () => {
    const a = customersOptions({ sort: 'expectedLoss', order: 'desc' });
    const b = customersOptions({ sort: 'expectedLoss', order: 'asc' });
    expect(a.queryKey).not.toEqual(b.queryKey);
    expect(a.queryKey[0]).toBe('customers');
  });

  it('грузит страницу с мок-сервера', async () => {
    const client = new QueryClient();
    const page = await client.fetchQuery(customersOptions({ sort: 'expectedLoss', order: 'desc' }));
    expect(page.total).toBe(page.items.length);
  });

  it('страница 2 при размере 50 → offset=50, limit=50', async () => {
    let query = new URLSearchParams();
    server.use(
      http.get('/api/v1/customers', ({ request }) => {
        query = new URL(request.url).searchParams;
        return HttpResponse.json({ items: [], total: 0 });
      }),
    );
    await new QueryClient().fetchQuery(
      customersOptions({ sort: 'expectedLoss', order: 'desc', page: 2, limit: 50 }),
    );
    expect(query.get('offset')).toBe('50');
    expect(query.get('limit')).toBe('50');
  });

  it('без страницы: limit=200 и без offset', async () => {
    let query = new URLSearchParams();
    server.use(
      http.get('/api/v1/customers', ({ request }) => {
        query = new URL(request.url).searchParams;
        return HttpResponse.json({ items: [], total: 0 });
      }),
    );
    await new QueryClient().fetchQuery(customersOptions({ sort: 'expectedLoss', order: 'desc' }));
    expect(query.get('limit')).toBe('200');
    expect(query.has('offset')).toBe(false);
  });
});

describe('customersExportHref', () => {
  it('несёт сортировку, уровень и поиск, без страниц', () => {
    const href = customersExportHref({
      sort: 'balance',
      order: 'asc',
      level: 'high',
      q: 'a@b.test',
      page: 3,
      limit: 50,
    });
    const url = new URL(href, 'http://x');
    expect(url.pathname).toBe('/api/v1/customers/export.csv');
    expect(Object.fromEntries(url.searchParams)).toEqual({
      sort: 'balance',
      order: 'asc',
      level: 'high',
      q: 'a@b.test',
    });
  });

  it('без фильтров — только сортировка', () => {
    const url = new URL(customersExportHref({ sort: 'expectedLoss', order: 'desc' }), 'http://x');
    expect([...url.searchParams.keys()]).toEqual(['sort', 'order']);
  });
});

describe('причина в очереди', () => {
  it('reason уходит на сервер, а sort по потере по причине — нет', async () => {
    let query = new URLSearchParams();
    server.use(
      http.get('/api/v1/customers', ({ request }) => {
        query = new URL(request.url).searchParams;
        return HttpResponse.json({ items: [], total: 0 });
      }),
    );
    await new QueryClient().fetchQuery(
      customersOptions({ sort: 'reasonLoss', order: 'desc', reason: 'susp' }),
    );
    expect(query.get('reason')).toBe('susp');
    expect(query.has('sort')).toBe(false);
  });

  it('явная сортировка остаётся вместе с причиной', async () => {
    let query = new URLSearchParams();
    server.use(
      http.get('/api/v1/customers', ({ request }) => {
        query = new URL(request.url).searchParams;
        return HttpResponse.json({ items: [], total: 0 });
      }),
    );
    await new QueryClient().fetchQuery(
      customersOptions({ sort: 'balance', order: 'asc', reason: 'susp' }),
    );
    expect(query.get('sort')).toBe('balance');
    expect(query.get('reason')).toBe('susp');
  });

  it('CSV несёт причину и не несёт sort по причине', () => {
    const url = new URL(
      customersExportHref({ sort: 'reasonLoss', order: 'desc', reason: 'susp' }),
      'http://x',
    );
    expect(url.searchParams.get('reason')).toBe('susp');
    expect(url.searchParams.has('sort')).toBe(false);
  });
});
