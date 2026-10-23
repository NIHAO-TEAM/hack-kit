import { QueryClient } from '@tanstack/react-query';
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
});
