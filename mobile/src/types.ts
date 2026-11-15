export type TaskStatus = 'todo' | 'in_progress' | 'done';

export const STATUSES: TaskStatus[] = ['todo', 'in_progress', 'done'];

export const STATUS_LABEL: Record<TaskStatus, string> = {
  todo: 'To do',
  in_progress: 'In progress',
  done: 'Done',
};

export interface Task {
  id: number;
  title: string;
  description: string | null;
  status: TaskStatus;
  assignee: string | null;
  due_date: string | null; // YYYY-MM-DD
  created_at: string;
  updated_at: string;
}

export interface PageMeta {
  page: number;
  limit: number;
  total: number;
  total_pages: number;
}

export interface TaskPage {
  data: Task[];
  meta: PageMeta;
}

export interface TaskInput {
  title: string;
  description: string | null;
  status: TaskStatus;
  assignee: string | null;
  due_date: string | null;
}

export interface ListQuery {
  keyword?: string;
  status?: TaskStatus;
  assignee?: string;
  page: number;
  limit: number;
  sort?: string;
}
