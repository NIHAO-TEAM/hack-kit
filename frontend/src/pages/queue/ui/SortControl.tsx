import { NativeSelect } from '@mantine/core';
import type { CustomerOrder, CustomerSort } from '@entities/customer';
import { IconArrowDown, IconArrowUp } from '@shared/ui';
import classes from './SortControl.module.css';

const fields: Array<{ value: CustomerSort; label: string }> = [
  { value: 'id', label: 'ID' },
  { value: 'email', label: 'Email' },
  { value: 'balance', label: 'Баланс' },
  { value: 'totalReplenished', label: 'Пополнено' },
  { value: 'score', label: 'Скор' },
  { value: 'expectedLoss', label: 'Ожидаемая потеря' },
];

/**
 * Сортировка на планшете и телефоне, где заголовки таблицы спрятаны.
 * Те же `sort`/`order`, что у заголовков: направление меняется отдельной кнопкой.
 */
export function SortControl({
  sort,
  order,
  onField,
  onToggleOrder,
}: {
  sort: CustomerSort;
  order: CustomerOrder;
  /** Выбор поля: направление по умолчанию задаёт страница, как при клике по заголовку. */
  onField: (column: CustomerSort) => void;
  onToggleOrder: () => void;
}) {
  const Icon = order === 'asc' ? IconArrowUp : IconArrowDown;
  return (
    <div className={classes.root} role="group" aria-label="Сортировка">
      <NativeSelect
        className={classes.field}
        classNames={{ input: classes.input }}
        aria-label="Сортировать по"
        data={fields}
        value={sort}
        onChange={(e) => onField(e.currentTarget.value as CustomerSort)}
      />
      <button type="button" className={classes.order} onClick={onToggleOrder}>
        <Icon size={16} aria-hidden />
        {order === 'asc' ? 'по возрастанию' : 'по убыванию'}
      </button>
    </div>
  );
}
