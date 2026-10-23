import {
  Alert,
  Button,
  Container,
  Paper,
  PasswordInput,
  Stack,
  TextInput,
  Title,
} from '@mantine/core';
import { useForm } from '@mantine/form';
import { useState } from 'react';
import { loginErrorText } from '@entities/session';

export function LoginPage({
  onSubmit,
}: {
  onSubmit: (credentials: { login: string; password: string }) => Promise<void>;
}) {
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);
  const form = useForm({
    initialValues: { login: '', password: '' },
    validate: {
      login: (v) => (v.trim() ? null : 'Введите логин'),
      password: (v) => (v ? null : 'Введите пароль'),
    },
  });

  async function submit(values: { login: string; password: string }) {
    setError(null);
    setPending(true);
    try {
      await onSubmit({ login: values.login.trim(), password: values.password });
    } catch (e) {
      setError(loginErrorText(e));
      form.setFieldValue('password', '');
    } finally {
      setPending(false);
    }
  }

  return (
    <Container size={400} px="md" mt="15vh" component="main">
      <Paper withBorder p="xl">
        <form onSubmit={form.onSubmit(submit)} noValidate>
          <Stack>
            <Title order={1} size="h3">
              Контур удержания
            </Title>
            <TextInput
              label="Логин"
              autoComplete="username"
              data-autofocus
              {...form.getInputProps('login')}
            />
            <PasswordInput
              label="Пароль"
              autoComplete="current-password"
              {...form.getInputProps('password')}
            />
            {error ? (
              <Alert color="red" role="alert">
                {error}
              </Alert>
            ) : null}
            <Button type="submit" loading={pending}>
              Войти
            </Button>
          </Stack>
        </form>
      </Paper>
    </Container>
  );
}
