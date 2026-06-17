// __IS_DESKTOP__ is replaced at build time by Vite `define`.
// The window.go runtime check guards against accidentally running the desktop
// Vite config (`npm run dev`) in a plain browser — it gracefully falls back to
// the WASM path instead of crashing with "window.go is undefined".
// In a real Wails webview, window.go is always injected before the page loads.
export const IS_DESKTOP: boolean =
  __IS_DESKTOP__ && typeof (window as any).go !== 'undefined'
