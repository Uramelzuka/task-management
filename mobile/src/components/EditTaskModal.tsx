import { useEffect, useState } from 'react';
import {
  ActivityIndicator,
  KeyboardAvoidingView,
  Modal,
  Platform,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  View,
} from 'react-native';

import { ApiError } from '../api/client';
import { colors } from '../theme';
import type { Task, TaskInput, TaskStatus } from '../types';
import { StatusFilter } from './StatusFilter';

interface Props {
  visible: boolean;
  /** Task being edited; null means "create a new task". */
  task: Task | null;
  onSubmit: (input: TaskInput) => Promise<void>;
  onClose: () => void;
}

interface FormState {
  title: string;
  description: string;
  status: TaskStatus;
  assignee: string;
  dueDate: string;
}

const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;

function toForm(task: Task | null): FormState {
  return {
    title: task?.title ?? '',
    description: task?.description ?? '',
    status: task?.status ?? 'todo',
    assignee: task?.assignee ?? '',
    dueDate: task?.due_date ?? '',
  };
}

export function validate(form: FormState): string | null {
  if (!form.title.trim()) return 'Title is required';
  if (form.title.trim().length > 200) return 'Title must be at most 200 characters';
  const d = form.dueDate.trim();
  if (d && (!DATE_RE.test(d) || Number.isNaN(Date.parse(d)))) return 'Due date must be YYYY-MM-DD';
  return null;
}

const blankToNull = (s: string) => (s.trim() ? s.trim() : null);

export function EditTaskModal({ visible, task, onSubmit, onClose }: Props) {
  const [form, setForm] = useState<FormState>(() => toForm(task));
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  // Reset the form each time the modal opens for a (possibly different) task.
  useEffect(() => {
    if (visible) {
      setForm(toForm(task));
      setError(null);
      setSaving(false);
    }
  }, [visible, task]);

  const set = <K extends keyof FormState>(key: K) => (value: FormState[K]) =>
    setForm((f) => ({ ...f, [key]: value }));

  const handleSave = async () => {
    const msg = validate(form);
    if (msg) {
      setError(msg);
      return;
    }
    setSaving(true);
    setError(null);
    try {
      await onSubmit({
        title: form.title.trim(),
        description: blankToNull(form.description),
        status: form.status,
        assignee: blankToNull(form.assignee),
        due_date: blankToNull(form.dueDate),
      });
    } catch (e) {
      // 409 DUPLICATE_TITLE and other API errors are shown inline; the modal stays open.
      setError(e instanceof ApiError ? e.message : 'Something went wrong. Please try again.');
      setSaving(false);
    }
  };

  const isEdit = task !== null;

  return (
    <Modal visible={visible} animationType="slide" transparent onRequestClose={onClose}>
      <KeyboardAvoidingView behavior={Platform.OS === 'ios' ? 'padding' : undefined} style={styles.backdrop}>
        <View style={styles.sheet}>
          <Text style={styles.heading} accessibilityRole="header">
            {isEdit ? 'Edit task' : 'New task'}
          </Text>
          <ScrollView keyboardShouldPersistTaps="handled" contentContainerStyle={styles.body}>
            <Field label="Title">
              <TextInput
                accessibilityLabel="Title"
                value={form.title}
                onChangeText={set('title')}
                style={styles.input}
                maxLength={200}
                editable={!saving}
              />
            </Field>
            <Field label="Description">
              <TextInput
                accessibilityLabel="Description"
                value={form.description}
                onChangeText={set('description')}
                style={[styles.input, styles.multiline]}
                multiline
                editable={!saving}
              />
            </Field>
            <Field label="Status">
              <StatusFilter allowAll={false} value={form.status} onChange={(s) => s && set('status')(s)} />
            </Field>
            <Field label="Assignee">
              <TextInput
                accessibilityLabel="Assignee"
                value={form.assignee}
                onChangeText={set('assignee')}
                style={styles.input}
                autoCapitalize="none"
                editable={!saving}
              />
            </Field>
            <Field label="Due date (YYYY-MM-DD)">
              <TextInput
                accessibilityLabel="Due date"
                value={form.dueDate}
                onChangeText={set('dueDate')}
                style={styles.input}
                placeholder="2026-12-31"
                placeholderTextColor={colors.muted}
                editable={!saving}
              />
            </Field>
            {error ? (
              <Text style={styles.error} accessibilityRole="alert">
                {error}
              </Text>
            ) : null}
          </ScrollView>
          <View style={styles.actions}>
            <Pressable accessibilityRole="button" onPress={onClose} disabled={saving} style={styles.secondary}>
              <Text style={styles.secondaryText}>Cancel</Text>
            </Pressable>
            <Pressable
              accessibilityRole="button"
              accessibilityLabel="Save"
              accessibilityState={{ disabled: saving, busy: saving }}
              onPress={handleSave}
              disabled={saving}
              style={[styles.primary, saving && styles.primaryBusy]}
            >
              {saving ? (
                <ActivityIndicator color={colors.onPrimary} testID="saving-indicator" />
              ) : (
                <Text style={styles.primaryText}>Save</Text>
              )}
            </Pressable>
          </View>
        </View>
      </KeyboardAvoidingView>
    </Modal>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <View style={styles.field}>
      <Text style={styles.label}>{label}</Text>
      {children}
    </View>
  );
}

const styles = StyleSheet.create({
  backdrop: { flex: 1, justifyContent: 'flex-end', backgroundColor: 'rgba(0,0,0,0.4)' },
  sheet: {
    backgroundColor: colors.surface,
    borderTopLeftRadius: 16,
    borderTopRightRadius: 16,
    maxHeight: '90%',
    paddingTop: 16,
  },
  heading: { fontSize: 18, fontWeight: '700', color: colors.text, paddingHorizontal: 16 },
  body: { padding: 16, gap: 12 },
  field: { gap: 6 },
  label: { fontSize: 13, color: colors.muted, fontWeight: '600' },
  input: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    paddingHorizontal: 12,
    paddingVertical: 10,
    fontSize: 16,
    color: colors.text,
  },
  multiline: { minHeight: 80, textAlignVertical: 'top' },
  error: { color: colors.danger, fontSize: 14 },
  actions: {
    flexDirection: 'row',
    justifyContent: 'flex-end',
    gap: 12,
    padding: 16,
    borderTopWidth: 1,
    borderTopColor: colors.border,
  },
  secondary: { paddingHorizontal: 16, paddingVertical: 10 },
  secondaryText: { color: colors.text, fontSize: 16 },
  primary: {
    minWidth: 90,
    alignItems: 'center',
    paddingHorizontal: 16,
    paddingVertical: 10,
    borderRadius: 8,
    backgroundColor: colors.primary,
  },
  primaryBusy: { opacity: 0.7 },
  primaryText: { color: colors.onPrimary, fontSize: 16, fontWeight: '600' },
});
