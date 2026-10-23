/** Подписи проверок качества данных; неизвестный код показываем как есть. */
const titles: Record<string, string> = {
  prolong_wait_over_5d: 'Ошибка данных: в ожидании оплаты дольше 5 дней',
  paid_term_mismatch: 'Срок услуги не сходится с платежами',
  no_transactions: 'У услуги нет платежей',
};

export function qualityTitle(id: string): string {
  return titles[id] ?? id;
}

/** Евро из поля формы («529,84», «529.84», «1 000») → центы; пусто или не число → null. */
export function eurosToCents(text: string): number | null {
  const t = text.replace(/\s/g, '').replace(',', '.');
  if (!/^\d+(\.\d{1,2})?$/.test(t)) return null;
  return Math.round(Number(t) * 100);
}

export const sourceLabel: Record<string, string> = {
  seed: 'Набор организаторов',
  api: 'Загрузка через API',
  emulator: 'Эмулятор биллинга',
};
