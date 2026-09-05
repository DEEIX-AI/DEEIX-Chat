"use client";

import * as React from "react";

import { type FeaturePolicy, getFeaturePolicy } from "@/shared/api/settings";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

const DEFAULT_FEATURE_POLICY: FeaturePolicy = { knowledgeBaseEnabled: true };

let cachedFeaturePolicy: FeaturePolicy | null = null;
let inflightFeaturePolicy: Promise<void> | null = null;
const featurePolicyListeners = new Set<() => void>();

function setCachedFeaturePolicy(policy: FeaturePolicy) {
  cachedFeaturePolicy = policy;
  for (const listener of featurePolicyListeners) listener();
}

/** 管理端修改功能开关成功后调用，让当前标签页内的消费方立即生效。 */
export function overrideFeaturePolicy(patch: Partial<FeaturePolicy>) {
  setCachedFeaturePolicy({ ...(cachedFeaturePolicy ?? DEFAULT_FEATURE_POLICY), ...patch });
}

function loadFeaturePolicy(): Promise<void> {
  inflightFeaturePolicy ??= (async () => {
    try {
      const token = await resolveAccessToken();
      if (!token) {
        setCachedFeaturePolicy(DEFAULT_FEATURE_POLICY);
        return;
      }
      setCachedFeaturePolicy(await getFeaturePolicy(token));
    } catch {
      // 拉取失败按默认全部开启处理（fail-open），避免误伤正常部署或阻塞路由守卫。
      setCachedFeaturePolicy(DEFAULT_FEATURE_POLICY);
    } finally {
      inflightFeaturePolicy = null;
    }
  })();
  return inflightFeaturePolicy;
}

function subscribeFeaturePolicy(listener: () => void): () => void {
  featurePolicyListeners.add(listener);
  return () => featurePolicyListeners.delete(listener);
}

function getFeaturePolicySnapshot(): FeaturePolicy | null {
  return cachedFeaturePolicy;
}

/**
 * 读取后台功能开关策略（会话级缓存，首次挂载时拉取一次，缓存更新时全量通知）。
 * 加载完成前返回默认全开，`loaded` 用于需要等待确定结果的场景（如路由守卫）。
 */
export function useFeaturePolicy(): FeaturePolicy & { loaded: boolean } {
  const policy = React.useSyncExternalStore(
    subscribeFeaturePolicy,
    getFeaturePolicySnapshot,
    getFeaturePolicySnapshot,
  );

  React.useEffect(() => {
    if (!cachedFeaturePolicy) void loadFeaturePolicy();
  }, []);

  return policy ? { ...policy, loaded: true } : { ...DEFAULT_FEATURE_POLICY, loaded: false };
}
