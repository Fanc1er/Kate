/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,ts}'],
  corePlugins: {
    // preflight 的全局 reset 会与 @opentiny/vue-theme 及老页面既有样式冲突，保持关闭；
    // border 类所需的基础规则在入口 CSS 中手动补齐。
    preflight: false,
  },
  theme: {
    extend: {},
  },
  plugins: [],
}
