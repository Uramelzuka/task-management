import { fireEvent, render, screen } from '@testing-library/react-native';

import { ApiError } from '../../api/client';
import type { Task } from '../../types';
import { EditTaskModal } from '../EditTaskModal';

const task: Task = {
  id: 7,
  title: 'Write docs',
  description: 'first draft',
  status: 'todo',
  assignee: 'alice',
  due_date: '2026-12-01',
  created_at: '2026-11-01T00:00:00Z',
  updated_at: '2026-11-01T00:00:00Z',
};

describe('EditTaskModal', () => {
  it('prefills the form from the task being edited', async () => {
    await render(<EditTaskModal visible task={task} onSubmit={jest.fn()} onClose={jest.fn()} />);

    expect(screen.getByText('Edit task')).toBeOnTheScreen();
    expect(screen.getByLabelText('Title')).toHaveDisplayValue('Write docs');
    expect(screen.getByLabelText('Assignee')).toHaveDisplayValue('alice');
    expect(screen.getByLabelText('Status To do')).toBeSelected();
  });

  it('submits trimmed values and turns blank optional fields into null', async () => {
    const onSubmit = jest.fn().mockResolvedValue(undefined);
    await render(<EditTaskModal visible task={task} onSubmit={onSubmit} onClose={jest.fn()} />);

    await fireEvent.changeText(screen.getByLabelText('Title'), '  Write README  ');
    await fireEvent.changeText(screen.getByLabelText('Assignee'), '   ');
    await fireEvent.changeText(screen.getByLabelText('Due date'), '');
    await fireEvent.press(screen.getByLabelText('Status Done'));
    await fireEvent.press(screen.getByLabelText('Save'));

    expect(onSubmit).toHaveBeenCalledWith({
      title: 'Write README',
      description: 'first draft',
      status: 'done',
      assignee: null,
      due_date: null,
    });
  });

  it('blocks submit and shows an error when the title is empty', async () => {
    const onSubmit = jest.fn();
    await render(<EditTaskModal visible task={task} onSubmit={onSubmit} onClose={jest.fn()} />);

    await fireEvent.changeText(screen.getByLabelText('Title'), '   ');
    await fireEvent.press(screen.getByLabelText('Save'));

    expect(screen.getByRole('alert')).toHaveTextContent('Title is required');
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it('rejects a malformed due date', async () => {
    const onSubmit = jest.fn();
    await render(<EditTaskModal visible task={task} onSubmit={onSubmit} onClose={jest.fn()} />);

    await fireEvent.changeText(screen.getByLabelText('Due date'), '01/12/2026');
    await fireEvent.press(screen.getByLabelText('Save'));

    expect(screen.getByRole('alert')).toHaveTextContent('Due date must be YYYY-MM-DD');
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it('shows the API message (e.g. 409 duplicate title) and stays open', async () => {
    const onSubmit = jest
      .fn()
      .mockRejectedValue(new ApiError(409, 'DUPLICATE_TITLE', 'a task with this title already exists'));
    await render(<EditTaskModal visible task={task} onSubmit={onSubmit} onClose={jest.fn()} />);

    await fireEvent.press(screen.getByLabelText('Save'));

    expect(await screen.findByRole('alert')).toHaveTextContent('a task with this title already exists');
    expect(screen.getByLabelText('Save')).toBeEnabled();
  });

  it('shows a loading state while saving', async () => {
    let resolve!: () => void;
    const onSubmit = jest.fn(() => new Promise<void>((r) => (resolve = r)));
    await render(<EditTaskModal visible task={task} onSubmit={onSubmit} onClose={jest.fn()} />);

    // fireEvent awaits the handler, which is blocked on onSubmit, so don't await it yet.
    const pressing = fireEvent.press(screen.getByLabelText('Save'));

    expect(await screen.findByTestId('saving-indicator')).toBeOnTheScreen();
    expect(screen.getByLabelText('Save')).toBeDisabled();
    resolve();
    await pressing;
  });

  it('opens empty in create mode', async () => {
    await render(<EditTaskModal visible task={null} onSubmit={jest.fn()} onClose={jest.fn()} />);
    expect(screen.getByText('New task')).toBeOnTheScreen();
    expect(screen.getByLabelText('Title')).toHaveDisplayValue('');
  });
});
