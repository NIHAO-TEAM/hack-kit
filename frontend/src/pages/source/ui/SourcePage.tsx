import { Grid, Stack, Title } from '@mantine/core';
import { useQuery } from '@tanstack/react-query';
import { statusQuery } from '../api/source';
import { AuditLog } from '@widgets/audit-log';
import { ApiError } from '@shared/api';
import { StateError, StateLoading, StateOffline } from '@shared/ui';
import { EditPanel } from './EditPanel';
import { EmulatorPanel } from './EmulatorPanel';
import { PollingPanel } from './PollingPanel';
import { QualityChecks } from './QualityChecks';
import { StreamPanel } from './StreamPanel';
import { StatusHeader } from './StatusHeader';
import { UploadPanel } from './UploadPanel';

export function SourcePage() {
  const query = useQuery(statusQuery);
  const error = query.error instanceof ApiError ? query.error : null;

  if (query.isPending) return <StateLoading label="Загрузка состояния источника" />;
  if (!query.data) {
    return error?.isNetwork ? (
      <StateOffline onRetry={() => void query.refetch()} />
    ) : (
      <StateError
        title="Не удалось загрузить состояние источника"
        requestId={error?.requestId}
        onRetry={() => void query.refetch()}
      >
        {error?.message}
      </StateError>
    );
  }

  return (
    <Stack>
      <Title order={1}>Источник данных</Title>
      {query.isError ? <StateOffline onRetry={() => void query.refetch()} /> : null}
      <StatusHeader status={query.data} />
      <Grid>
        <Grid.Col span={{ base: 12, md: 7 }}>
          <Stack>
            <QualityChecks checks={query.data.checks} />
            <UploadPanel />
          </Stack>
        </Grid.Col>
        <Grid.Col span={{ base: 12, md: 5 }}>
          <Stack>
            <PollingPanel status={query.data} />
            <EditPanel />
            <EmulatorPanel />
            <StreamPanel />
          </Stack>
        </Grid.Col>
      </Grid>
      <AuditLog />
    </Stack>
  );
}
