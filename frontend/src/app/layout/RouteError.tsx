import { useRouter } from '@tanstack/react-router';
import { ApiError } from '@shared/api';
import { StateError, StateOffline } from '@shared/ui';

/** Ошибка загрузки маршрута: нет связи или сбой сервера, с повтором. */
export function RouteError({ error }: { error: unknown }) {
  const router = useRouter();
  const retry = () => void router.invalidate();
  if (error instanceof ApiError && error.isNetwork) {
    return <StateOffline onRetry={retry} />;
  }
  return (
    <StateError
      title="Не удалось загрузить страницу"
      requestId={error instanceof ApiError ? error.requestId : null}
      onRetry={retry}
    >
      Сервер ответил ошибкой. Повторите попытку.
    </StateError>
  );
}
