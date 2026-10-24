import { Card, Group, Text, Title } from '@mantine/core';
import classes from './LossByReason.module.css';
import { reasonRows, type ReasonLoss } from '../model/loss-by-reason';
import { Money } from '@shared/ui';

/** «Где теряем деньги»: ожидаемая потеря по причинам, по убыванию. Доли — по вкладу признаков. */
export function LossByReason({ items }: { items: readonly ReasonLoss[] }) {
  const rows = reasonRows(items);
  return (
    <Card withBorder aria-labelledby="loss-by-reason-title">
      <Title order={2} size="h4" id="loss-by-reason-title" mb="sm">
        Где теряем деньги
      </Title>
      {rows.length === 0 ? (
        <Text c="dimmed">Ожидаемой потери по причинам пока нет.</Text>
      ) : (
        <ul className={classes.list}>
          {rows.map((r) => (
            <li key={r.featureId}>
              <Group justify="space-between" gap="sm" wrap="nowrap">
                <Text fw={500}>{r.title}</Text>
                <Text span>
                  <Money cents={r.lossCents} /> · {r.share}
                </Text>
              </Group>
              <div
                className={classes.track}
                role="img"
                aria-label={`${r.title}: ${r.share} потери, клиентов ${r.customers}`}
              >
                <div className={classes.fill} style={{ width: `${r.width}%` }} />
              </div>
            </li>
          ))}
        </ul>
      )}
      <Text size="sm" c="dimmed" mt="sm">
        Доли распределены по вкладу признаков. Это не доказанная причинность.
      </Text>
    </Card>
  );
}
