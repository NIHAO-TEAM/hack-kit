import { Alert, Paper, Stack, Switch, Text, Title } from '@mantine/core';
import { useMutation } from '@tanstack/react-query';
import { setPolling, type SourceStatus } from '../api/source';
import { pollingNote } from '../model/polling';
import { useAfterWrite } from './useAfterWrite';

/** Опрос REST-биллинга: видно только при заданном BILLING_URL. */
export function PollingPanel({ status }: { status: SourceStatus }) {
  const after = useAfterWrite();
  const toggle = useMutation({
    mutationFn: (paused: boolean) => setPolling(paused),
    onSuccess: (p) => after.done(p.paused ? 'Опрос на паузе' : 'Опрос включён'),
    onError: (e) => after.fail('Не удалось переключить опрос', e),
  });
  const polling = status.polling;
  if (!polling) return null;
  const note = pollingNote(polling, status.loadedAt);

  return (
    <Paper withBorder p="lg" radius="lg" component="section" aria-label="Опрос биллинга">
      <Stack gap="sm">
        <Title order={2} size="h4">
          Опрос биллинга по API
        </Title>
        <Text size="sm" c="dimmed">
          Адаптер под наш контракт: формат API aéza неизвестен.
        </Text>
        <Switch
          label="Забирать данные из API автоматически"
          checked={!polling.paused}
          disabled={toggle.isPending}
          onChange={(e) => toggle.mutate(!e.currentTarget.checked)}
        />
        {note.failing ? (
          <Alert color="red" variant="light">
            {note.text}
          </Alert>
        ) : (
          <Text size="sm">{note.text}</Text>
        )}
      </Stack>
    </Paper>
  );
}
