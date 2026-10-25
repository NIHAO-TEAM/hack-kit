import { Card, Group, Text, Title, Tooltip } from '@mantine/core';
import { Link } from '@tanstack/react-router';
import classes from './LossByReason.module.css';
import { reasonRows, type ReasonLoss } from '../model/loss-by-reason';
import { formatCompact, formatMoney } from '@shared/lib';
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
                <Link
                  to="/queue"
                  search={{ reason: r.featureId }}
                  className={classes.link}
                  aria-label={`${r.title}: показать клиентов в очереди`}
                >
                  {r.title}
                </Link>
                <Text span>
                  <Money cents={r.lossCents} /> · {r.share}
                </Text>
              </Group>
              <Tooltip
                label={`${r.title}: ${formatMoney(r.lossCents)} · ${r.share} · клиентов ${formatCompact(r.customers)}`}
                events={{ hover: true, focus: true, touch: true }}
                openDelay={0}
              >
                <div
                  className={classes.track}
                  role="img"
                  tabIndex={0}
                  aria-label={`${r.title}: ${r.share} потери, клиентов ${r.customers}`}
                >
                  <div className={classes.fill} style={{ width: `${r.width}%` }} />
                </div>
              </Tooltip>
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
