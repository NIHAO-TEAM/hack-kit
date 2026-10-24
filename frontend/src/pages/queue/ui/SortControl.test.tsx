import { MantineProvider } from '@mantine/core';
import { fireEvent, render, screen } from '@testing-library/react';
import { SortControl } from './SortControl';

const setup = (order: 'asc' | 'desc') => {
  const onField = vi.fn();
  const onToggleOrder = vi.fn();
  render(
    <MantineProvider>
      <SortControl
        sort="expectedLoss"
        order={order}
        onField={onField}
        onToggleOrder={onToggleOrder}
      />
    </MantineProvider>,
  );
  return { onField, onToggleOrder };
};

describe('SortControl: сортировка на узком экране', () => {
  it('все поля кейса доступны для выбора', () => {
    setup('desc');
    const options = screen.getAllByRole('option').map((o) => o.textContent);
    expect(options).toEqual(['ID', 'Email', 'Баланс', 'Пополнено', 'Скор', 'Ожидаемая потеря']);
  });

  it('выбор «Баланс» передаёт поле, кнопка меняет направление', () => {
    const { onField, onToggleOrder } = setup('desc');
    fireEvent.change(screen.getByLabelText('Сортировать по'), { target: { value: 'balance' } });
    expect(onField).toHaveBeenCalledWith('balance');
    fireEvent.click(screen.getByRole('button', { name: 'по убыванию' }));
    expect(onToggleOrder).toHaveBeenCalledTimes(1);
  });

  it('подпись кнопки — текущее направление', () => {
    setup('asc');
    expect(screen.getByRole('button', { name: 'по возрастанию' })).toBeInTheDocument();
  });
});
