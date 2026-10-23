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
import { useEffect, useState } from 'react';
import { loginErrorText } from '@entities/session';

const SSO_LOGIN_URL = '/api/v1/auth/oidc/login';

export function LoginPage({
  onSubmit,
}: {
  onSubmit: (credentials: { login: string; password: string }) => Promise<void>;
}) {
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);
  const [ssoOff, setSsoOff] = useState(false);

  // 501 — SSO не настроен на сервере: кнопка не нужна. Другие ответы и сбои сети кнопку оставляют.
  useEffect(() => {
    const ctrl = new AbortController();
    fetch(SSO_LOGIN_URL, { redirect: 'manual', signal: ctrl.signal })
      .then((res) => setSsoOff(res.status === 501))
      .catch(() => {});
    return () => ctrl.abort();
  }, []);
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
            {ssoOff ? null : (
              <Button component="a" href={SSO_LOGIN_URL} variant="default">
                Войти через SSO
              </Button>
            )}
          </Stack>
        </form>
      </Paper>
    </Container>
  );
}
