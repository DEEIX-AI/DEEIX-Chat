"use client";

import * as React from "react";

type UIBlockErrorBoundaryProps = {
  // Clears a caught error when it changes (compared by identity). Hosts pass the block
  // source, so a block that throws on a half-streamed body is retried as soon as more of
  // it arrives; live blocks pass the resolved node, new on every resolution.
  resetKey: unknown;
  fallback: React.ReactNode;
  children: React.ReactNode;
};

type UIBlockErrorBoundaryState = {
  failed: boolean;
  resetKey: unknown;
};

// Keeps a component that throws while rendering from taking the whole message down with it.
// React only supports error boundaries as class components.
export class UIBlockErrorBoundary extends React.Component<UIBlockErrorBoundaryProps, UIBlockErrorBoundaryState> {
  state: UIBlockErrorBoundaryState = { failed: false, resetKey: this.props.resetKey };

  static getDerivedStateFromError(): Partial<UIBlockErrorBoundaryState> {
    return { failed: true };
  }

  static getDerivedStateFromProps(props: UIBlockErrorBoundaryProps, state: UIBlockErrorBoundaryState): Partial<UIBlockErrorBoundaryState> | null {
    if (props.resetKey === state.resetKey) {
      return null;
    }
    return { failed: false, resetKey: props.resetKey };
  }

  render() {
    return this.state.failed ? this.props.fallback : this.props.children;
  }
}
