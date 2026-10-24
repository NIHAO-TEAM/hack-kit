import { Pagination as MantinePagination, type PaginationProps } from '@mantine/core';

const controlLabels = {
  first: 'Первая страница',
  previous: 'Предыдущая страница',
  next: 'Следующая страница',
  last: 'Последняя страница',
} as const;

/**
 * Пагинация с доступными именами: кнопки без текста («‹», «›») иначе нарушают axe `button-name`.
 * Текущая страница отмечена `aria-current="page"`.
 */
export function Pagination(props: Omit<PaginationProps, 'getControlProps' | 'getItemProps'>) {
  const current = props.value;
  return (
    <MantinePagination
      {...props}
      getControlProps={(control) => ({ 'aria-label': controlLabels[control] })}
      getItemProps={(page) => ({
        'aria-label': `Страница ${page}`,
        'aria-current': page === current ? 'page' : undefined,
      })}
    />
  );
}
