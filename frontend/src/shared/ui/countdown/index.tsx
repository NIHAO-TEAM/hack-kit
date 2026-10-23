import { Text } from '@mantine/core';
import { formatCountdown } from '../../lib';

/** Остаток до срока от времени стенда (`asOf`), а не от часов браузера. */
export function Countdown({ until, asOf }: { until: string; asOf: string }) {
  const ms = Date.parse(until) - Date.parse(asOf);
  return (
    <Text span inherit>
      {formatCountdown(ms)}
    </Text>
  );
}
