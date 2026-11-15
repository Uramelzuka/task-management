import { Pressable, StyleSheet, Text, View } from 'react-native';

import { colors } from '../theme';

interface Props {
  page: number;
  totalPages: number;
  total: number;
  disabled?: boolean;
  onChange: (page: number) => void;
}

export function Pagination({ page, totalPages, total, disabled, onChange }: Props) {
  const canPrev = !disabled && page > 1;
  const canNext = !disabled && page < totalPages;
  return (
    <View style={styles.row}>
      <PageButton label="‹ Prev" enabled={canPrev} onPress={() => onChange(page - 1)} />
      <Text style={styles.info} accessibilityLabel={`Page ${page} of ${Math.max(totalPages, 1)}`}>
        {page} / {Math.max(totalPages, 1)} · {total} task{total === 1 ? '' : 's'}
      </Text>
      <PageButton label="Next ›" enabled={canNext} onPress={() => onChange(page + 1)} />
    </View>
  );
}

function PageButton({ label, enabled, onPress }: { label: string; enabled: boolean; onPress: () => void }) {
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={label.replace(/[‹›]/g, '').trim() + ' page'}
      accessibilityState={{ disabled: !enabled }}
      disabled={!enabled}
      onPress={onPress}
      style={[styles.btn, !enabled && styles.btnDisabled]}
    >
      <Text style={[styles.btnText, !enabled && styles.btnTextDisabled]}>{label}</Text>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', paddingVertical: 10 },
  info: { color: colors.muted, fontSize: 14 },
  btn: { paddingHorizontal: 14, paddingVertical: 8, borderRadius: 8, backgroundColor: colors.primary },
  btnDisabled: { backgroundColor: colors.border },
  btnText: { color: colors.onPrimary, fontWeight: '600' },
  btnTextDisabled: { color: colors.muted },
});
