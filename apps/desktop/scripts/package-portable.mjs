// Packs the Windows portable edition:
//
//   DEEIX-Chat-<version>-windows-<arch>-portable[-offline].zip
//   └── DEEIX Chat/
//       ├── DEEIX Chat.exe          the cargo binary (deeix-chat-desktop.exe), renamed
//       ├── deeix-chat-server.exe   the local-mode sidecar
//       ├── portable                marker that switches the app to portable mode
//       ├── README.txt
//       └── webview2/               offline variant only: WebView2 Fixed Version runtime
//
// Where the binaries come from after `tauri build` (verified against
// tauri-bundler 2.9.4, used by @tauri-apps/cli 2.11.5):
//   - target[/<triple>]/release/deeix-chat-desktop.exe is the *original*
//     binary. The bundler patches the bundle type into it, signs it and packs
//     it once per installer, then restores the unpatched, UNSIGNED original.
//     So this exe reports no bundle type (the marker decides portable mode)
//     and CI must sign it before packing.
//   - binaries/deeix-chat-server-<triple>.exe is signed in place by the
//     bundler when a certificate is configured, and is preferred over the
//     copy tauri-build placed in target/ at compile time (never signed).
//
// The zip is written here (store/deflate, no ZIP64) rather than
// with Compress-Archive: Windows PowerShell 5.1 writes backslash separators,
// and a pure implementation is deterministic and testable on any OS.
//
// Usage:
//   node scripts/package-portable.mjs [--version <v>] [--webview2 <dir>] [--out-dir <dir>]
//        [--exe <path>] [--sidecar <path>] [--target <rust-triple>]
// Prints the path of the zip it wrote.

import { closeSync, existsSync, mkdirSync, openSync, readdirSync, readFileSync, renameSync, rmSync, statSync, writeSync } from "node:fs";
import { basename, dirname, join, relative, resolve, sep } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { crc32, deflateRawSync } from "node:zlib";

const here = dirname(fileURLToPath(import.meta.url));
const srcTauri = resolve(here, "..", "src-tauri");

export const ROOT_FOLDER = "DEEIX Chat";
export const APP_EXE = "DEEIX Chat.exe";
export const SIDECAR_EXE = "deeix-chat-server.exe";
export const MARKER_FILE = "portable";
export const WEBVIEW2_FOLDER = "webview2";
const SIDECAR_STEM = "deeix-chat-server";
const DEFAULT_TRIPLE = "x86_64-pc-windows-msvc";
const WEBVIEW2_URL = "https://developer.microsoft.com/microsoft-edge/webview2/";

// Windows tools (Notepad on Server 2016, `type`) expect CRLF.
const crlf = (text) => text.replace(/\r?\n/g, "\r\n");

export const MARKER_TEXT = crlf(
  "This file makes DEEIX Chat run as a portable app: all data stays in the \"data\" folder next to it. Do not delete it.\n",
);

export function portableAssetName(version, { arch = "x64", offline = false } = {}) {
  return `DEEIX-Chat-${version}-windows-${arch}-portable${offline ? "-offline" : ""}.zip`;
}

export function archFromTriple(triple) {
  if (triple.startsWith("x86_64")) return "x64";
  if (triple.startsWith("aarch64")) return "arm64";
  if (/^i[3-6]86/.test(triple)) return "x86";
  throw new Error(`unsupported target triple: ${triple}`);
}

