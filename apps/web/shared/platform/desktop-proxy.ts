"use client";

import { invoke } from "@tauri-apps/api/core";

import { isBoolean, isFiniteNumber, isOneOf, isRecord, isString } from "@/shared/lib/type-guards";

import { isDesktopApp } from "./runtime";

// Network proxy of the desktop app. The shell (apps/desktop/src-tauri/src/proxy.rs)
// resolves it once per launch and applies it to the bundled server, its own
// requests and the webviews; this page only edits the saved setting and shows
// what is in force. Payloads are validated because they cross IPC as untyped JSON.

export const PROXY_MODES = ["system", "manual", "direct"] as const;
export const PROXY_SCHEMES = ["http", "socks5"] as const;
export const PROXY_SOURCES = ["policy", "manual", "direct", "environment", "system", "none"] as const;
const WEBVIEW_PROXIES = ["system", "fixed", "direct"] as const;
const PROXY_ERROR_KINDS = ["policy", "invalid", "storage"] as const;
const PROXY_ERROR_FIELDS = ["host", "port", "bypass"] as const;

export type ProxyMode = (typeof PROXY_MODES)[number];
export type ProxyScheme = (typeof PROXY_SCHEMES)[number];
/** Which source decided the route in force: policy, the app setting, or the system. */
export type ProxySource = (typeof PROXY_SOURCES)[number];
export type ProxyErrorField = (typeof PROXY_ERROR_FIELDS)[number];

export type ProxySettings = {
  mode: ProxyMode;
  scheme: ProxyScheme;
  host: string;
  port: number | null;
  username: string;
  /** Hosts, addresses and ranges that skip the proxy. */
  bypass: string;
};

export type ProxyStatus = {
  /** As saved; applies from the next launch when `restartRequired`. */
  settings: ProxySettings;
  /** A password is stored in the system keychain; it is never sent back. */
  hasPassword: boolean;
  managedByPolicy: boolean;
  restartRequired: boolean;
  /** The system settings changed since launch; a restart picks them up. */
  systemChanged: boolean;
  effective: {
    source: ProxySource;
    http: string | null;
    https: string | null;
    bypass: string;
    /** Only a PAC script is configured, which the app cannot apply. */
    pacIgnored: boolean;
    webview: (typeof WEBVIEW_PROXIES)[number];
    /** The app window could not take the setting (platform limits, or a proxy that needs a password). */
    webviewFollowsSystem: boolean;
  };
};

/** `undefined` keeps the stored password, `""` removes it. */
export type ProxyUpdate = { settings: ProxySettings; password?: string };

/** Why the shell refused a setting; `field` names the input at fault and `value` the offending entry. */
export type ProxyError = {
  kind: (typeof PROXY_ERROR_KINDS)[number];
  field?: ProxyErrorField;
  value?: string;
  message: string;
};

export const DEFAULT_PROXY_SETTINGS: ProxySettings = {
  mode: "system",
  scheme: "http",
  host: "",
  port: null,
  username: "",
  bypass: "",
};

const isNullableString = (value: unknown): value is string | null => value === null || isString(value);

function isProxySettings(value: unknown): value is ProxySettings {
  return (
    isRecord(value) &&
    isOneOf(PROXY_MODES)(value.mode) &&
    isOneOf(PROXY_SCHEMES)(value.scheme) &&
    isString(value.host) &&
    (value.port === null || isFiniteNumber(value.port)) &&
    isString(value.username) &&
    isString(value.bypass)
  );
}

function isEffective(value: unknown): value is ProxyStatus["effective"] {
  return (
    isRecord(value) &&
    isOneOf(PROXY_SOURCES)(value.source) &&
    isNullableString(value.http) &&
    isNullableString(value.https) &&
    isString(value.bypass) &&
    isBoolean(value.pacIgnored) &&
    isOneOf(WEBVIEW_PROXIES)(value.webview) &&
    isBoolean(value.webviewFollowsSystem)
  );
}

function isProxyStatus(value: unknown): value is ProxyStatus {
  return (
    isRecord(value) &&
    isProxySettings(value.settings) &&
    isBoolean(value.hasPassword) &&
    isBoolean(value.managedByPolicy) &&
    isBoolean(value.restartRequired) &&
    isBoolean(value.systemChanged) &&
    isEffective(value.effective)
  );
}

export function isProxyError(value: unknown): value is ProxyError {
  return (
    isRecord(value) &&
    isOneOf(PROXY_ERROR_KINDS)(value.kind) &&
    (value.field === undefined || isOneOf(PROXY_ERROR_FIELDS)(value.field)) &&
    (value.value === undefined || isString(value.value)) &&
    isString(value.message)
  );
}

/** The saved setting and the route in force. Null in a browser. */
export async function getNetworkProxy(): Promise<ProxyStatus | null> {
  if (!isDesktopApp()) {
    return null;
  }
  const payload: unknown = await invoke("get_network_proxy");
  if (!isProxyStatus(payload)) {
    throw new Error("Unexpected get_network_proxy payload");
  }
  return payload;
}

/** Save the setting; it applies from the next launch. Rejects with a `ProxyError`. */
export async function setNetworkProxy(update: ProxyUpdate): Promise<ProxyStatus> {
  const payload: unknown = await invoke("set_network_proxy", { update });
  if (!isProxyStatus(payload)) {
    throw new Error("Unexpected set_network_proxy payload");
  }
  return payload;
}
