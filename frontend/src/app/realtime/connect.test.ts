import { QueryClient } from '@tanstack/react-query';
import { connectEvents, type EventSourceFactory } from './connect';

class FakeSource {
  readyState = 0;
  onopen: ((e: Event) => void) | null = null;
  onerror: ((e: Event) => void) | null = null;
  listeners = new Map<string, (e: Event) => void>();
  closed = false;
  addEventListener(type: string, fn: (e: Event) => void) {
    this.listeners.set(type, fn);
  }
  close() {
    this.closed = true;
  }
  emit(type: string, data: string) {
    this.listeners.get(type)?.(new MessageEvent(type, { data }));
  }
}

function setup() {
  const queryClient = new QueryClient();
  const spy = vi.spyOn(queryClient, 'invalidateQueries');
  const sources: FakeSource[] = [];
  const factory: EventSourceFactory = () => {
    const s = new FakeSource();
    sources.push(s);
    return s as never;
  };
  const states: unknown[] = [];
  const stop = connectEvents(queryClient, (s) => states.push(s), factory);
  return { spy, sources, states, stop };
}

const roots = (spy: ReturnType<typeof setup>['spy']) =>
  spy.mock.calls.map((c) => (c[0]?.queryKey as string[])[0]);

describe('connectEvents', () => {
  it('на recalculated сбрасывает ключи очереди, карточки, сводки, настроек и источника', () => {
    const { spy, sources, states } = setup();
    sources[0]!.onopen?.(new Event('open'));
    sources[0]!.emit('recalculated', '{"type":"recalculated","version":5}');
    expect(roots(spy)).toEqual(['customers', 'customer', 'summary', 'model-settings', 'source']);
    expect(states.at(-1)).toEqual({ status: 'online', version: 5 });
  });

  it('на tasks сбрасывает очередь, карточку, сводку и задачи', () => {
    const { spy, sources } = setup();
    sources[0]!.onopen?.(new Event('open'));
    sources[0]!.emit('tasks', '{"type":"changed","taskId":3,"customerId":9}');
    expect(roots(spy)).toEqual(['customers', 'customer', 'summary', 'task']);
  });

  it('на streamProgress кладёт кадр хода потока в кэш и не перезапрашивает данные', () => {
    const queryClient = new QueryClient();
    const spy = vi.spyOn(queryClient, 'invalidateQueries');
    const sources: FakeSource[] = [];
    connectEvents(
      queryClient,
      () => undefined,
      () => {
        const s = new FakeSource();
        sources.push(s);
        return s as never;
      },
    );
    const frame = { state: 'running', events: 2500, applied: 300 };
    sources[0]!.emit('streamProgress', JSON.stringify(frame));
    expect(queryClient.getQueryData(['stream'])).toEqual(frame);
    sources[0]!.emit('streamProgress', 'не JSON');
    expect(queryClient.getQueryData(['stream'])).toEqual(frame);
    expect(spy).not.toHaveBeenCalled();
  });

  it('обрыв даёт offline, возврат связи — один перезапрос', () => {
    const { spy, sources, states } = setup();
    sources[0]!.onopen?.(new Event('open'));
    sources[0]!.onerror?.(new Event('error'));
    expect(states.at(-1)).toMatchObject({ status: 'offline' });
    expect(spy).not.toHaveBeenCalled();
    sources[0]!.onopen?.(new Event('open'));
    expect(states.at(-1)).toMatchObject({ status: 'online' });
    expect(spy).toHaveBeenCalledTimes(5);
    sources[0]!.onopen?.(new Event('open'));
    expect(spy).toHaveBeenCalledTimes(5);
  });

  it('закрытый поток поднимается заново через паузу', () => {
    vi.useFakeTimers();
    const { sources } = setup();
    sources[0]!.readyState = 2;
    sources[0]!.onerror?.(new Event('error'));
    expect(sources[0]!.closed).toBe(true);
    vi.advanceTimersByTime(3000);
    expect(sources).toHaveLength(2);
    vi.useRealTimers();
  });

  it('отписка закрывает поток', () => {
    const { sources, stop } = setup();
    stop();
    expect(sources[0]!.closed).toBe(true);
  });
});
