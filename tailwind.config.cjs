/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./internal/web/templates/**/*.tmpl",
    "./internal/web/assets/**/*.js",
    "./internal/web/**/*.go",
    "!./internal/web/**/*_test.go",
  ],
  theme: {
    extend: {
      colors: {
        tomb: {
          bg: "var(--mt-bg)",
          surface: "var(--mt-surface)",
          raised: "var(--mt-surface-raised)",
          border: "var(--mt-border)",
          text: "var(--mt-text)",
          muted: "var(--mt-text-muted)",
          accent: "var(--mt-accent)",
        },
      },
    },
  },
};
