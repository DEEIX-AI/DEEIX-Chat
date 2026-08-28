export type SidebarRevealTarget = "expanded" | "collapsed";

export type SidebarRevealTransitionState = {
  target: SidebarRevealTarget;
  layout: SidebarRevealTarget;
  resizing: boolean;
};

export type SidebarRevealTransitionAction =
  | { type: "begin"; target: SidebarRevealTarget }
  | { type: "settle"; target: SidebarRevealTarget }
  | { type: "sync"; target: SidebarRevealTarget };

export function createSidebarRevealTransitionState(
  target: SidebarRevealTarget,
): SidebarRevealTransitionState {
  return { target, layout: target, resizing: false };
}

export function sidebarRevealTransitionReducer(
  state: SidebarRevealTransitionState,
  action: SidebarRevealTransitionAction,
): SidebarRevealTransitionState {
  if (action.type === "sync") {
    return createSidebarRevealTransitionState(action.target);
  }

  if (action.type === "begin") {
    return {
      target: action.target,
      // Expanded children must be laid out before the shell reveals them. On
      // collapse, keep that geometry stable until the width transition ends.
      layout: action.target === "expanded" ? "expanded" : state.layout,
      resizing: true,
    };
  }

  if (action.target !== state.target) {
    return state;
  }

  return {
    target: action.target,
    layout: action.target,
    resizing: false,
  };
}
