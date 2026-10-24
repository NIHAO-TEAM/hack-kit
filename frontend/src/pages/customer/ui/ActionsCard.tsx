import { Badge, Card, Stack, Text, Title } from '@mantine/core';
import type { CustomerAction } from '@entities/customer-detail';
import { Changed, Money } from '@shared/ui';
import classes from './ActionsCard.module.css';

/** «На каждый €»: снятая потеря на евро цены, два знака. Пусто при нулевой цене. */
function formatReturn(v: number | undefined): string {
  if (v === undefined || v === null) return '—';
  return `${v.toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 })} €`;
}

/** Таблица действий с ценой: что стоит, сколько потери снимает, отдача на евро. Применение — в эмуляторе (этап 5). */
export function ActionsCard({
  actions,
  combo,
  changed = false,
}: {
  actions: readonly CustomerAction[];
  /** Эффект всех применимых действий вместе, центы; нет, если действий меньше двух. */
  combo?: number | null;
  /** Действия изменились после пересчёта: содержимое карточки подсвечено. */
  changed?: boolean;
}) {
  if (actions.length === 0) {
    return (
      <Card withBorder radius="sm" padding="md">
        <Text c="dimmed">Действий для этого клиента нет.</Text>
      </Card>
    );
  }
  // первое действие — с наибольшим снижением потери (сервер сортирует)
  const bestId = actions[0]?.id;
  return (
    <Card withBorder radius="sm" padding="md">
      <Changed active={changed}>
        <Stack gap="sm">
          <Title order={2} size="h4">
            Что сделать
          </Title>
          <Text size="sm" c="dimmed">
            Сумма — расчётное снижение ожидаемой потери, если клиент выполнит действие полностью.
            Отклик и стоимость контакта не учтены; «на каждый €» — оценка масштаба, не окупаемость.
          </Text>
          <table className={classes.table} aria-label="Что можно сделать">
            <thead>
              <tr>
                <th scope="col">Действие</th>
                <th scope="col" className={classes.num}>
                  Цена
                </th>
                <th scope="col" className={classes.num}>
                  Снимает потерю
                </th>
                <th scope="col" className={classes.num}>
                  На каждый €
                </th>
              </tr>
            </thead>
            <tbody>
              {actions.map((a) => (
                <tr key={a.id}>
                  <td>
                    <Text span fw={a.id === bestId ? 600 : 400} c="var(--text-strong)">
                      {a.title}
                    </Text>
                    {a.id === 'billing' || a.id === 'to_billing' ? (
                      <Badge variant="outline" ml="xs">
                        к биллингу
                      </Badge>
                    ) : null}
                    {a.id === bestId ? (
                      <Badge variant="light" ml="xs">
                        лучшее по эффекту
                      </Badge>
                    ) : null}
                  </td>
                  <td className={classes.num} data-label="Цена">
                    <Money cents={a.costCents ?? 0} />
                  </td>
                  <td className={classes.num} data-label="Снимает потерю">
                    <Money cents={a.effectCents} tone="effect" />
                  </td>
                  <td className={classes.num} data-label="На каждый €">
                    {formatReturn(a.returnPerEuro)}
                  </td>
                </tr>
              ))}
              {combo !== undefined && combo !== null ? (
                <tr>
                  <td>
                    <Text span fw={600} c="var(--text-strong)">
                      Всё вместе
                    </Text>
                  </td>
                  <td className={classes.num} data-label="Цена">
                    —
                  </td>
                  <td className={classes.num} data-label="Снимает потерю">
                    <Money cents={combo} tone="effect" />
                  </td>
                  <td className={classes.num} data-label="На каждый €">
                    —
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </Stack>
      </Changed>
    </Card>
  );
}
