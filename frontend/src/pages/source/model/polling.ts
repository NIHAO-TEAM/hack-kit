import type { components } from '@shared/api';

type Polling = components['schemas']['SourcePolling'];

const hm = new Intl.DateTimeFormat('ru-RU', { timeStyle: 'short', timeZone: 'UTC' });

/** Строка состояния опроса: пауза после ручной загрузки, сбой с временем последних данных или норма. */
export function pollingNote(p: Polling, loadedAt: string): { text: string; failing: boolean } {
  if (p.paused) {
    return {
      text: 'Опрос на паузе: ручная загрузка главнее. Включите, чтобы снова брать данные из API.',
      failing: false,
    };
  }
  if (p.lastError) {
    return {
      text: `Биллинг не отвечает, данные от ${hm.format(new Date(loadedAt))} UTC. Сбоев: ${p.errorsTotal}. ${p.lastError}`,
      failing: true,
    };
  }
  return { text: `Опрос каждые ${p.intervalSeconds} с, сбоев: ${p.errorsTotal}.`, failing: false };
}
