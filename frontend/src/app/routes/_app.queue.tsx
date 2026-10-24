import { createFileRoute } from '@tanstack/react-router';
import type {
  CustomerOrder,
  CustomerParams,
  CustomerSort,
  RiskLevel,
  TaskFilter,
} from '@entities/customer';
import { QueuePage } from '@pages/queue';
import { RightGate } from '../layout/RightGate';

const sorts: CustomerSort[] = [
  'id',
  'email',
  'balance',
  'totalReplenished',
  'score',
  'expectedLoss',
];
const tasks: TaskFilter[] = ['none', 'open', 'mine'];
const levels: RiskLevel[] = ['high', 'medium', 'low', 'unscored', 'lost'];

/** Вид очереди — в адресе. Неизвестные значения отбрасываются, остаётся вид по умолчанию. */
function validateSearch(search: Record<string, unknown>): Partial<CustomerParams> {
  const sort = sorts.find((s) => s === search.sort);
  const order: CustomerOrder | undefined =
    search.order === 'asc' || search.order === 'desc' ? search.order : undefined;
  return {
    sort,
    order,
    level: levels.find((l) => l === search.level),
    task: tasks.find((t) => t === search.task),
    q: typeof search.q === 'string' && search.q ? search.q : undefined,
    // Первая страница в адрес не пишется.
    page:
      Number.isInteger(Number(search.page)) && Number(search.page) > 1
        ? Number(search.page)
        : undefined,
  };
}

export const Route = createFileRoute('/_app/queue')({
  validateSearch,
  component: Section,
});

function Section() {
  const { user } = Route.useRouteContext();
  const search = Route.useSearch();
  const params: CustomerParams = {
    ...search,
    sort: search.sort ?? 'expectedLoss',
    order: search.order ?? 'desc',
  };
  const navigate = Route.useNavigate();
  return (
    <RightGate role={user.role} right="queue.view" section="Очередь">
      <QueuePage
        params={params}
        role={user.role}
        onParams={(next) => void navigate({ search: next })}
      />
    </RightGate>
  );
}
