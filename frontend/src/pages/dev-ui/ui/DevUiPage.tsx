import {
  Button,
  Divider,
  SegmentedControl,
  Stack,
  Text,
  Title,
  useMantineColorScheme,
} from '@mantine/core';
import type { ReactNode } from 'react';
import {
  Countdown,
  LevelBadge,
  Money,
  RescueWindow,
  StateEmpty,
  StateError,
  StateForbidden,
  StateLoading,
  StateOffline,
  tokenNames,
  type RiskLevel,
  type TokenName,
} from '@shared/ui';
import classes from './DevUiPage.module.css';

const AS_OF = '2026-10-19T15:00:00Z';

const groups: { title: string; meaning: string; match: (name: TokenName) => boolean }[] = [
  {
    title: 'Поверхности и текст',
    meaning: 'нормальное состояние — без цвета',
    match: (n) => /^(bg|surface|border|field|text|focus)/.test(n),
  },
  {
    title: 'Акцент',
    meaning: 'главное действие и активная навигация, одна заливка на экран',
    match: (n) => n.includes('primary'),
  },
  {
    title: 'Окно закрывается',
    meaning: 'меньше 5 дней — оранжевый',
    match: (n) => n.startsWith('warn'),
  },
  {
    title: 'Критично',
    meaning: 'меньше 24 часов — красный',
    match: (n) => n.startsWith('critical'),
  },
  { title: 'Эффект', meaning: 'эффект действия — бирюзовый', match: (n) => n.startsWith('effect') },
  { title: 'Потеряно', meaning: 'потеряно и закрыто — серый', match: (n) => n.startsWith('lost') },
];

const levels: RiskLevel[] = ['high', 'medium', 'low', 'unscored', 'lost'];

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <Stack gap="sm" component="section">
      <Title order={2} size="h4">
        {title}
      </Title>
      {children}
    </Stack>
  );
}

function Palette() {
  return (
    <Stack gap="lg">
      {groups.map((g) => (
        <Stack key={g.title} gap="xs">
          <Text fw={600}>
            {g.title}{' '}
            <Text span c="var(--text-muted)" fw={400}>
              — {g.meaning}
            </Text>
          </Text>
          <div className={classes.swatches}>
            {tokenNames.filter(g.match).map((name) => (
              <div key={name} className={classes.swatch}>
                <span className={classes.chip} style={{ background: `var(--${name})` }} />
                <code>--{name}</code>
              </div>
            ))}
          </div>
        </Stack>
      ))}
    </Stack>
  );
}

function ThemeSwitch() {
  const { colorScheme, setColorScheme } = useMantineColorScheme();
  return (
    <SegmentedControl
      aria-label="Тема"
      value={colorScheme}
      onChange={(v) => setColorScheme(v as 'light' | 'dark' | 'auto')}
      data={[
        { value: 'dark', label: 'Тёмная' },
        { value: 'light', label: 'Светлая' },
        { value: 'auto', label: 'Системная' },
      ]}
    />
  );
}

/** Витрина: токены, типографика и компоненты `shared/ui` в обеих темах. */
export function DevUiPage() {
  return (
    <Stack gap="xl">
      <Stack gap="sm">
        <Title order={1}>Витрина</Title>
        <ThemeSwitch />
      </Stack>

      <Section title="Палитра">
        <Palette />
      </Section>
      <Divider />

      <Section title="Типографика (Exo 2, Inter, табличные цифры)">
        <Title order={1}>Заголовок страницы</Title>
        <Title order={2}>Заголовок диалога</Title>
        <Title order={3}>Секция</Title>
        <Text>Основной текст: клиент не продлил услугу, окно спасения закрывается.</Text>
        <Text size="sm" c="var(--text-muted)">
          Мелкий поясняющий текст
        </Text>
        <Text>
          1 111,11 · 2 222,22 · 3 879,73 · 7 368,34{' '}
          <Text span c="var(--text-muted)">
            — цифры одной ширины
          </Text>
        </Text>
      </Section>
      <Divider />

      <Section title="Отступы и радиусы">
        <Text size="sm">
          Шаг 4 px: 4, 8, 12, 16, 20, 24, 32, 48. Радиусы: плашка 4, контрол 6, карточка 12, диалог
          14.
        </Text>
        <Button>Основное действие</Button>
        <Button variant="default">Вторичное</Button>
      </Section>
      <Divider />

      <Section title="Money">
        <Text>
          Ожидаемая потеря <Money cents={387973} /> · эффект{' '}
          <Money cents={36529} tone="effect" signed /> · потеряно{' '}
          <Money cents={736834} tone="lost" /> · <Money cents={-36529} />
        </Text>
      </Section>

      <Section title="Countdown">
        <Text>
          <Countdown until="2026-10-21T22:00:00Z" asOf={AS_OF} /> ·{' '}
          <Countdown until="2026-10-19T22:05:00Z" asOf={AS_OF} /> ·{' '}
          <Countdown until="2026-10-19T15:20:00Z" asOf={AS_OF} />
        </Text>
      </Section>

      <Section title="LevelBadge">
        <Stack gap="xs" align="flex-start">
          {levels.map((l) => (
            <LevelBadge key={l} level={l} />
          ))}
        </Stack>
      </Section>

      <Section title="RescueWindow">
        <div className={classes.card}>
          <Stack gap="lg">
            <Stack gap={4}>
              <Text size="sm" c="var(--text-muted)">
                До срока (обычное)
              </Text>
              <RescueWindow expiresAt="2026-10-21T00:00:00Z" asOf={AS_OF} />
            </Stack>
            <Stack gap={4}>
              <Text size="sm" c="var(--text-muted)">
                Окно закрывается — внимание (меньше 5 дней)
              </Text>
              <RescueWindow expiresAt="2026-10-17T00:00:00Z" asOf={AS_OF} />
            </Stack>
            <Stack gap={4}>
              <Text size="sm" c="var(--text-muted)">
                Меньше 24 часов — критично
              </Text>
              <RescueWindow expiresAt="2026-10-14T20:00:00Z" asOf={AS_OF} />
            </Stack>
            <Stack gap={4}>
              <Text size="sm" c="var(--text-muted)">
                Окно закрыто
              </Text>
              <RescueWindow expiresAt="2026-10-01T00:00:00Z" asOf={AS_OF} />
            </Stack>
            <Stack gap={4}>
              <Text size="sm" c="var(--text-muted)">
                Пусто (срока нет)
              </Text>
              <RescueWindow expiresAt={null} asOf={AS_OF} />
            </Stack>
          </Stack>
        </div>
      </Section>
      <Divider />

      <Section title="Состояния экрана: загрузка">
        <StateLoading />
      </Section>
      <Section title="Пусто">
        <StateEmpty title="Клиентов под риском нет">Когда появятся, они будут здесь.</StateEmpty>
      </Section>
      <Section title="Ошибка">
        <StateError
          title="Не удалось загрузить очередь"
          requestId="req-42"
          onRetry={() => undefined}
        >
          Сервер не ответил за 10 секунд.
        </StateError>
      </Section>
      <Section title="Нет связи">
        <StateOffline onRetry={() => undefined} />
      </Section>
      <Section title="Нет прав">
        <StateForbidden>Ваша роль — маркетолог. Раздел «Модель» доступен аналитику.</StateForbidden>
      </Section>
    </Stack>
  );
}
