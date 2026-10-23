import type { ReactNode } from 'react';
import classes from './Changed.module.css';

/** Блок, который на время подсвечивается, если его содержимое изменилось после пересчёта. */
export function Changed({ active, children }: { active: boolean; children: ReactNode }) {
  return (
    <div className={classes.changed} data-changed={active}>
      {children}
    </div>
  );
}
