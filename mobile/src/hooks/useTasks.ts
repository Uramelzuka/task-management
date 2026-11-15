import { useCallback, useEffect, useRef, useState } from 'react';

import { tasksApi } from '../api/tasks';
import type { ListQuery, PageMeta, Task } from '../types';

interface State {
  tasks: Task[];
  meta: PageMeta | null;
  loading: boolean;
  error: string | null;
}

/**
 * Loads one page of tasks for `query`. Re-fetches whenever the query changes
 * and exposes `reload()` so mutations can refresh the list (this was the
 * "list not refreshed after update" bug).
 *
 * Out-of-order responses are discarded: each request is aborted when a newer
 * one starts, so fast typing can never show results for an older keyword.
 */
export function useTasks(query: ListQuery) {
  const [state, setState] = useState<State>({ tasks: [], meta: null, loading: true, error: null });
  const [reloadToken, setReloadToken] = useState(0);
  const controllerRef = useRef<AbortController | null>(null);

  const { keyword, status, assignee, page, limit, sort } = query;

  useEffect(() => {
    controllerRef.current?.abort();
    const controller = new AbortController();
    controllerRef.current = controller;

    setState((s) => ({ ...s, loading: true, error: null }));
    tasksApi
      .list({ keyword, status, assignee, page, limit, sort }, controller.signal)
      .then((res) => {
        if (controller.signal.aborted) return;
        setState({ tasks: res.data, meta: res.meta, loading: false, error: null });
      })
      .catch((e: Error) => {
        if (controller.signal.aborted || e.name === 'AbortError') return;
        setState((s) => ({ ...s, loading: false, error: e.message }));
      });

    return () => controller.abort();
  }, [keyword, status, assignee, page, limit, sort, reloadToken]);

  const reload = useCallback(() => setReloadToken((n) => n + 1), []);

  return { ...state, reload };
}
