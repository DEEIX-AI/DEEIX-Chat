// Global injected by the Tauri shell at document start (desktop builds only).

// biome-ignore lint/style/useConsistentTypeDefinitions: augmenting the global Window requires interface declaration merging.
interface Window {
  /** Present when the page runs inside the Tauri webview. */
  isTauri?: boolean;
}
