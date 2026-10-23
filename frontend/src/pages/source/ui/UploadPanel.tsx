import { Alert, Button, FileInput, List, Paper, Stack, Text, Title } from '@mantine/core';
import { useMutation } from '@tanstack/react-query';
import { useState } from 'react';
import { putSnapshot } from '../api/source';
import { ApiError } from '@shared/api';
import { useAfterWrite } from './useAfterWrite';

/** Загрузка файла снимка. Не прошёл проверку — показываем поля по-человечески, данные остаются прежними. */
export function UploadPanel() {
  const after = useAfterWrite();
  const [file, setFile] = useState<File | null>(null);
  const upload = useMutation({
    mutationFn: (f: File) => putSnapshot(f),
    onSuccess: (r) => {
      setFile(null);
      after.done(`Снимок принят, версия ${r.version}`);
    },
  });
  const error = upload.error instanceof ApiError ? upload.error : null;
  const fields = error ? Object.entries(error.fieldErrors) : [];

  return (
    <Paper withBorder p="lg" radius="lg" component="section" aria-label="Загрузка снимка">
      <Stack gap="sm">
        <Title order={2} size="h4">
          Загрузить снимок
        </Title>
        <Text size="sm" c="dimmed">
          Файл JSON с клиентами, услугами и платежами заменит текущие данные целиком, очередь
          пересчитается.
        </Text>
        <FileInput
          label="Файл снимка"
          placeholder="snapshot.json"
          styles={{ placeholder: { color: 'var(--text-muted)' } }}
          accept="application/json,.json"
          value={file}
          onChange={setFile}
          clearable
        />
        <Button
          disabled={!file}
          loading={upload.isPending}
          onClick={() => file && upload.mutate(file)}
        >
          Загрузить
        </Button>
        {error ? (
          <Alert color="red" title="Снимок не принят, данные прежние" role="alert">
            {fields.length > 0 ? (
              <List size="sm">
                {fields.map(([field, message]) => (
                  <List.Item key={field}>
                    {field}: {message}
                  </List.Item>
                ))}
              </List>
            ) : (
              error.message
            )}
          </Alert>
        ) : null}
      </Stack>
    </Paper>
  );
}
