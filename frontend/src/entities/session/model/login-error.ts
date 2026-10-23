import { ApiError } from '@shared/api';

/** Текст ошибки входа для человека. */
export function loginErrorText(error: unknown): string {
  if (!(error instanceof ApiError)) return 'Не удалось войти. Попробуйте ещё раз.';
  if (error.status === 401) return 'Неверный логин или пароль';
  if (error.status === 429) {
    const minutes = Math.max(1, Math.ceil((error.retryAfterSeconds ?? 60) / 60));
    return `Слишком много попыток входа. Повторите через ${minutes} мин.`;
  }
  if (error.isNetwork) return 'Нет связи с сервером. Проверьте соединение и повторите.';
  return 'Не удалось войти. Попробуйте ещё раз.';
}
