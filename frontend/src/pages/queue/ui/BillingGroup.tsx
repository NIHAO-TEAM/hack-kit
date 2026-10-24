import { Accordion, Anchor, Button, Group, Skeleton, Stack, Table, Text } from '@mantine/core';
import type { CustomerRow } from '@entities/customer';
import { Money } from '@shared/ui';

/**
 * Клиенты со сбоем автосписания: ими занимается биллинг, денежного действия маркетолога нет.
 * Группа свёрнута; при N = 0 её нет.
 */
export function BillingGroup({
  rows,
  total,
  loading,
  failed,
  onRetry,
  onOpen,
}: {
  rows: CustomerRow[] | undefined;
  total: number | undefined;
  loading: boolean;
  failed: boolean;
  onRetry: () => void;
  onOpen: (id: number) => void;
}) {
  if (loading)
    return <Skeleton height={44} radius="sm" aria-label="Загрузка группы «У биллинга»" />;
  if (failed)
    return (
      <Group gap="sm" role="alert">
        <Text fw={600}>У биллинга — не загрузилось</Text>
        <Button variant="default" size="xs" onClick={onRetry}>
          Повторить
        </Button>
      </Group>
    );
  if (!rows || rows.length === 0) return null;
  return (
    <Accordion variant="separated" radius="sm" chevronPosition="left">
      <Accordion.Item value="billing">
        <Accordion.Control>
          <Text fw={600}>У биллинга ({total ?? rows.length})</Text>
        </Accordion.Control>
        <Accordion.Panel>
          <Stack gap="xs">
            <Table aria-label="Клиенты у биллинга">
              <Table.Thead>
                <Table.Tr>
                  <Table.Th>ID</Table.Th>
                  <Table.Th>Email</Table.Th>
                  <Table.Th ta="right">Баланс</Table.Th>
                  <Table.Th>Причина</Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {rows.map((r) => (
                  <Table.Tr key={r.id}>
                    <Table.Td>{r.id}</Table.Td>
                    <Table.Td>
                      <Anchor
                        href={`/queue/${r.id}`}
                        onClick={(e) => {
                          e.preventDefault();
                          onOpen(r.id);
                        }}
                      >
                        {r.email}
                      </Anchor>
                    </Table.Td>
                    <Table.Td ta="right">
                      <Money cents={r.balance} />
                    </Table.Td>
                    <Table.Td>
                      {r.topAction?.id === 'to_billing'
                        ? r.topAction.title
                        : 'К биллингу: сбой автосписания'}
                    </Table.Td>
                  </Table.Tr>
                ))}
              </Table.Tbody>
            </Table>
          </Stack>
        </Accordion.Panel>
      </Accordion.Item>
    </Accordion>
  );
}
