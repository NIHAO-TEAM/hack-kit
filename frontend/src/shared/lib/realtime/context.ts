import { createContext, useContext } from 'react';

/** Состояние потока событий: связь есть, потеряна, ещё не установлена. */
export type RealtimeStatus = 'connecting' | 'online' | 'offline';

export interface RealtimeState {
  status: RealtimeStatus;
  /** Номер версии последнего пересчёта; null, пока событий не было. */
  version: number | null;
}

export const RealtimeContext = createContext<RealtimeState>({
  status: 'connecting',
  version: null,
});

export function useRealtime(): RealtimeState {
  return useContext(RealtimeContext);
}
