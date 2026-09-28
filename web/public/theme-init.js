(function () {
  try {
    var stored = localStorage.getItem('theme')
    var theme = stored === 'light' || stored === 'dark' || stored === 'system'
      ? stored
      : 'system'
    var isDark = theme === 'dark' || (
      theme === 'system' &&
      window.matchMedia('(prefers-color-scheme: dark)').matches
    )
    if (isDark) document.documentElement.classList.add('dark')
    document.documentElement.style.colorScheme = isDark ? 'dark' : 'light'
  } catch (_) {
    // localStorage can throw in sandboxed contexts; fall back to light.
  }
})()