export function readmeText(version, { offline = false } = {}) {
  const webviewEn = offline
    ? `- Microsoft Edge WebView2 Runtime: a private copy is included in the
  "webview2" folder, so nothing needs to be installed.`
    : `- Microsoft Edge WebView2 Runtime. It is part of Windows 11 and of
  up-to-date Windows 10; if it is missing the app tells you and links to
  ${WEBVIEW2_URL}
  The "-portable-offline" package includes it and needs nothing installed.`;
  const webviewZh = offline
    ? `- Microsoft Edge WebView2 运行时：已随附在“webview2”文件夹中，无需安装。`
    : `- Microsoft Edge WebView2 运行时。Windows 11 及保持更新的 Windows 10 已自带；
  如缺失，应用会提示并给出下载地址：
  ${WEBVIEW2_URL}
  “-portable-offline”包已自带该运行时，无需安装。`;

  return crlf(`DEEIX Chat ${version} - portable edition for Windows
====================================================

REQUIREMENTS
- Windows 10 or 11 (64-bit), or Windows Server 2016 or later.
${webviewEn}
- No installation and no administrator rights.

GETTING STARTED
1. Extract the whole "${ROOT_FOLDER}" folder to a place you can write to,
   such as Documents, the desktop or a USB drive. Do not run it from inside
   the zip file or from "Program Files".
2. Start "${APP_EXE}".

WHERE YOUR DATA IS
- Everything is kept in the "data" folder next to "${APP_EXE}" (created on
  first start): open tabs, the built-in local server's database and files,
  and the browser profile (cookies, cache). Nothing is written to %APPDATA%
  or %LOCALAPPDATA%.
- Sign-in tokens are NOT kept in that folder. They stay in the Windows
  Credential Manager of the current Windows user, never as plain text on
  disk. On another computer or Windows account you sign in again.
- The file "${MARKER_FILE}" switches the app to this mode. Do not delete it.

UPDATING
The portable edition never updates itself; it tells you when a new version
is available and links to the download page. To update, quit DEEIX Chat
(tray icon > Quit), download the new portable zip and extract it over this
folder, keeping the "data" folder.

REMOVING
1. In the app, sign out of every server. This deletes its token from
   Windows Credential Manager.
2. Quit DEEIX Chat (tray icon > Quit).
3. Delete the "${ROOT_FOLDER}" folder.
Tokens of a copy that was deleted without signing out can be removed in
Control Panel > Credential Manager > Windows Credentials: entries whose name
contains "com.deeix.chat.desktop.portable-".


DEEIX Chat ${version} - Windows 便携版
======================================

系统要求
- Windows 10 或 11（64 位），或 Windows Server 2016 及以上。
${webviewZh}
- 无需安装，无需管理员权限。

开始使用
1. 将整个“${ROOT_FOLDER}”文件夹解压到可写入的位置，例如“文档”、桌面或 U 盘。
   不要直接在压缩包内运行，也不要放在“Program Files”下。
2. 运行“${APP_EXE}”。

数据存放位置
- 所有数据都保存在“${APP_EXE}”旁边的“data”文件夹中（首次启动时创建）：
  已打开的标签页、内置本地服务器的数据库和文件、浏览器配置（Cookie、缓存）。
  不会写入 %APPDATA% 或 %LOCALAPPDATA%。
- 登录凭据不保存在该文件夹中，而是保存在当前 Windows 用户的“凭据管理器”里，
  绝不以明文形式写入磁盘。换一台电脑或 Windows 账户需要重新登录。
- “${MARKER_FILE}”文件用于启用便携模式，请勿删除。

更新
便携版不会自动更新，有新版本时只会提示并给出下载页面链接。更新方法：退出
DEEIX Chat（托盘图标 > 退出），下载新的便携版压缩包并解压覆盖本文件夹，
保留“data”文件夹即可。

彻底删除
1. 在应用中退出所有服务器的登录，这会从 Windows 凭据管理器中删除对应凭据。
2. 退出 DEEIX Chat（托盘图标 > 退出）。
3. 删除“${ROOT_FOLDER}”文件夹。
如果某个副本在未退出登录的情况下被删除，可在“控制面板 > 凭据管理器 >
Windows 凭据”中删除名称包含“com.deeix.chat.desktop.portable-”的条目。
`);
}

/** Cargo package name = the binary `tauri build` produces (no mainBinaryName is configured). */
export function cargoBinaryName(cargoToml) {
  const section = cargoToml.slice(cargoToml.indexOf("[package]"));
  const name = /^name\s*=\s*"([^"]+)"/m.exec(section)?.[1];
  if (!name) {
    throw new Error("cannot read [package] name from Cargo.toml");
  }
  return name;
}

/** Locate the release exe and sidecar under src-tauri. */
export function findBuildOutputs(tauriDir, triple = DEFAULT_TRIPLE) {
  const binary = `${cargoBinaryName(readFileSync(join(tauriDir, "Cargo.toml"), "utf8"))}.exe`;
  const newest = (paths) =>
    paths.filter((p) => existsSync(p)).sort((a, b) => statSync(b).mtimeMs - statSync(a).mtimeMs)[0];
  const exe = newest([join(tauriDir, "target", triple, "release", binary), join(tauriDir, "target", "release", binary)]);
  // First match wins: the bundler-signed source before tauri-build's copies.
  const sidecar = [
    join(tauriDir, "binaries", `${SIDECAR_STEM}-${triple}.exe`),
    join(tauriDir, "target", triple, "release", SIDECAR_EXE),
    join(tauriDir, "target", "release", SIDECAR_EXE),
  ].find((p) => existsSync(p));
  if (!exe) {
    throw new Error(`no ${binary} under ${join(tauriDir, "target")}; run \`pnpm tauri build\` first`);
  }
  if (!sidecar) {
    throw new Error(`no ${SIDECAR_STEM} build for ${triple}; run \`pnpm build:sidecar\` first`);
  }
  return { exe, sidecar };
}

