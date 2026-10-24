import { HttpResponse, http } from 'msw';
import type { components } from '@shared/api';

type Task = components['schemas']['RetentionTask'];
type Brief = components['schemas']['TaskBrief'];
type Event = components['schemas']['TaskEvent'];

const API = '/api/v1';
const SESSION_KEY = 'mock-session';
const roles: Record<string, string> = { marketer: 'marketer', analyst: 'analyst', jury: 'jury' };

let tasks: Task[] = [];
let nextId = 1;
let nextEvent = 1;

/** Задачи моков живут в памяти: между тестами сбрасываются через `resetTasks`. */
export function resetTasks(): void {
  tasks = [];
  nextId = 1;
  nextEvent = 1;
}

/** Клиенты, у которых «чужая правка» случится при первой записи: для тестов 409. */
export const conflictOn = new Set<number>();

function login(): string | null {
  return sessionStorage.getItem(SESSION_KEY);
}

function problem(status: number, title: string, extra: object = {}) {
  return HttpResponse.json(
    { type: 'about:blank', title, status, ...extra },
    { status, headers: { 'Content-Type': 'application/problem+json' } },
  );
}

/** Завтра 18:00 по часам браузера: у моков нет часов стенда. */
function tomorrow(): string {
  const d = new Date();
  d.setDate(d.getDate() + 1);
  d.setHours(18, 0, 0, 0);
  return d.toISOString();
}

const isOpen = (t: Task) => t.status !== 'closed';

export function openTaskBrief(customerId: number): Brief | undefined {
  const t = tasks.find((x) => x.customerId === customerId && isOpen(x));
  return (
    t && { id: t.id, status: t.status, assignee: t.assignee, dueAt: t.dueAt, overdue: t.overdue }
  );
}

/** Кладёт в мок готовую задачу, например просроченную. */
export function seedTask(customerId: number, patch: Partial<Task> = {}): Task {
  const at = new Date().toISOString();
  const task: Task = {
    id: nextId++,
    customerId,
    status: 'in_work',
    assignee: 'marketer',
    dueAt: tomorrow(),
    overdue: false,
    note: '',
    version: 1,
    customerMissing: false,
    baseline: { version: 1, level: 'high', expectedLossCents: 0, balance: 0, services: [] },
    createdBy: 'marketer',
    createdAt: at,
    updatedAt: at,
    updatedBy: 'marketer',
    events: [{ id: nextEvent++, at, actor: 'marketer', kind: 'created', text: '', changes: [] }],
    ...patch,
  };
  tasks.push(task);
  return task;
}

function find(id: unknown): Task | undefined {
  return tasks.find((t) => t.id === Number(id));
}

function guard(write: boolean) {
  const who = login();
  if (!who) return problem(401, 'Нет сессии');
  if (write && roles[who] === 'analyst') return problem(403, 'Недостаточно прав');
  return null;
}

/** Чужая правка: задачу «уже изменил» другой сотрудник — версия ушла вперёд. */
function conflict(task: Task) {
  return problem(409, 'Задача уже изменена', { task });
}

function touch(task: Task, text = '', kind: Event['kind'] = 'changed', channel?: Event['channel']) {
  // Как на сервере: контакт и заметка версию задачи не меняют.
  if (kind === 'changed') task.version += 1;
  task.updatedAt = new Date().toISOString();
  task.updatedBy = login() ?? 'marketer';
  task.events = [
    ...(task.events ?? []),
    {
      id: nextEvent++,
      at: task.updatedAt,
      actor: task.updatedBy,
      kind,
      channel,
      text,
      changes: [],
    },
  ];
}

export const taskHandlers = [
  http.get(`${API}/tasks`, ({ request }) => {
    const denied = guard(false);
    if (denied) return denied;
    const q = new URL(request.url).searchParams;
    const me = login();
    const now = Date.now();
    const items = tasks
      .filter((t) => !q.get('status') || t.status === q.get('status'))
      .filter(
        (t) =>
          !q.get('assignee') ||
          t.assignee === (q.get('assignee') === 'me' ? me : q.get('assignee')),
      )
      .filter(
        (t) => q.get('overdue') !== 'true' || (isOpen(t) && new Date(t.dueAt).getTime() < now),
      )
      .map((t) => ({ ...t, events: undefined }));
    return HttpResponse.json({ items, total: items.length });
  }),
  http.get(`${API}/tasks/assignees`, () => {
    const denied = guard(false);
    if (denied) return denied;
    return HttpResponse.json({
      items: [
        { login: 'marketer', role: 'marketer' },
        { login: 'jury', role: 'jury' },
      ],
    });
  }),
  http.get(`${API}/tasks/:taskId`, ({ params }) => {
    const denied = guard(false);
    if (denied) return denied;
    const task = find(params.taskId);
    return task ? HttpResponse.json(task) : problem(404, 'Задача не найдена');
  }),
  http.post(`${API}/customers/:id/task`, async ({ params, request }) => {
    const denied = guard(true);
    if (denied) return denied;
    const customerId = Number(params.id);
    const body = (await request.json().catch(() => ({}))) as Partial<Task>;
    const existing = tasks.find((t) => t.customerId === customerId && isOpen(t));
    if (existing) return conflict(existing);
    const task = seedTask(customerId, {
      assignee: body.assignee ?? login() ?? 'marketer',
      dueAt: body.dueAt ?? tomorrow(),
      note: body.note ?? '',
      createdBy: login() ?? 'marketer',
      updatedBy: login() ?? 'marketer',
    });
    return HttpResponse.json(task, { status: 201 });
  }),
  http.patch(`${API}/tasks/:taskId`, async ({ params, request }) => {
    const denied = guard(true);
    if (denied) return denied;
    const task = find(params.taskId);
    if (!task) return problem(404, 'Задача не найдена');
    const body = (await request.json()) as Partial<Task> & { version: number };
    if (conflictOn.has(task.customerId)) {
      conflictOn.delete(task.customerId);
      task.updatedBy = 'jury';
      touch(task);
      task.updatedBy = 'jury';
      return conflict(task);
    }
    if (body.version !== task.version) return conflict(task);
    if (body.status === 'closed' && !body.outcome) return problem(400, 'Для закрытия нужен итог');
    if (body.status === 'postponed' && !body.dueAt) return problem(400, 'Для отсрочки нужен срок');
    if (body.status) task.status = body.status;
    if (body.outcome) task.outcome = body.outcome;
    if (body.assignee) task.assignee = body.assignee;
    if (body.dueAt) task.dueAt = body.dueAt;
    if (body.note !== undefined) task.note = body.note;
    task.overdue = isOpen(task) && new Date(task.dueAt).getTime() < Date.now();
    touch(task);
    return HttpResponse.json(task);
  }),
  http.post(`${API}/tasks/:taskId/events`, async ({ params, request }) => {
    const denied = guard(true);
    if (denied) return denied;
    const task = find(params.taskId);
    if (!task) return problem(404, 'Задача не найдена');
    const body = (await request.json()) as {
      kind: Event['kind'];
      channel?: Event['channel'];
      text?: string;
    };
    if (body.kind === 'contact' && !body.channel) return problem(400, 'Нужен канал контакта');
    touch(task, body.text ?? '', body.kind, body.channel);
    return HttpResponse.json(task, { status: 201 });
  }),
];
