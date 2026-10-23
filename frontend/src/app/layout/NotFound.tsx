import { Button } from '@mantine/core';
import { Link } from '@tanstack/react-router';
import { StateEmpty } from '@shared/ui';

export function NotFound() {
  return (
    <StateEmpty
      title="Страница не найдена"
      action={
        <Button component={Link} to="/">
          На главную
        </Button>
      }
    >
      Проверьте адрес или вернитесь на главную.
    </StateEmpty>
  );
}
