/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: { extend: {} },
  safelist: [{ pattern: /max-w-.*/gm }, { pattern: /bg-.*/gm }],
  plugins: [require('@tailwindcss/forms')],
}
