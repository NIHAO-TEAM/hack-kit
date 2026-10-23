import { Alert } from '@mantine/core';
import { useQueryClient } from '@tanstack/react-query';
import { useEffect, useState, type ReactNode } from 'react';
import { RealtimeContext, type RealtimeState } from '@shared/lib';
import { IconWifiOff } from '@shared/ui';
import { connectEvents } from './connect';

/** Держит поток событий пока открыта оболочка; при обрыве показывает плашку над содержимым. */
export function RealtimeProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();
  const [state, setState] = useState<RealtimeState>({ status: 'connecting', version: null });

  useEffect(() => {
    // Нет EventSource (старый браузер, тесты) — работаем без живого обновления.
    if (typeof EventSource === 'undefined') return;
    return connectEvents(queryClient, setState);
  }, [queryClient]);

  return (
    <RealtimeContext.Provider value={state}>
      {state.status === 'offline' ? (
        <Alert icon={<IconWifiOff size={18} aria-hidden />} color="orange" role="alert" mb="md">
          Нет связи, данные могли устареть
        </Alert>
      ) : null}
      {children}
    </RealtimeContext.Provider>
  );
}
