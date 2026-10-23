import { ApiError } from '@shared/api';
import { loginErrorText } from './login-error';

describe('loginErrorText', () => {
  it('неверный пароль', () => {
    expect(loginErrorText(new ApiError(401, 'x'))).toBe('Неверный логин или пароль');
  });

  it('слишком много попыток: минуты из Retry-After, округление вверх', () => {
    const text = loginErrorText(new ApiError(429, 'x', { retryAfterSeconds: 301 }));
    expect(text).toContain('через 6 мин');
  });

  it('нет сети', () => {
    expect(loginErrorText(new ApiError(0, 'x'))).toContain('Нет связи');
  });
});
