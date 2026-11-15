import { memo } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import { colors } from '../theme';
import { STATUS_LABEL, type Task } from '../types';

interface Props {
  task: Task;
  onPress: (task: Task) => void;
  onDelete: (task: Task) => void;
}

export const TaskItem = memo(function TaskItemCard({ task, onPress, onDelete }: Props) {
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={`Edit task ${task.title}`}
      onPress={() => onPress(task)}
      style={({ pressed }) => [styles.card, pressed && styles.pressed]}
    >
      <View style={styles.header}>
        <Text style={styles.title} numberOfLines={2}>
          {task.title}
        </Text>
        <View style={[styles.badge, { backgroundColor: colors.status[task.status] }]}>
          <Text style={styles.badgeText}>{STATUS_LABEL[task.status]}</Text>
        </View>
      </View>
      {task.description ? (
        <Text style={styles.desc} numberOfLines={2}>
          {task.description}
        </Text>
      ) : null}
      <View style={styles.footer}>
        <Text style={styles.meta}>
          {task.assignee ? `@${task.assignee}` : 'Unassigned'}
          {task.due_date ? ` · due ${task.due_date}` : ''}
        </Text>
        <Pressable
          accessibilityRole="button"
          accessibilityLabel={`Delete task ${task.title}`}
          onPress={() => onDelete(task)}
          hitSlop={8}
        >
          <Text style={styles.delete}>Delete</Text>
        </Pressable>
      </View>
    </Pressable>
  );
});

const styles = StyleSheet.create({
  card: {
    backgroundColor: colors.surface,
    borderRadius: 12,
    padding: 14,
    borderWidth: 1,
    borderColor: colors.border,
    gap: 6,
  },
  pressed: { opacity: 0.7 },
  header: { flexDirection: 'row', alignItems: 'flex-start', gap: 8 },
  title: { flex: 1, fontSize: 16, fontWeight: '600', color: colors.text },
  badge: { borderRadius: 999, paddingHorizontal: 8, paddingVertical: 2 },
  badgeText: { color: '#fff', fontSize: 12, fontWeight: '600' },
  desc: { color: colors.muted, fontSize: 14 },
  footer: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' },
  meta: { color: colors.muted, fontSize: 13 },
  delete: { color: colors.danger, fontSize: 13, fontWeight: '600' },
});
