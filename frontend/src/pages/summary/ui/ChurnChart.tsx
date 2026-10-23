import { Box, Group, Stack, Text } from '@mantine/core';
import type { ChurnPoint } from '../api/analytics';
import { layout, path } from '../model/geometry';

const BOX = { width: 720, height: 260, left: 36, right: 16, top: 16, bottom: 32 };

const dateFmt = new Intl.DateTimeFormat('ru-RU', {
  day: 'numeric',
  month: 'short',
  timeZone: 'UTC',
});
const fmt = (iso: string) => dateFmt.format(new Date(`${iso}T00:00:00Z`));

/**
 * График оттока по неделям. Факт — сплошная линия с маркерами-кругами, прогноз — штриховая с
 * ромбами другого цвета: различимы и без цвета. Вертикаль «сейчас» — последняя неделя с фактом.
 */
export function ChurnChart({ points }: { points: ChurnPoint[] }) {
  const g = layout(points, BOX);
  const ticks = [0, 1, 2, 3, 4];
  const step = g.yMax / 4;
  const desc = points
    .map(
      (p) =>
        `${fmt(p.weekStart)}: ${p.actual !== null ? `удалено ${p.actual}` : `прогноз ${p.forecast}`}`,
    )
    .join('; ');

  return (
    <Stack gap="xs">
      <Box
        component="svg"
        viewBox={`0 0 ${BOX.width} ${BOX.height}`}
        role="img"
        aria-label="График оттока по неделям: удаления услуг и прогноз непродлений"
        style={{ width: '100%', height: 'auto', maxHeight: 320 }}
      >
        <desc>{desc}</desc>
        {ticks.map((t) => {
          const v = t * step;
          return (
            <g key={t}>
              <line
                x1={BOX.left}
                x2={BOX.width - BOX.right}
                y1={g.yOf(v)}
                y2={g.yOf(v)}
                stroke="var(--border)"
                strokeWidth={1}
              />
              <text
                x={BOX.left - 8}
                y={g.yOf(v)}
                textAnchor="end"
                dominantBaseline="middle"
                fontSize={11}
                fill="var(--text-muted)"
              >
                {v}
              </text>
            </g>
          );
        })}
        {points.map((p, i) =>
          i % 2 === 0 ? (
            <text
              key={p.weekStart}
              x={g.xOf(i)}
              y={BOX.height - 10}
              textAnchor="middle"
              fontSize={11}
              fill="var(--text-muted)"
            >
              {fmt(p.weekStart)}
            </text>
          ) : null,
        )}
        {g.nowX !== null ? (
          <g data-testid="chart-now">
            <line
              x1={g.nowX}
              x2={g.nowX}
              y1={BOX.top}
              y2={BOX.height - BOX.bottom}
              stroke="var(--text-subtle)"
              strokeWidth={1}
            />
            <text x={g.nowX + 4} y={BOX.top + 8} fontSize={11} fill="var(--text-strong)">
              сейчас
            </text>
          </g>
        ) : null}
        <path
          d={path(g.forecast)}
          fill="none"
          stroke="var(--warn-solid)"
          strokeWidth={2}
          strokeDasharray="6 4"
          data-testid="chart-forecast"
        />
        <path
          d={path(g.actual)}
          fill="none"
          stroke="var(--primary-fg)"
          strokeWidth={2.5}
          data-testid="chart-actual"
        />
        {g.actual.map((p) => (
          <circle key={`a${p.x}`} cx={p.x} cy={p.y} r={3.5} fill="var(--primary-fg)" />
        ))}
        {g.forecast.map((p) => (
          <rect
            key={`f${p.x}`}
            x={p.x - 3.5}
            y={p.y - 3.5}
            width={7}
            height={7}
            fill="var(--warn-solid)"
            transform={`rotate(45 ${p.x} ${p.y})`}
          />
        ))}
      </Box>
      <Group gap="lg" aria-label="Легенда графика">
        <Group gap={6}>
          <svg width="28" height="10" aria-hidden="true">
            <line x1="0" x2="28" y1="5" y2="5" stroke="var(--primary-fg)" strokeWidth="2.5" />
          </svg>
          <Text size="sm">Факт: услуги удалены биллингом</Text>
        </Group>
        <Group gap={6}>
          <svg width="28" height="10" aria-hidden="true">
            <line
              x1="0"
              x2="28"
              y1="5"
              y2="5"
              stroke="var(--warn-solid)"
              strokeWidth="2"
              strokeDasharray="6 4"
            />
          </svg>
          <Text size="sm">Прогноз: ожидаемые непродления (сумма вероятностей)</Text>
        </Group>
      </Group>
      <Text size="xs" c="dimmed">
        Дата удаления — срок услуги плюс 5 дней ожидания оплаты.
      </Text>
    </Stack>
  );
}
