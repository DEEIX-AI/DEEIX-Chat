"use client";

import * as React from "react";

type MobileHeaderActionContextValue = {
  slot: HTMLElement | null;
  setSlot: React.Dispatch<React.SetStateAction<HTMLElement | null>>;
};

const MobileHeaderActionContext = React.createContext<MobileHeaderActionContextValue | null>(null);

export function MobileHeaderActionProvider({ children }: { children: React.ReactNode }) {
  const [slot, setSlot] = React.useState<HTMLElement | null>(null);
  const value = React.useMemo(() => ({ slot, setSlot }), [slot]);

  return (
    <MobileHeaderActionContext.Provider value={value}>
      {children}
    </MobileHeaderActionContext.Provider>
  );
}

export function MobileHeaderActionSlot() {
  const context = React.useContext(MobileHeaderActionContext);
  if (!context) {
    throw new Error("MobileHeaderActionSlot must be used within MobileHeaderActionProvider");
  }
  return <div ref={context.setSlot} className="flex items-center" />;
}

export function useMobileHeaderActionSlot() {
  const context = React.useContext(MobileHeaderActionContext);
  if (!context) {
    throw new Error("useMobileHeaderActionSlot must be used within MobileHeaderActionProvider");
  }
  return context.slot;
}
