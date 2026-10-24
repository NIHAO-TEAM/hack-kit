import { Box, Group, Paper, SimpleGrid, Stack, Text } from '@mantine/core';
import { formatCount, type StreamWindowPoint } from '@entities/stream-run';
import { linePoints, niceMax, pathOf, seriesOf, type Series } from '../model/window';

const BOX = { width: 240, height: 56 };
/** Вторая линия отличается штрихом, а не только цветом. */
const LINES = [
  { stroke: 'var(--primary-fg)', dash: undefined },
  { stroke: 'var(--warn-solid)', dash: '5 3' },
];

function fmtValue(v: number, unit: string): string {
  if (unit === 'мс') return `${v.toFixed(v < 10 ? 1 : 0).replace('.', ',')} мс`;
  return Number.isInteger(v) ? formatCount(v) : v.toFixed(1).replace('.', ',');
}

function Tile({ s }: { s: Series }) {
  const all = s.lines.flatMap((l) => l.values);
  const yMax = niceMax(Math.max(0, ...all));
  const last = (i: number) => s.lines[i]?.values.at(-1) ?? 0;
  const summary = s.lines
    .map((l) => `${l.label} ${fmtValue(l.values.at(-1) ?? 0, s.unit)}`)
    .join(', ');
  return (
    <Paper withBorder p="sm" radius="md" component="figure" m={0} data-testid={`live-${s.key}`}>
      <Stack gap={4}>
        <Text component="figcaption" size="sm" fw={500}>
          {s.title}
        </Text>
        <Text size="xl" fw={600} lh={1.1}>
          {fmtValue(last(s.lines.length - 1), s.unit)}{' '}
          <Text span size="xs" c="dimmed">
            {s.unit}
          </Text>
        </Text>
        <Box
          component="svg"
          viewBox={`0 0 ${BOX.width} ${BOX.height}`}
          role="img"
          aria-label={`${s.title} за последние 60 секунд: ${summary}`}
          style={{ width: '100%', height: 'auto' }}
        >
          <line x1={0} x2={BOX.width} y1={BOX.height} y2={BOX.height} stroke="var(--border)" />
          {s.lines.map((l, i) => (
            <path
              key={l.label}
              d={pathOf(linePoints(l.values, BOX, yMax))}
              fill="none"
              stroke={(LINES[i] ?? LINES[0]!).stroke}
              strokeDasharray={LINES[i]?.dash}
              strokeWidth={2}
            />
          ))}
        </Box>
        <Group gap="sm">
          {s.lines.map((l, i) => (
            <Text key={l.label} size="xs" c="dimmed">
              {s.lines.length > 1 ? `${i === 0 ? '—' : '- -'} ` : ''}
              {l.label} · пик {fmtValue(Math.max(0, ...l.values), s.unit)}
            </Text>
          ))}
        </Group>
      </Stack>
    </Paper>
  );
}

/** Живые графики прогона: шесть рядов за последние 60 с, цифры берутся из окна, пришедшего с сервера. */
export function LiveCharts({ window }: { window: StreamWindowPoint[] }) {
  if (window.length === 0) return null;
  return (
    <Stack gap="xs" component="section" aria-label="Живые графики потока">
      <SimpleGrid cols={{ base: 1, sm: 2, lg: 3 }} spacing="sm">
        {seriesOf(window).map((s) => (
          <Tile key={s.key} s={s} />
        ))}
      </SimpleGrid>
      <Text size="xs" c="dimmed">
        Последние 60 секунд, по точке на секунду. Ошибок должно быть 0.
      </Text>
    </Stack>
  );
}
