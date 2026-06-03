export type SessionSnapshot = {
  accessToken: string;
  sessionID: string;
};

export const SESSION_SNAPSHOT_CHANGED_EVENT = "deeix-chat:session-snapshot-changed";

const SESSION_CHANNEL_NAME = "deeix-chat:session-snapshot";
const SESSION_CHANNEL_MESSAGE_TYPE = "session_snapshot";

type SessionSnapshotWriteOptions = {
  syncPeers?: boolean;
};

type SessionChannelMessage = {
  type: typeof SESSION_CHANNEL_MESSAGE_TYPE;
  snapshot: SessionSnapshot;
};

let sessionRevision = 0;
let sessionChannel: BroadcastChannel | null = null;
let sessionChannelInitialized = false;

const sessionSnapshot: SessionSnapshot = {
  accessToken: "",
  sessionID: "",
};

function isSessionSnapshot(value: unknown): value is SessionSnapshot {
  if (!value || typeof value !== "object") {
    return false;
  }

  const snapshot = value as Partial<SessionSnapshot>;
  return typeof snapshot.accessToken === "string" && typeof snapshot.sessionID === "string";
}

function ensureSessionChannel(): BroadcastChannel | null {
  if (sessionChannelInitialized) {
    return sessionChannel;
  }

  sessionChannelInitialized = true;
  if (typeof window === "undefined" || !("BroadcastChannel" in window)) {
    return null;
  }

  try {
    sessionChannel = new BroadcastChannel(SESSION_CHANNEL_NAME);
    sessionChannel.onmessage = (event: MessageEvent<SessionChannelMessage>) => {
      const message = event.data;
      if (message?.type !== SESSION_CHANNEL_MESSAGE_TYPE || !isSessionSnapshot(message.snapshot)) {
        return;
      }
      applySessionSnapshot(message.snapshot, { syncPeers: false });
    };
  } catch {
    sessionChannel = null;
  }

  return sessionChannel;
}

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

function publishSessionSnapshotChanged(): void {
  ensureSessionChannel()?.postMessage({
    type: SESSION_CHANNEL_MESSAGE_TYPE,
    snapshot: readSessionSnapshot(),
  } satisfies SessionChannelMessage);
}

function applySessionSnapshot(next: Partial<SessionSnapshot>, options: SessionSnapshotWriteOptions): void {
  const previousAccessToken = sessionSnapshot.accessToken;
  const previousSessionID = sessionSnapshot.sessionID;
  if (typeof next.accessToken === "string") sessionSnapshot.accessToken = next.accessToken;
  if (typeof next.sessionID === "string") sessionSnapshot.sessionID = next.sessionID;
  if (sessionSnapshot.accessToken !== previousAccessToken || sessionSnapshot.sessionID !== previousSessionID) {
    sessionRevision += 1;
    dispatchSessionSnapshotChanged();
    if (options.syncPeers !== false) {
      publishSessionSnapshotChanged();
    }
  }
}

export function readAccessToken(): string {
  ensureSessionChannel();
  return sessionSnapshot.accessToken;
}

export function readSessionID(): string {
  ensureSessionChannel();
  return sessionSnapshot.sessionID;
}

export function readSessionSnapshot(): SessionSnapshot {
  ensureSessionChannel();
  return {
    ...sessionSnapshot,
  };
}

export function readSessionRevision(): number {
  ensureSessionChannel();
  return sessionRevision;
}

export function writeSessionSnapshot(next: Partial<SessionSnapshot>, options: SessionSnapshotWriteOptions = {}): void {
  applySessionSnapshot(next, options);
}

export function writeAccessToken(token: string): void {
  writeSessionSnapshot({ accessToken: token });
}

export function clearSessionSnapshot(options: SessionSnapshotWriteOptions = {}): void {
  writeSessionSnapshot(
    {
      accessToken: "",
      sessionID: "",
    },
    options,
  );
}

export function clearSessionAndRedirectToLogin(): void {
  clearSessionSnapshot();
  if (typeof window !== "undefined") {
    window.location.replace("/login");
  }
}
