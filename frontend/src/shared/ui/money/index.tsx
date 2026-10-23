import { Text } from '@mantine/core';
import { formatMoney } from '../../lib';

type MoneyTone = 'default' | 'effect' | 'lost';

const color: Record<MoneyTone, string> = {
  default: 'var(--text-strong)',
  effect: 'var(--effect-fg)',
  lost: 'var(--lost-fg)',
};

/** Деньги в евро из центов. Бирюзовый — эффект действия, серый — потеряно. */
export function Money({
  cents,
  tone = 'default',
  signed = false,
}: {
  cents: number;
  tone?: MoneyTone;
  /** Плюс перед положительным значением (эффект: «+365,29 €»). */
  signed?: boolean;
}) {
  const text = formatMoney(cents);
  return (
    <Text span inherit fw={500} c={color[tone]}>
      {signed && cents > 0 ? `+${text}` : text}
    </Text>
  );
}
