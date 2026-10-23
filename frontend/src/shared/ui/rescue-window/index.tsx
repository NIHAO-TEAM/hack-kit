import { formatCountdown } from '../../lib';
import classes from './RescueWindow.module.css';

const DAY = 86_400_000;
export const WAIT_DAYS = 5;

type Phase = 'before' | 'wait' | 'closed';

/**
 * Полоса «окно спасения»: срок → 5 дней ожидания → удаление.
 * Положение метки «сейчас» считается от времени стенда `asOf`; уровни и деньги приходят с сервера.
 */
export function RescueWindow({
  expiresAt,
  asOf,
}: {
  /** Срок услуги: с него начинаются 5 дней ожидания. */
  expiresAt: string | null;
  asOf: string;
}) {
  if (!expiresAt) {
    return (
      <div className={classes.root}>
        <span className={classes.caption} data-tone="lost">
          окна нет
        </span>
      </div>
    );
  }
  const now = Date.parse(asOf);
  const expires = Date.parse(expiresAt);
  const deletion = expires + WAIT_DAYS * DAY;
  const phase: Phase = now < expires ? 'before' : now < deletion ? 'wait' : 'closed';
  const leftToDeletion = deletion - now;
  const tone =
    phase === 'closed'
      ? 'lost'
      : phase === 'wait'
        ? leftToDeletion < DAY
          ? 'critical'
          : 'warn'
        : null;

  // Лента: 1 часть «до срока» и 2,5 части ожидания, метка кладётся пропорционально.
  const lead = 1 / 3.5;
  const position =
    phase === 'before'
      ? Math.max(0, 1 - (expires - now) / (2 * DAY)) * lead
      : phase === 'wait'
        ? lead + ((now - expires) / (WAIT_DAYS * DAY)) * (1 - lead)
        : 1;

  const caption =
    phase === 'closed'
      ? 'окно закрыто'
      : phase === 'wait'
        ? `до удаления ${formatCountdown(leftToDeletion)}`
        : `до срока ${formatCountdown(expires - now)}`;

  return (
    <div className={classes.root} data-phase={phase}>
      <div className={`${classes.track} ${phase === 'closed' ? classes.closed : ''}`} aria-hidden>
        <span className={classes.lead} />
        <span className={classes.wait} data-tone={tone === 'critical' ? 'critical' : undefined} />
        {phase !== 'closed' ? (
          <span className={classes.marker} style={{ left: `${position * 100}%` }} />
        ) : null}
      </div>
      <div className={classes.scale} aria-hidden>
        <span>срок</span>
        <span>
          <span>{phase === 'wait' ? 'сейчас · 5 дней ожидания' : '5 дней ожидания'}</span>
          <span>удаление</span>
        </span>
      </div>
      <span className={classes.caption} data-tone={tone ?? undefined}>
        {caption}
      </span>
    </div>
  );
}
