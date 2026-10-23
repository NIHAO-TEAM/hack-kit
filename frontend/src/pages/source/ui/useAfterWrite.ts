import { notifications } from '@mantine/notifications';
import { useQueryClient } from '@tanstack/react-query';
import { ApiError } from '@shared/api';

/** После записи данных обновляем источник, сводку и очередь; ошибку показываем человеческим текстом. */
export function useAfterWrite() {
  const queryClient = useQueryClient();
  return {
    done(message: string) {
      for (const root of ['source', 'summary', 'customers', 'customer', 'audit']) {
        void queryClient.invalidateQueries({ queryKey: [root] });
      }
      notifications.show({ message, color: 'teal' });
    },
    fail(title: string, e: unknown) {
      notifications.show({
        title,
        message: e instanceof ApiError ? e.message : 'Ошибка запроса',
        color: 'red',
      });
    },
  };
}
