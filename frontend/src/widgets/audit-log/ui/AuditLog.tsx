import { Group, Pagination, Paper, Stack, Text, Title } from '@mantine/core';
import { useQuery } from '@tanstack/react-query';
import { PAGE_SIZE, auditQuery } from '../api/audit';
import { changeText, clampPage } from '../model/labels';
import { StateEmpty, StateError, StateLoading } from '@shared/ui';
import { useState } from 'react';

const when = new Intl.DateTimeFormat('ru-RU', { dateStyle: 'short', timeStyle: 'medium' });

/** Журнал действий: кто и когда менял модель и данные. Новые записи сверху, по 10 на страницу. */
export function AuditLog() {
  const [page, setPage] = useState(0);
  const query = useQuery(auditQuery(page));

  let body;
  if (query.isPending) body = <StateLoading label="Загрузка журнала" />;
  else if (query.isError && !query.data) {
    body = (
      <StateError title="Не удалось загрузить журнал" onRetry={() => void query.refetch()}>
        {query.error.message}
      </StateError>
    );
  } else if (query.data) {
    const { items, total } = query.data;
    const pages = Math.max(1, Math.ceil(total / PAGE_SIZE));
    body =
      items.length === 0 ? (
        <StateEmpty title="Записей пока нет">Правки настроек и данных появятся здесь.</StateEmpty>
      ) : (
        <Stack gap="sm">
          <Stack
            component="ul"
            gap="sm"
            p={0}
            m={0}
            style={{ listStyle: 'none' }}
            aria-label="Записи журнала"
          >
            {items.map((e) => (
              <li key={e.id}>
                <Group justify="space-between" wrap="nowrap" align="flex-start" gap="md">
                  <Stack gap={2}>
                    <Text fw={600}>{e.summary}</Text>
                    {e.changes.map((c) => (
                      <Text key={c.field} size="sm" c="dimmed">
                        {changeText(c)}
                      </Text>
                    ))}
                  </Stack>
                  <Stack gap={2} align="flex-end" style={{ flexShrink: 0, maxWidth: '50%' }}>
                    <Text size="sm" style={{ overflowWrap: 'anywhere' }}>
                      {e.actor}
                    </Text>
                    <Text size="xs" c="dimmed">
                      <time dateTime={e.at}>{when.format(new Date(e.at))}</time>
                    </Text>
                  </Stack>
                </Group>
              </li>
            ))}
          </Stack>
          {pages > 1 ? (
            <Pagination
              total={pages}
              value={clampPage(page, total, PAGE_SIZE) + 1}
              onChange={(p) => setPage(p - 1)}
              size="sm"
              aria-label="Страницы журнала"
            />
          ) : null}
        </Stack>
      );
  }

  return (
    <Paper withBorder p="lg" radius="lg" component="section" aria-label="Журнал действий">
      <Stack gap="sm">
        <Title order={2} size="h4">
          Журнал
        </Title>
        {body}
      </Stack>
    </Paper>
  );
}
