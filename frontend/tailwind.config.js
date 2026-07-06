/** @type {import('tailwindcss').Config} */
// Design tokens mirror design/ui.pen variables. Dark-first; the `.light` class on
// <html> flips the CSS variables (see src/index.css), so Tailwind color tokens
// reference those variables and both themes resolve from one definition.
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  darkMode: ['class', '.light'],
  theme: {
    extend: {
      colors: {
        bg: 'var(--bg)',
        base: 'var(--base)',
        sidebar: 'var(--sidebar)',
        winbar: 'var(--winbar)',
        surface: 'var(--surface)',
        surface2: 'var(--surface2)',
        border: 'var(--border)',
        border2: 'var(--border2)',
        amber: 'var(--amber)',
        'amber-bg': 'var(--amber-bg)',
        'amber-ink': 'var(--amber-ink)',
        teal: 'var(--teal)',
        'teal-bg': 'var(--teal-bg)',
        'teal-ink': 'var(--teal-ink)',
        red: 'var(--red)',
        'red-bg': 'var(--red-bg)',
        'red-ink': 'var(--red-ink)',
        ink: 'var(--ink)',
        text: 'var(--text)',
        text2: 'var(--text2)',
        muted: 'var(--muted)',
        muted2: 'var(--muted2)',
        'on-primary': 'var(--on-primary)',
      },
      fontFamily: {
        display: ['Space Grotesk', 'system-ui', 'sans-serif'],
        body: ['Hanken Grotesk', 'system-ui', 'sans-serif'],
        mono: ['Spline Sans Mono', 'ui-monospace', 'monospace'],
      },
      borderRadius: {
        pill: '999px',
      },
    },
  },
  plugins: [],
}
