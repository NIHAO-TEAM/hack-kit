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

const AUTH_CONFIG_URL = '/api/v1/auth/config';
const SSO_LOGIN_URL = '/api/v1/auth/oidc/login';

export function LoginPage({
  onSubmit,
}: {
  onSubmit: (credentials: { login: string; password: string }) => Promise<void>;
}) {
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);
  const [ssoEnabled, setSsoEnabled] = useState(false);

  // Пока ответа нет и при ошибке запроса кнопки SSO нет: вход по паролю работает без неё.
  useEffect(() => {
    const ctrl = new AbortController();
    fetch(AUTH_CONFIG_URL, { signal: ctrl.signal })
      .then((res) => (res.ok ? res.json() : null))
      .then((cfg: { ssoEnabled?: boolean } | null) => setSsoEnabled(cfg?.ssoEnabled === true))
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
            {ssoEnabled ? (
              <Button component="a" href={SSO_LOGIN_URL} variant="default">
                Войти через SSO
              </Button>
            ) : null}
          </Stack>
        </form>
      </Paper>
    </Container>
  );
}
