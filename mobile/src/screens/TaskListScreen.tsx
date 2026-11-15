import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Alert, FlatList, Pressable, StyleSheet, Text, View } from 'react-native';

import { ApiError } from '../api/client';
import { tasksApi } from '../api/tasks';
import { EditTaskModal } from '../components/EditTaskModal';
import { Pagination } from '../components/Pagination';
import { SearchBar } from '../components/SearchBar';
import { StatusFilter } from '../components/StatusFilter';
import { TaskItem } from '../components/TaskItem';
import { useDebounce } from '../hooks/useDebounce';
import { useTasks } from '../hooks/useTasks';
import { colors } from '../theme';
import type { Task, TaskInput, TaskStatus } from '../types';

export const PAGE_SIZE = 10;
export const SEARCH_DEBOUNCE_MS = 400;

const Separator = () => <View style={styles.separator} />;

type Editing ={ mode: 'closed' } | { mode: 'create' } | { mode: 'edit'; task: Task };

function confirmDelete(title: string): Promise<boolean> {
  return new Promise((resolve) =>
    Alert.alert(
      'Delete task',
      `Delete "${title}"?`,
      [
        { text: 'Cancel', style: 'cancel', onPress: () => resolve(false) },
        { text: 'Delete', style: 'destructive', onPress: () => resolve(true) },
      ],
      // Android: tapping outside the dialog dismisses it without a button press.
      { cancelable: true, onDismiss: () => resolve(false) },
    ),
  );
}

export function TaskListScreen() {
  const [search, setSearch] = useState('');
  const [status, setStatus] = useState<TaskStatus | undefined>();
  const [page, setPage] = useState(1);
  const [editing, setEditing] = useState<Editing>({ mode: 'closed' });
  const [refreshing, setRefreshing] = useState(false);

  const keyword = useDebounce(search.trim(), SEARCH_DEBOUNCE_MS);

  // A new search or filter always starts from page 1.
  useEffect(() => setPage(1), [keyword, status]);

  const { tasks, meta, loading, error, reload } = useTasks({
    keyword,
    status,
    page,
    limit: PAGE_SIZE,
    sort: '-created_at',
  });

  useEffect(() => {
    if (!loading) setRefreshing(false);
  }, [loading]);

  // If a delete empties the last page, step back instead of showing nothing.
  useEffect(() => {
    if (meta && meta.total_pages > 0 && page > meta.total_pages) setPage(meta.total_pages);
  }, [meta, page]);

  const handleSubmit = useCallback(
    async (input: TaskInput) => {
      if (editing.mode === 'edit') {
        await tasksApi.update(editing.task.id, input);
      } else {
        await tasksApi.create(input);
      }
      setEditing({ mode: 'closed' });
      // Refetch so the list reflects the change (sort order, filters, counts).
      // The backend has already bumped its cache version, so this is fresh.
      reload();
    },
    [editing, reload],
  );

  const handleDelete = useCallback(
    async (task: Task) => {
      if (!(await confirmDelete(task.title))) return;
      try {
        await tasksApi.remove(task.id);
      } catch (e) {
        // 404 means it is already gone; refreshing is the right outcome either way.
        if (!(e instanceof ApiError && e.status === 404)) {
          Alert.alert('Delete failed', e instanceof Error ? e.message : 'Unknown error');
        }
      }
      reload();
    },
    [reload],
  );

  const openEdit = useCallback((task: Task) => setEditing({ mode: 'edit', task }), []);

  const initialLoading = loading && !meta;
  const isSearching = search.trim() !== keyword;

  return (
    <View style={styles.screen}>
      <View style={styles.header}>
        <Text style={styles.h1} accessibilityRole="header">
          Tasks
        </Text>
        <Pressable
          accessibilityRole="button"
          accessibilityLabel="New task"
          onPress={() => setEditing({ mode: 'create' })}
          style={styles.newBtn}
        >
          <Text style={styles.newBtnText}>+ New</Text>
        </Pressable>
      </View>

      <View style={styles.controls}>
        <SearchBar value={search} onChangeText={setSearch} />
        <StatusFilter value={status} onChange={setStatus} />
      </View>

      {error ? (
        <View style={styles.errorBox} accessibilityRole="alert">
          <Text style={styles.errorText}>{error}</Text>
          <Pressable accessibilityRole="button" onPress={reload}>
            <Text style={styles.retry}>Retry</Text>
          </Pressable>
        </View>
      ) : null}

      {initialLoading ? (
        <View style={styles.center}>
          <ActivityIndicator size="large" color={colors.primary} testID="list-loading" />
        </View>
      ) : (
        <FlatList
          data={tasks}
          keyExtractor={(t) => String(t.id)}
          renderItem={({ item }) => <TaskItem task={item} onPress={openEdit} onDelete={handleDelete} />}
          contentContainerStyle={styles.list}
          ItemSeparatorComponent={Separator}
          refreshing={refreshing}
          onRefresh={() => {
            setRefreshing(true);
            reload();
          }}
          ListHeaderComponent={
            (loading || isSearching) && !refreshing ? (
              <ActivityIndicator style={styles.inlineLoading} color={colors.primary} testID="list-refreshing" />
            ) : undefined
          }
          ListEmptyComponent={
            !loading && !error ? (
              <Text style={styles.empty}>
                {keyword || status ? 'No tasks match your filters.' : 'No tasks yet. Tap “+ New” to add one.'}
              </Text>
            ) : undefined
          }
        />
      )}

      {meta && meta.total > 0 ? (
        <View style={styles.footer}>
          <Pagination
            page={meta.page}
            totalPages={meta.total_pages}
            total={meta.total}
            disabled={loading}
            onChange={setPage}
          />
        </View>
      ) : null}

      <EditTaskModal
        visible={editing.mode !== 'closed'}
        task={editing.mode === 'edit' ? editing.task : null}
        onSubmit={handleSubmit}
        onClose={() => setEditing({ mode: 'closed' })}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  screen: { flex: 1, backgroundColor: colors.background },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingHorizontal: 16,
    paddingTop: 12,
  },
  h1: { fontSize: 28, fontWeight: '700', color: colors.text },
  newBtn: { backgroundColor: colors.primary, paddingHorizontal: 14, paddingVertical: 8, borderRadius: 8 },
  newBtnText: { color: colors.onPrimary, fontWeight: '600' },
  controls: { paddingHorizontal: 16, paddingTop: 12, gap: 10 },
  list: { padding: 16, flexGrow: 1 },
  separator: { height: 10 },
  center: { flex: 1, alignItems: 'center', justifyContent: 'center' },
  inlineLoading: { marginBottom: 10 },
  empty: { textAlign: 'center', color: colors.muted, marginTop: 40, fontSize: 15 },
  footer: { paddingHorizontal: 16, borderTopWidth: 1, borderTopColor: colors.border, backgroundColor: colors.surface },
  errorBox: {
    marginHorizontal: 16,
    marginTop: 10,
    padding: 12,
    borderRadius: 8,
    backgroundColor: '#fee2e2',
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    gap: 8,
  },
  errorText: { color: colors.danger, flex: 1 },
  retry: { color: colors.primary, fontWeight: '600' },
});
