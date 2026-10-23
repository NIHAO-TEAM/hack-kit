// Тема ставится до загрузки приложения, без мигания: сохранённый выбор, иначе тёмная.
try {
  var saved = localStorage.getItem('mantine-color-scheme-value');
  var mode =
    saved === 'light' || saved === 'dark'
      ? saved
      : saved === 'auto' && matchMedia('(prefers-color-scheme: light)').matches
        ? 'light'
        : 'dark';
  document.documentElement.setAttribute('data-mantine-color-scheme', mode);
} catch {
  document.documentElement.setAttribute('data-mantine-color-scheme', 'dark');
}
