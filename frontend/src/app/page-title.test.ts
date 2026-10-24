import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

describe('заголовок вкладки', () => {
  it('«Контур удержания · aéza»', () => {
    const html = readFileSync(resolve(process.cwd(), 'index.html'), 'utf8');
    expect(html).toContain('<title>Контур удержания · aéza</title>');
  });
});
