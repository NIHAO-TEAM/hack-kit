import { createFileRoute, redirect } from '@tanstack/react-router';

/** Экрана «Задач» нет (ADR-0004): старые ссылки ведут на очередь. */
export const Route = createFileRoute('/_app/tasks')({
  beforeLoad: () => {
    throw redirect({ to: '/queue', replace: true });
  },
});