/**
 * A WebView2 Fixed Version runtime folder: `dir` itself, or its only
 * subfolder (the .cab expands to Microsoft.WebView2.FixedVersionRuntime.<v>.<arch>/).
 */
export function resolveWebview2Dir(dir) {
  const hasRuntime = (d) => existsSync(join(d, "msedgewebview2.exe"));
  if (hasRuntime(dir)) {
    return dir;
  }
  const children = readdirSync(dir, { withFileTypes: true }).filter((e) => e.isDirectory());
  if (children.length === 1 && hasRuntime(join(dir, children[0].name))) {
    return join(dir, children[0].name);
  }
  throw new Error(`${dir} does not contain a WebView2 Fixed Version runtime (msedgewebview2.exe)`);
}

function walk(dir) {
  const out = [];
  for (const entry of readdirSync(dir, { withFileTypes: true }).sort((a, b) => (a.name < b.name ? -1 : 1))) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) {
      out.push({ dir: path }, ...walk(path));
    } else if (entry.isFile()) {
      out.push({ file: path });
    } else {
      throw new Error(`unsupported file type in the WebView2 runtime: ${path}`);
    }
  }
  return out;
}

/** Zip entries in archive order: `{ name, source }`, `{ name, content }` or a directory `{ name }` ending in "/". */
export function planEntries({ exe, sidecar, version, webview2 }) {
  const root = `${ROOT_FOLDER}/`;
  const entries = [
    { name: root },
    { name: `${root}${APP_EXE}`, source: exe },
    { name: `${root}${SIDECAR_EXE}`, source: sidecar },
    { name: `${root}${MARKER_FILE}`, content: MARKER_TEXT },
    { name: `${root}README.txt`, content: readmeText(version, { offline: Boolean(webview2) }) },
  ];
  if (webview2) {
    const base = resolveWebview2Dir(webview2);
    entries.push({ name: `${root}${WEBVIEW2_FOLDER}/` });
    for (const item of walk(base)) {
      const rel = relative(base, item.dir ?? item.file).split(sep).join("/");
      entries.push(item.dir ? { name: `${root}${WEBVIEW2_FOLDER}/${rel}/` } : { name: `${root}${WEBVIEW2_FOLDER}/${rel}`, source: item.file });
    }
  }
  return entries;
}

function dosDateTime(date) {
  const year = Math.max(date.getUTCFullYear(), 1980);
  return {
    time: (date.getUTCHours() << 11) | (date.getUTCMinutes() << 5) | Math.floor(date.getUTCSeconds() / 2),
    date: ((year - 1980) << 9) | ((date.getUTCMonth() + 1) << 5) | date.getUTCDate(),
  };
}

const LIMIT_32 = 0xffffffff;
const FLAG_UTF8 = 0x0800;
const ATTR_DIRECTORY = 0x10;

