import { Pressable, StyleSheet, Text, TextInput, View } from 'react-native';

import { colors } from '../theme';

interface Props {
  value: string;
  onChangeText: (text: string) => void;
}

/** Controlled input; debouncing happens in the screen so the input stays responsive. */
export function SearchBar({ value, onChangeText }: Props) {
  return (
    <View style={styles.wrap}>
      <TextInput
        accessibilityLabel="Search tasks"
        placeholder="Search title or description…"
        placeholderTextColor={colors.muted}
        value={value}
        onChangeText={onChangeText}
        autoCapitalize="none"
        autoCorrect={false}
        returnKeyType="search"
        style={styles.input}
      />
      {value.length > 0 && (
        <Pressable accessibilityRole="button" accessibilityLabel="Clear search" onPress={() => onChangeText('')} hitSlop={8}>
          <Text style={styles.clear}>✕</Text>
        </Pressable>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: {
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: colors.surface,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 10,
    paddingHorizontal: 12,
  },
  input: { flex: 1, paddingVertical: 10, fontSize: 16, color: colors.text },
  clear: { color: colors.muted, fontSize: 16, paddingLeft: 8 },
});
