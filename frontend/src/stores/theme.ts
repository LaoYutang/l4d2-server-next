import { defineStore } from 'pinia';
import { ref, watch } from 'vue';

const THEME_STORAGE_KEY = 'l4d2_manager_theme';
const LEGACY_THEME_STORAGE_KEY = 'theme';

type StoredTheme = 'dark' | 'light';

const getStoredTheme = (): StoredTheme | null => {
  const storedTheme =
    localStorage.getItem(THEME_STORAGE_KEY) ?? localStorage.getItem(LEGACY_THEME_STORAGE_KEY);
  return storedTheme === 'dark' || storedTheme === 'light' ? storedTheme : null;
};

export const useThemeStore = defineStore('theme', () => {
  const storedTheme = getStoredTheme();
  const systemDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
  const isDark = ref(storedTheme === 'dark' || (!storedTheme && systemDark));

  const toggleTheme = () => {
    isDark.value = !isDark.value;
  };

  watch(
    isDark,
    (val) => {
      if (val) {
        document.documentElement.classList.add('dark');
        localStorage.setItem(THEME_STORAGE_KEY, 'dark');
      } else {
        document.documentElement.classList.remove('dark');
        localStorage.setItem(THEME_STORAGE_KEY, 'light');
      }
      localStorage.removeItem(LEGACY_THEME_STORAGE_KEY);
    },
    { immediate: true }
  );

  return {
    isDark,
    toggleTheme,
  };
});