/** Write `entries` to `outFile` (atomically, via a temp file). */
export function writeZip(entries, outFile, { mtime = new Date() } = {}) {
  if (entries.length >= 0xffff) {
    throw new Error("too many entries for a zip without ZIP64");
  }
  const { time, date } = dosDateTime(mtime);
  const tmp = `${outFile}.partial`;
  mkdirSync(dirname(outFile), { recursive: true });
  const fd = openSync(tmp, "w");
  const central = [];
  let offset = 0;
  const write = (buffer) => {
    writeSync(fd, buffer);
    offset += buffer.length;
  };
  try {
    for (const entry of entries) {
      const isDir = entry.name.endsWith("/");
      const data = isDir ? Buffer.alloc(0) : entry.source ? readFileSync(entry.source) : Buffer.from(entry.content, "utf8");
      const deflated = data.length > 0 ? deflateRawSync(data, { level: 9 }) : null;
      const [method, body] = deflated && deflated.length < data.length ? [8, deflated] : [0, data];
      const crc = crc32(data);
      const name = Buffer.from(entry.name, "utf8");
      // Only flag UTF-8 when needed: some unzip tools mishandle the flag.
      const flags = /^[\x20-\x7e]*$/.test(entry.name) ? 0 : FLAG_UTF8;
      if (data.length > LIMIT_32 || offset > LIMIT_32) {
        throw new Error(`${entry.name}: archive exceeds 4 GiB, which needs ZIP64`);
      }

      const local = Buffer.alloc(30);
      local.writeUInt32LE(0x04034b50, 0);
      local.writeUInt16LE(20, 4); // version needed: 2.0 (deflate)
      local.writeUInt16LE(flags, 6);
      local.writeUInt16LE(method, 8);
      local.writeUInt16LE(time, 10);
      local.writeUInt16LE(date, 12);
      local.writeUInt32LE(crc, 14);
      local.writeUInt32LE(body.length, 18);
      local.writeUInt32LE(data.length, 22);
      local.writeUInt16LE(name.length, 26);
      local.writeUInt16LE(0, 28);
      const localOffset = offset;
      write(local);
      write(name);
      write(body);

      const header = Buffer.alloc(46);
      header.writeUInt32LE(0x02014b50, 0);
      header.writeUInt16LE(20, 4); // made by: MS-DOS/FAT attributes, spec 2.0
      header.writeUInt16LE(20, 6);
      header.writeUInt16LE(flags, 8);
      header.writeUInt16LE(method, 10);
      header.writeUInt16LE(time, 12);
      header.writeUInt16LE(date, 14);
      header.writeUInt32LE(crc, 16);
      header.writeUInt32LE(body.length, 20);
      header.writeUInt32LE(data.length, 24);
      header.writeUInt16LE(name.length, 28);
      header.writeUInt32LE(isDir ? ATTR_DIRECTORY : 0, 38);
      header.writeUInt32LE(localOffset, 42);
      central.push(header, name);
    }
    const centralOffset = offset;
    for (const buffer of central) {
      write(buffer);
    }
    const centralSize = offset - centralOffset;
    if (offset > LIMIT_32) {
      throw new Error("archive exceeds 4 GiB, which needs ZIP64");
    }
    const end = Buffer.alloc(22);
    end.writeUInt32LE(0x06054b50, 0);
    end.writeUInt16LE(entries.length, 8);
    end.writeUInt16LE(entries.length, 10);
    end.writeUInt32LE(centralSize, 12);
    end.writeUInt32LE(centralOffset, 16);
    write(end);
  } catch (error) {
    closeSync(fd);
    rmSync(tmp, { force: true });
    throw error;
  }
  closeSync(fd);
  renameSync(tmp, outFile);
  return outFile;
}

export function parseArgs(argv) {
  const known = new Set(["--version", "--webview2", "--out-dir", "--exe", "--sidecar", "--target"]);
  const options = {};
  for (let i = 0; i < argv.length; i += 2) {
    const flag = argv[i];
    const value = argv[i + 1];
    if (!known.has(flag) || value === undefined || value.startsWith("--")) {
      throw new Error(`usage: package-portable.mjs [${[...known].map((f) => `${f} <value>`).join("] [")}]`);
    }
    options[flag.slice(2).replace(/-(\w)/g, (_, c) => c.toUpperCase())] = value;
  }
  return options;
}

function main() {
  const options = parseArgs(process.argv.slice(2));
  const triple = options.target ?? DEFAULT_TRIPLE;
  const found = options.exe && options.sidecar ? {} : findBuildOutputs(srcTauri, triple);
  const exe = resolve(options.exe ?? found.exe);
  const sidecar = resolve(options.sidecar ?? found.sidecar);
  const version = options.version ?? JSON.parse(readFileSync(join(srcTauri, "tauri.conf.json"), "utf8")).version;
  const webview2 = options.webview2 ? resolve(options.webview2) : undefined;
  const outDir = resolve(options.outDir ?? join(dirname(exe), "bundle", "portable"));
  // SOURCE_DATE_EPOCH makes the archive reproducible.
  const epoch = Number(process.env.SOURCE_DATE_EPOCH);
  const mtime = Number.isFinite(epoch) && epoch > 0 ? new Date(epoch * 1000) : new Date();

  const outFile = join(outDir, portableAssetName(version, { arch: archFromTriple(triple), offline: Boolean(webview2) }));
  const entries = planEntries({ exe, sidecar, version, webview2 });
  console.error(`packing ${basename(exe)} as "${APP_EXE}", ${basename(sidecar)} as ${SIDECAR_EXE}${webview2 ? `, WebView2 from ${webview2}` : ""}`);
  writeZip(entries, outFile, { mtime });
  console.log(outFile);
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  try {
    main();
  } catch (error) {
    console.error(error.message);
    process.exit(1);
  }
}
