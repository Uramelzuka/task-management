import { act, fireEvent, render, screen, waitFor } from '@testing-library/react-native';

import { tasksApi } from '../../api/tasks';
import type { Task, TaskPage } from '../../types';
import { SEARCH_DEBOUNCE_MS, TaskListScreen } from '../TaskListScreen';

jest.mock('../../api/tasks', () => ({
  tasksApi: { list: jest.fn(), create: jest.fn(), update: jest.fn(), remove: jest.fn() },
}));
const api = tasksApi as jest.Mocked<typeof tasksApi>;

const mkTask = (id: number, title: string, status: Task['status'] = 'todo'): Task => ({
  id,
  title,
  description: null,
  status,
  assignee: null,
  due_date: null,
  created_at: '2026-11-01T00:00:00Z',
  updated_at: '2026-11-01T00:00:00Z',
});

const page = (data: Task[], p = 1, total = data.length, limit = 10): TaskPage => ({
  data,
  meta: { page: p, limit, total, total_pages: Math.ceil(total / limit) },
});

beforeEach(() => {
  jest.clearAllMocks();
});

describe('TaskListScreen', () => {
  it('shows a loading indicator, then the tasks', async () => {
    let resolve!: (p: TaskPage) => void;
    api.list.mockReturnValueOnce(new Promise((r) => (resolve = r)));
    await render(<TaskListScreen />);

    expect(screen.getByTestId('list-loading')).toBeOnTheScreen();
    await act(async () => resolve(page([mkTask(1, 'First')])));
    expect(screen.getByText('First')).toBeOnTheScreen();
    expect(screen.queryByTestId('list-loading')).not.toBeOnTheScreen();
  });

  it('debounces search: one request after typing stops, with page reset to 1', async () => {
    jest.useFakeTimers();
    try {
      api.list.mockResolvedValue(page([mkTask(1, 'First')]));
      await render(<TaskListScreen />);
      await waitFor(() => expect(api.list).toHaveBeenCalledTimes(1));

      await fireEvent.changeText(screen.getByLabelText('Search tasks'), 'w');
      await fireEvent.changeText(screen.getByLabelText('Search tasks'), 'wr');
      await fireEvent.changeText(screen.getByLabelText('Search tasks'), 'write');
      await act(async () => jest.advanceTimersByTime(SEARCH_DEBOUNCE_MS - 50));
      expect(api.list).toHaveBeenCalledTimes(1);

      await act(async () => jest.advanceTimersByTime(100));
      expect(api.list).toHaveBeenCalledTimes(2);
      expect(api.list).toHaveBeenLastCalledWith(
        expect.objectContaining({ keyword: 'write', page: 1 }),
        expect.anything(),
      );
    } finally {
      jest.useRealTimers();
    }
  });

  it('filters by status', async () => {
    api.list.mockResolvedValue(page([mkTask(1, 'First')]));
    await render(<TaskListScreen />);
    await screen.findByText('First');

    await fireEvent.press(screen.getByLabelText('Status Done'));

    await waitFor(() =>
      expect(api.list).toHaveBeenLastCalledWith(expect.objectContaining({ status: 'done', page: 1 }), expect.anything()),
    );
  });

  it('paginates', async () => {
    api.list.mockResolvedValueOnce(page([mkTask(1, 'First')], 1, 15));
    api.list.mockResolvedValueOnce(page([mkTask(11, 'Eleventh')], 2, 15));
    await render(<TaskListScreen />);
    await screen.findByText('First');
    expect(screen.getByLabelText('Prev page')).toBeDisabled();

    await fireEvent.press(screen.getByLabelText('Next page'));

    expect(await screen.findByText('Eleventh')).toBeOnTheScreen();
    expect(api.list).toHaveBeenLastCalledWith(expect.objectContaining({ page: 2 }), expect.anything());
    expect(screen.getByLabelText('Next page')).toBeDisabled();
  });

  it('refreshes the list after a task is updated (regression)', async () => {
    api.list.mockResolvedValueOnce(page([mkTask(1, 'Old title')]));
    api.list.mockResolvedValueOnce(page([mkTask(1, 'New title', 'done')]));
    api.update.mockResolvedValue(mkTask(1, 'New title', 'done'));
    await render(<TaskListScreen />);

    await fireEvent.press(await screen.findByLabelText('Edit task Old title'));
    await fireEvent.changeText(screen.getByLabelText('Title'), 'New title');
    await fireEvent.press(screen.getByLabelText('Save'));

    expect(api.update).toHaveBeenCalledWith(1, expect.objectContaining({ title: 'New title' }));
    expect(await screen.findByText('New title')).toBeOnTheScreen();
    expect(screen.queryByText('Old title')).not.toBeOnTheScreen();
    expect(api.list).toHaveBeenCalledTimes(2);
  });

  it('shows an error with retry when loading fails', async () => {
    api.list.mockRejectedValueOnce(new Error('Cannot reach the server'));
    api.list.mockResolvedValueOnce(page([mkTask(1, 'Recovered')]));
    await render(<TaskListScreen />);

    expect(await screen.findByText('Cannot reach the server')).toBeOnTheScreen();
    await fireEvent.press(screen.getByText('Retry'));
    expect(await screen.findByText('Recovered')).toBeOnTheScreen();
  });
});
