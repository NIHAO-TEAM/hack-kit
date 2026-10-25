import { Stack, Title } from '@mantine/core';
import { AuditLog } from '@widgets/audit-log';

export function AuditPage() {
  return (
    <Stack>
      <Title order={1}>Журнал</Title>
      <AuditLog />
    </Stack>
  );
}
