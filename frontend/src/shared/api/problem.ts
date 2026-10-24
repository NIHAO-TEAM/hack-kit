import type { components } from './schema.gen';

type Problem = components['schemas']['Problem'];

/** Ошибка API: `application/problem+json` либо сбой сети (`status` 0). */
export class ApiError extends Error {
  readonly status: number;
  readonly retryAfterSeconds: number | null;
  readonly fieldErrors: Record<string, string>;
  readonly requestId: string | null;

  constructor(
    status: number,
    message: string,
    options: {
      retryAfterSeconds?: number | null;
      fieldErrors?: Record<string, string>;
      requestId?: string | null;
    } = {},
  ) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.retryAfterSeconds = options.retryAfterSeconds ?? null;
    this.fieldErrors = options.fieldErrors ?? {};
    this.requestId = options.requestId ?? null;
  }

  get isNetwork(): boolean {
    return this.status === 0;
  }
}

export function toApiError(status: number, body: unknown, response?: Response): ApiError {
  const problem = (body && typeof body === 'object' ? body : {}) as Partial<Problem>;
  const retryAfter = Number(response?.headers.get('Retry-After'));
  // сервер пишет причину в detail, а в title — «Forbidden»: для CSRF показываем понятную фразу
  const csrfRejected = status === 403 && /X-CSRF-Token/i.test(problem.detail ?? '');
  // 503 — перегрузка сервера (быстрый отказ): title «Service Unavailable» человеку ничего не говорит
  const message = csrfRejected
    ? 'Сессия устарела, обновите страницу'
    : status === 503
      ? 'Сервис перегружен, повторите через несколько секунд'
      : (problem.title ?? problem.detail ?? 'Ошибка запроса');
  return new ApiError(status, message, {
    retryAfterSeconds: Number.isFinite(retryAfter) && retryAfter > 0 ? retryAfter : null,
    fieldErrors: Object.fromEntries((problem.errors ?? []).map((e) => [e.field, e.message])),
    requestId: problem.correlation_id ?? response?.headers.get('X-Request-Id') ?? null,
  });
}
