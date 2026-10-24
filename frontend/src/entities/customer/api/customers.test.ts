import { QueryClient } from '@tanstack/react-query';
import { HttpResponse, http } from 'msw';
import { server } from '../../../mocks/server';
import { customersOptions } from './customers';

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
