import { SegmentedControl, VisuallyHidden, useMantineColorScheme } from '@mantine/core';
import { IconDeviceDesktop, IconMoon, IconSun } from '@shared/ui';

type Scheme = 'light' | 'dark' | 'auto';

const options: { value: Scheme; label: string; icon: typeof IconSun }[] = [
  { value: 'dark', label: 'Тёмная', icon: IconMoon },
  { value: 'light', label: 'Светлая', icon: IconSun },
  { value: 'auto', label: 'Как в системе', icon: IconDeviceDesktop },
];

/** Выбор темы: значок плюс подпись для экранного диктора. Mantine сохраняет выбор в localStorage. */
export function ThemeSwitch() {
  const { colorScheme, setColorScheme } = useMantineColorScheme();
  return (
    <SegmentedControl
      size="xs"
      aria-label="Тема"
      value={colorScheme}
      onChange={(v) => setColorScheme(v as Scheme)}
      data={options.map(({ value, label, icon: Icon }) => ({
        value,
        label: (
          <>
            <Icon size={16} aria-hidden style={{ display: 'block' }} />
            <VisuallyHidden>{label}</VisuallyHidden>
          </>
        ),
      }))}
    />
  );
}
