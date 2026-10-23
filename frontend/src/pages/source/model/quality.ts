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
