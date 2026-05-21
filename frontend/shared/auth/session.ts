export type SessionSnapshot = {
  accessToken: string;
  sessionID: string;
};

export const SESSION_SNAPSHOT_CHANGED_EVENT = "deeix-chat:session-snapshot-changed";

const sessionSnapshot: SessionSnapshot = {
  accessToken: "",
  sessionID: "",
};

function dispatchSessionSnapshotChanged(): void {
  if (typeof window === "undefined") {
    return;
  }
  window.dispatchEvent(
    new CustomEvent<SessionSnapshot>(SESSION_SNAPSHOT_CHANGED_EVENT, {
      detail: readSessionSnapshot(),
    }),
  );
}

export function readAccessToken(): string {
  return sessionSnapshot.accessToken;
}

export function readSessionID(): string {
  return sessionSnapshot.sessionID;
}

export function readSessionSnapshot(): SessionSnapshot {
  return {
    ...sessionSnapshot,
  };
}

export function writeSessionSnapshot(next: Partial<SessionSnapshot>): void {
  const previousAccessToken = sessionSnapshot.accessToken;
  const previousSessionID = sessionSnapshot.sessionID;
  if (typeof next.accessToken === "string") sessionSnapshot.accessToken = next.accessToken;
  if (typeof next.sessionID === "string") sessionSnapshot.sessionID = next.sessionID;
  if (sessionSnapshot.accessToken !== previousAccessToken || sessionSnapshot.sessionID !== previousSessionID) {
    dispatchSessionSnapshotChanged();
  }
}

export function writeAccessToken(token: string): void {
  writeSessionSnapshot({ accessToken: token });
}

export function clearSessionSnapshot(): void {
  writeSessionSnapshot({
    accessToken: "",
    sessionID: "",
  });
}

export function clearSessionAndRedirectToLogin(): void {
  clearSessionSnapshot();
  if (typeof window !== "undefined") {
    window.location.replace("/login");
  }
}
