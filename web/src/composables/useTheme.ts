import { ref, watchEffect } from 'vue'

type Theme = 'dark' | 'light'

const theme = ref<Theme>((localStorage.getItem('theme') as Theme) || 'dark')

export function useTheme() {
  function setTheme(t: Theme) {
    theme.value = t
    localStorage.setItem('theme', t)
  }

  function toggleTheme() {
    setTheme(theme.value === 'dark' ? 'light' : 'dark')
  }

  // Apply theme class to document
  watchEffect(() => {
    document.documentElement.setAttribute('data-theme', theme.value)
  })

  return { theme, setTheme, toggleTheme }
}
