import { createFileRoute } from '@tanstack/react-router';
import { customerQuery } from '@entities/customer-detail';
import { CustomerPage } from '@pages/customer';
import { RightGate } from '../layout/RightGate';

// `queue_` — карточка не вложена в экран очереди, адрес тот же: /queue/$id.
export const Route = createFileRoute('/_app/queue_/$id')({
  // Прогрев кэша: карточка грузится, пока рисуется оболочка.
  loader: ({ context, params }) => {
    const id = Number(params.id);
    if (Number.isInteger(id)) void context.queryClient.prefetchQuery(customerQuery(id));
  },
  component: Section,
});

function Section() {
  const { user } = Route.useRouteContext();
  const { id } = Route.useParams();
  return (
    <RightGate role={user.role} right="customer.view" section="Карточка клиента">
      <CustomerPage id={Number(id)} />
    </RightGate>
  );
}
