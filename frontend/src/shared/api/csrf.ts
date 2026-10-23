const KEY = 'csrf-token';

let memory: string | null = null;

/** Токен хранится в памяти и sessionStorage: после перезагрузки вкладки он не теряется. */
export function getCsrfToken(): string | null {
  if (memory) return memory;
  try {
    memory = sessionStorage.getItem(KEY);
  } catch {
    // хранилище недоступно — остаётся только память
  }
  return memory;
}

export function setCsrfToken(token: string | null): void {
  memory = token;
  try {
    if (token) sessionStorage.setItem(KEY, token);
    else sessionStorage.removeItem(KEY);
  } catch {
    // хранилище недоступно — токен только в памяти
  }
}
