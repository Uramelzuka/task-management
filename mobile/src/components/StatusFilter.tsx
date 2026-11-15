import { Pressable, ScrollView, StyleSheet, Text } from 'react-native';

import { STATUS_LABEL, STATUSES, type TaskStatus } from '../types';
import { colors } from '../theme';

interface Props {
  value: TaskStatus | undefined;
  onChange: (value: TaskStatus | undefined) => void;
  /** Hide the "All" chip, e.g. when choosing a status inside a form. */
  allowAll?: boolean;
}

export function StatusFilter({ value, onChange, allowAll = true }: Props) {
  const options: (TaskStatus | undefined)[] = allowAll ? [undefined, ...STATUSES] : STATUSES;
  return (
    <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={styles.row}>
      {options.map((s) => {
        const selected = s === value;
        const label = s ? STATUS_LABEL[s] : 'All';
        return (
          <Pressable
            key={s ?? 'all'}
            accessibilityRole="button"
            accessibilityState={{ selected }}
            accessibilityLabel={`Status ${label}`}
            onPress={() => onChange(s)}
            style={[styles.chip, selected && styles.chipSelected]}
          >
            <Text style={[styles.chipText, selected && styles.chipTextSelected]}>{label}</Text>
          </Pressable>
        );
      })}
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  row: { gap: 8, paddingVertical: 4 },
  chip: {
    paddingHorizontal: 14,
    paddingVertical: 7,
    borderRadius: 999,
    borderWidth: 1,
    borderColor: colors.border,
    backgroundColor: colors.surface,
  },
  chipSelected: { backgroundColor: colors.primary, borderColor: colors.primary },
  chipText: { color: colors.text, fontSize: 14 },
  chipTextSelected: { color: colors.onPrimary, fontWeight: '600' },
});
