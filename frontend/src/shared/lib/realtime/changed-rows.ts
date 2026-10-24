import { useEffect, useRef, useState } from 'react';

export const HIGHLIGHT_MS = 2000;

export interface RowSignature {
  id: string | number;
  /** Всё, что видит пользователь в строке: изменилось — строка подсветится. */
  signature: string;
}

/** ID строк, которые изменились при смене version; живут HIGHLIGHT_MS. */
export function useChangedRows(
  items: readonly RowSignature[] | undefined,
  version: number | string | null,
  /** Только строки, что были на экране раньше: новые на странице (другой фильтр) не подсвечиваются. */
  existingOnly = false,
): ReadonlySet<string | number> {
  const previous = useRef<Map<string | number, string> | null>(null);
  const seenVersion = useRef<number | string | null>(version);
  const [changed, setChanged] = useState<ReadonlySet<string | number>>(new Set());

  useEffect(() => {
    if (!items) return;
    const next = new Map(items.map((i) => [i.id, i.signature]));
    const before = previous.current;
    previous.current = next;
    // Первая загрузка и обновления без нового пересчёта не подсвечиваем.
    if (!before || version === seenVersion.current) return;
    seenVersion.current = version;
    const ids = items
      .filter((i) => (!existingOnly || before.has(i.id)) && before.get(i.id) !== i.signature)
      .map((i) => i.id);
    if (ids.length === 0) return;
    setChanged(new Set(ids));
  }, [items, version]);

  useEffect(() => {
    if (changed.size === 0) return;
    const timer = setTimeout(() => setChanged(new Set()), HIGHLIGHT_MS);
    return () => clearTimeout(timer);
  }, [changed]);

  return changed;
}
