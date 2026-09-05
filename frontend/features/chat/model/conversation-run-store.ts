export type ConversationRunSnapshot = {
  runID: string;
  conversationPublicID: string;
};

// local 记录连续缺席权威快照超过该时长后视为失效并移除。
// 需大于「本地注册到服务端登记完成」的竞态窗口，小于用户可感知的卡死时长。
const LOCAL_RUN_SNAPSHOT_MISS_TTL_MS = 15_000;

type ConversationRunRecord = {
  conversationPublicID: string;
  status: "local" | "detached" | "server" | "settled";
  // local 记录首次缺席权威快照的时间戳；再次命中快照后清除。
  missingFromSnapshotSince?: number;
};

type Listener = () => void;

export class ConversationRunStore {
  private readonly records = new Map<string, ConversationRunRecord>();
  private readonly listeners = new Map<string, Set<Listener>>();

  subscribe(conversationPublicID: string, listener: Listener): () => void {
    const conversationID = conversationPublicID.trim();
    if (!conversationID) {
      return () => undefined;
    }
    let conversationListeners = this.listeners.get(conversationID);
    if (!conversationListeners) {
      conversationListeners = new Set();
      this.listeners.set(conversationID, conversationListeners);
    }
    conversationListeners.add(listener);
    return () => {
      conversationListeners.delete(listener);
      if (conversationListeners.size === 0) {
        this.listeners.delete(conversationID);
      }
    };
  }

  isConversationRunning(conversationPublicID: string): boolean {
    const conversationID = conversationPublicID.trim();
    if (!conversationID) {
      return false;
    }
    for (const record of this.records.values()) {
      if (record.conversationPublicID === conversationID && record.status !== "settled") {
        return true;
      }
    }
    return false;
  }

  register(runID: string, conversationPublicID: string): void {
    const normalizedRunID = runID.trim();
    const conversationID = conversationPublicID.trim();
    if (!normalizedRunID || !conversationID) {
      return;
    }
    this.mutate(() => {
      const existing = this.records.get(normalizedRunID);
      if (
        existing &&
        (existing.conversationPublicID !== conversationID || existing.status === "settled")
      ) {
        return;
      }
      if (existing?.status === "local") {
        return;
      }
      this.records.set(normalizedRunID, {
        conversationPublicID: conversationID,
        status: "local",
      });
    });
  }

  detach(runID: string): void {
    const normalizedRunID = runID.trim();
    if (!normalizedRunID) {
      return;
    }
    this.mutate(() => {
      const existing = this.records.get(normalizedRunID);
      if (existing?.status === "local") {
        this.records.set(normalizedRunID, { ...existing, status: "detached" });
      }
    });
  }

  settle(runID: string): void {
    const normalizedRunID = runID.trim();
    if (!normalizedRunID) {
      return;
    }
    this.mutate(() => {
      const existing = this.records.get(normalizedRunID);
      if (existing && existing.status !== "settled") {
        this.records.set(normalizedRunID, { ...existing, status: "settled" });
      }
    });
  }

  applyStarted(runID: string, conversationPublicID: string): void {
    const normalizedRunID = runID.trim();
    const conversationID = conversationPublicID.trim();
    if (!normalizedRunID || !conversationID) {
      return;
    }
    this.mutate(() => {
      const existing = this.records.get(normalizedRunID);
      if (existing?.status === "settled") {
        return;
      }
      if (
        existing?.conversationPublicID === conversationID &&
        existing.status === "server"
      ) {
        return;
      }
      this.records.set(normalizedRunID, {
        conversationPublicID: conversationID,
        status: "server",
      });
    });
  }

  applyFinished(runID: string, authoritative: boolean): void {
    const normalizedRunID = runID.trim();
    if (!normalizedRunID) {
      return;
    }
    this.mutate(() => {
      const existing = this.records.get(normalizedRunID);
      if (!existing) {
        return;
      }
      if (authoritative) {
        this.records.delete(normalizedRunID);
      } else if (existing.status !== "settled") {
        this.records.set(normalizedRunID, { ...existing, status: "settled" });
      }
    });
  }

  synchronize(snapshots: readonly ConversationRunSnapshot[]): void {
    const snapshotRunOwners = new Map<string, string>();
    for (const snapshot of snapshots) {
      const runID = snapshot.runID.trim();
      const conversationID = snapshot.conversationPublicID.trim();
      if (runID && conversationID && !snapshotRunOwners.has(runID)) {
        snapshotRunOwners.set(runID, conversationID);
      }
    }

    const now = Date.now();
    this.mutate(() => {
      for (const [runID, record] of this.records) {
        const serverOwner = snapshotRunOwners.get(runID);
        if (record.status === "settled") {
          if (!serverOwner) {
            this.records.delete(runID);
          }
          continue;
        }
        if (serverOwner && serverOwner !== record.conversationPublicID) {
          this.records.set(runID, {
            conversationPublicID: serverOwner,
            status: "server",
          });
          continue;
        }
        if ((record.status === "detached" || record.status === "server") && !serverOwner) {
          this.records.delete(runID);
          continue;
        }
        if (record.status === "detached" && serverOwner) {
          this.records.set(runID, {
            conversationPublicID: record.conversationPublicID,
            status: "server",
          });
          continue;
        }
        if (record.status === "local") {
          if (serverOwner) {
            this.records.set(runID, {
              conversationPublicID: record.conversationPublicID,
              status: "server",
            });
            continue;
          }
          // 服务端从未登记该运行（提交失败、流挂死等）时，本地记录会持续缺席快照。
          // 超时后移除，避免侧边栏加载指示永久卡住；竞态窗口内的缺席不受影响。
          const missingSince = record.missingFromSnapshotSince ?? now;
          if (now - missingSince >= LOCAL_RUN_SNAPSHOT_MISS_TTL_MS) {
            this.records.delete(runID);
          } else if (record.missingFromSnapshotSince === undefined) {
            this.records.set(runID, { ...record, missingFromSnapshotSince: now });
          }
        }
      }

      for (const [runID, conversationPublicID] of snapshotRunOwners) {
        const existing = this.records.get(runID);
        if (
          existing?.status === "settled" &&
          existing.conversationPublicID === conversationPublicID
        ) {
          continue;
        }
        if (
          existing?.conversationPublicID === conversationPublicID &&
          (existing.status === "local" || existing.status === "server")
        ) {
          continue;
        }
        this.records.set(runID, { conversationPublicID, status: "server" });
      }
    });
  }

  clear(): void {
    this.mutate(() => this.records.clear());
  }

  private activeConversationIDs(): Set<string> {
    const result = new Set<string>();
    for (const record of this.records.values()) {
      if (record.status !== "settled") {
        result.add(record.conversationPublicID);
      }
    }
    return result;
  }

  private mutate(update: () => void): void {
    const previous = this.activeConversationIDs();
    update();
    const next = this.activeConversationIDs();
    for (const conversationID of new Set([...previous, ...next])) {
      if (previous.has(conversationID) !== next.has(conversationID)) {
        for (const listener of this.listeners.get(conversationID) ?? []) {
          listener();
        }
      }
    }
  }
}
