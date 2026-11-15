import type { ListQuery, Task, TaskInput, TaskPage } from '../types';
import { request } from './client';

export function buildListQuery(q: ListQuery): string {
  const params = new URLSearchParams();
  const keyword = q.keyword?.trim();
  if (keyword) params.set('keyword', keyword);
  if (q.status) params.set('status', q.status);
  if (q.assignee?.trim()) params.set('assignee', q.assignee.trim());
  params.set('page', String(q.page));
  params.set('limit', String(q.limit));
  if (q.sort) params.set('sort', q.sort);
  return params.toString();
}

export const tasksApi = {
  list: (q: ListQuery, signal?: AbortSignal) =>
    request<TaskPage>(`/api/tasks?${buildListQuery(q)}`, { signal }),

  create: (input: TaskInput) =>
    request<{ data: Task }>('/api/tasks', { method: 'POST', body: JSON.stringify(input) }).then((r) => r.data),

  update: (id: number, input: TaskInput) =>
    request<{ data: Task }>(`/api/tasks/${id}`, { method: 'PUT', body: JSON.stringify(input) }).then(
      (r) => r.data,
    ),

  remove: (id: number) => request<void>(`/api/tasks/${id}`, { method: 'DELETE' }),
};
