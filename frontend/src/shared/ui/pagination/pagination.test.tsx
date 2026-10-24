import { MantineProvider } from '@mantine/core';
import { render, screen } from '@testing-library/react';
import { Pagination } from './index';

describe('Pagination', () => {
  it('у кнопок есть имена, у текущей страницы aria-current', () => {
    render(
      <MantineProvider>
        <Pagination total={3} value={2} onChange={() => {}} />
      </MantineProvider>,
    );
    expect(screen.getByRole('button', { name: 'Предыдущая страница' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Следующая страница' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Страница 1' })).not.toHaveAttribute('aria-current');
    expect(screen.getByRole('button', { name: 'Страница 2' })).toHaveAttribute(
      'aria-current',
      'page',
    );
  });
});
