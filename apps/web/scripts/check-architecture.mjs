// Architecture rules that lint cannot express. Run by `pnpm check`.
//
// Each rule is a boundary decided in docs/ARCHITECTURE.md; a violation means a
// platform concern leaked into shared code, not a style problem.

import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import { join, posix, relative, sep } from "node:path";
import { fileURLToPath } from "node:url";

// fileURLToPath, not URL.pathname: the latter yields "/E:/..." on Windows.
const root = fileURLToPath(new URL("..", import.meta.url));

// Naming domain of each feature directory: business collections are plural
// directories with a singular domain (`files` -> `file`), capability and UI areas
// are singular or uncountable. `settings` keeps its plural: it is the product term.
// Hooks are named `use-<domain>-<purpose>.ts` and export `use<Domain><Purpose>`.
// A new feature must be registered here before the naming rules accept it.
const FEATURE_DOMAINS = {
  admin: "admin",
  announcements: "announcement",
  auth: "auth",
  chat: "chat",
  desktop: "desktop",
  files: "file",
  "knowledge-bases": "knowledge-base",
  library: "library",
  recent: "recent",
  settings: "settings",
  share: "share",
  shell: "shell",
};

// Admin hooks are `use-admin-<section>-<purpose>.ts`. These serve several sections
// (or the admin shell itself) and so carry no section segment.
const ADMIN_CROSS_SECTION_HOOKS = new Set(["use-admin-circuit-breaker", "use-admin-provider-bridge-notice"]);

const ADMIN_ROUTE_DIR = "app/(app)/(project)/admin";
const ADMIN_SECTIONS_DIR = "features/admin/components/sections";

const rules = [
  {
    // Only the platform layer may talk to the Tauri shell. Everything else goes
    // through its typed bindings, so the webview's native surface stays auditable.
    name: "Tauri APIs only under shared/platform/",
    match: (file) => file.imports.some((specifier) => specifier.startsWith("@tauri-apps/")),
    allow: (file) => file.path.startsWith("shared/platform/"),
  },
  {
    // The refresh token is write-once from JS. Any read path is a regression of
    // the desktop credential model (docs/ARCHITECTURE.md §5).
    name: "no refresh-token read command",
    match: (file) => /invoke\(\s*["']read_refresh_token["']/.test(file.code),
    allow: () => false,
  },
  {
    // Server address is pinned in the shell; the web app must not keep its own copy.
    name: "no localStorage-backed server address",
    match: (file) => /localStorage\.(get|set)Item\(\s*["'][^"']*api-base-url/.test(file.code),
    allow: () => false,
  },
  {
    // What the UI offers is decided by the server's capability flags, never by
    // the platform the client runs on (docs/ARCHITECTURE.md §4). The
    // admin and settings surfaces are pure feature surfaces, so a platform check
    // there can only be doing visibility work.
    name: "feature visibility comes from capabilities, not the platform",
    match: (file) =>
      /\bisDesktopApp\(\)/.test(file.code) &&
      (file.path.startsWith("features/admin/") ||
        file.path.startsWith("features/settings/") ||
        file.resolvedImports.includes("shared/capabilities")),
    allow: () => false,
  },
  {
    // Every admin section page must be registered in ADMIN_SECTIONS: that table
    // drives both the sidebar and the route guard, so an unregistered section
    // would bypass capability gating.
    name: "admin section registered in ADMIN_SECTIONS",
    match: (file) => /^app\/\(app\)\/\(project\)\/admin\/[^/]+\/page\.tsx$/.test(file.path),
    allow: (file) => {
      const section = file.path.split("/").at(-2);
      const table = files.find((f) => f.path === "features/admin/model/admin-sections.ts");
      return table !== undefined && new RegExp(`href:\\s*["']/${section}["']`).test(table.source);
    },
  },
  {
    // An admin section has one name everywhere: its route segment
    // (/admin/<section>), its ADMIN_SECTIONS id and its component directory
    // (sections/<section>/ with the entry admin-<section>.tsx).
    name: "admin section dir matches route",
    match: (file) =>
      /^app\/\(app\)\/\(project\)\/admin\/[^/]+\/page\.tsx$/.test(file.path) ||
      file.path.startsWith(`${ADMIN_SECTIONS_DIR}/`),
    allow: (file) => {
      if (file.path.startsWith(`${ADMIN_SECTIONS_DIR}/`)) {
        const section = file.path.split("/")[4];
        return existsSync(join(root, ADMIN_ROUTE_DIR, section, "page.tsx"));
      }
      const section = file.path.split("/").at(-2);
      const table = files.find((f) => f.path === "features/admin/model/admin-sections.ts");
      return (
        existsSync(join(root, ADMIN_SECTIONS_DIR, section, `admin-${section}.tsx`)) &&
        table !== undefined &&
        new RegExp(`id:\\s*["']${section}["'][^}]*href:\\s*["']/${section}["']`).test(table.source)
      );
    },
  },
  {
    // Hooks are `use-<domain>-<purpose>.ts` (FEATURE_DOMAINS; an entity's domain is
    // its directory name) and every exported hook starts with `use<Domain>`, so a
    // hook's owner is readable from both its file name and its identifier. Admin
    // hooks also name their section: `use-admin-<section>-<purpose>.ts` exporting
    // `useAdmin<Section>...`. shared/ hooks are domain-free and exempt.
    name: "hook file naming",
    match: (file) => /^(features|entities)\/[^/]+\/hooks\/[^/]+\.tsx?$/.test(file.path),
    allow: (file) => hookNamingProblems(file).length === 0,
    detail: (file) => hookNamingProblems(file).join("; "),
  },
  {
    // Files inside a section directory carry the section name as prefix
    // (`tools/tools-mcp-order-sheet.tsx`), except the section entry
    // `<domain>-<section>.tsx` (`admin-users.tsx`, `settings-account.tsx`).
    // Components shared by several sections live in components/shared/.
    name: "section file prefix",
    match: (file) => /^features\/[^/]+\/components\/sections\/[^/]+\/[^/]+$/.test(file.path),
    allow: (file) => {
      const [, feature, , , section, name] = file.path.split("/");
      const base = name.replace(/\.tsx?$/, "");
      return base.startsWith(`${section}-`) || base === `${FEATURE_DOMAINS[feature]}-${section}`;
    },
  },
  {
    // Every feature exposes a public entry point (index.ts); other features may
    // only import through it (see "no deep cross-feature imports").
    name: "feature has public index",
    check: () =>
      readdirSync(join(root, "features"))
        .filter((name) => statSync(join(root, "features", name)).isDirectory())
        .filter((name) => !existsSync(join(root, "features", name, "index.ts")))
        .map((name) => `features/${name}/index.ts is missing`),
  },
  {
    // A feature reaches another feature only through its public entry point
    // (`@/features/<name>`, i.e. its index.ts), so internals can move freely.
    // Route files under app/ follow the same rule: they mount feature entry components
    // through the feature's index.ts. With package.json `sideEffects`, unused re-exports
    // are tree-shaken, so a route bundles only the entry it mounts.
    name: "no deep cross-feature imports",
    // Relative paths are resolved too, so `../../<other-feature>/...` cannot slip through.
    match: (file) => {
      const own = file.path.startsWith("app/") ? "" : /^features\/([^/]+)\//.exec(file.path)?.[1];
      if (own === undefined) return false;
      return file.resolvedImports.some((target) => {
        const match = /^features\/([^/]+)\/(.+)$/.exec(target);
        return match !== null && match[1] !== own && match[2] !== "index";
      });
    },
    allow: () => false,
  },
  {
    // components/ui is a design-system layer; business code depends on it, never
    // the other way round.
    name: "components/ui has no business dependencies",
    match: (file) =>
      file.path.startsWith("components/ui/") &&
      file.resolvedImports.some((target) => /^(features|entities)(\/|$)/.test(target)),
    allow: () => false,
  },
  {
    // entities sit below features: they are shared by several features and must
    // not reach back up into any of them.
    name: "entities do not depend on features",
    match: (file) =>
      file.path.startsWith("entities/") && file.resolvedImports.some((target) => /^features(\/|$)/.test(target)),
    allow: () => false,
  },
  {
    // shared/ is the domain-free base layer (docs: Shared boundary). Business
    // semantics live in entities/ or features/, so shared code never imports them;
    // a generic shared module that needs business behaviour receives it via props.
    name: "shared does not depend on entities or features",
    match: (file) =>
      file.path.startsWith("shared/") &&
      file.resolvedImports.some((target) => /^(entities|features)(\/|$)/.test(target)),
    allow: () => false,
  },
  {
    // Every entity exposes exactly one public entry point, its index.ts.
    name: "entity has public index",
    check: () =>
      readdirSync(join(root, "entities"))
        .filter((name) => statSync(join(root, "entities", name)).isDirectory())
        .filter((name) => !existsSync(join(root, "entities", name, "index.ts")))
        .map((name) => `entities/${name}/index.ts is missing`),
  },
  {
    // Code outside an entity (app/, features/, other entities) reaches it only through
    // `@/entities/<name>`. There are no subpath entries: package.json declares
    // "sideEffects", so unused re-exports are tree-shaken and one entry costs no
    // bundle size. Lazy UI is exported pre-wrapped (e.g. LazyFilePreviewDialog)
    // rather than `import()`-ing the entry, which would load the whole entity.
    name: "entities imported only via their public entry",
    match: (file) => deepEntityImports(file).length > 0,
    allow: () => false,
    detail: (file) => deepEntityImports(file).join(", "),
  },
  {
    // package.json "sideEffects" lists stylesheets plus explicitly named files with
    // top-level effects (instrumentation-client.ts): that is what lets the bundler drop
    // unused re-exports of every index.ts (feature and entity entries). Without it, one
    // imported symbol puts a whole entity or feature barrel into the chunk.
    name: "package.json declares sideEffects",
    check: () => {
      const { sideEffects } = JSON.parse(readFileSync(join(root, "package.json"), "utf8"));
      if (!Array.isArray(sideEffects)) return ['package.json must declare "sideEffects" as a list'];
      return sideEffects
        .filter((pattern) => !/\.css$/.test(pattern) && !(pattern.startsWith("./") && existsSync(join(root, pattern))))
        .map((pattern) => `sideEffects entry "${pattern}" is neither a *.css pattern nor an existing ./file`);
    },
  },
  {
    // Under that declaration a bare `import "./module"` of a script is dropped by the
    // bundler, so side-effect-only imports are reserved for stylesheets.
    name: "side-effect imports only for stylesheets",
    match: (file) => sideEffectScriptImports(file).length > 0,
    allow: () => false,
    detail: (file) => sideEffectScriptImports(file).join(", "),
  },
  {
    // Inside an entity, its own files import each other by path; going through its
    // own index.ts would create an import cycle.
    name: "entity does not import its own public entry",
    match: (file) => {
      const own = /^entities\/([^/]+)\//.exec(file.path)?.[1];
      return own !== undefined && file.path !== `entities/${own}/index.ts` && file.resolvedImports.includes(`entities/${own}`);
    },
    allow: () => false,
  },
  {
    // Entities may build on each other (conversation -> file), but the dependency
    // graph between entities stays acyclic so each one can be understood bottom-up.
    name: "no import cycles between entities",
    check: () => entityCycles(),
  },
  {
    // Route pages and layouts stay server components so the static export keeps
    // client boundaries inside features; interactive logic belongs to a feature
    // entry component. The image-loading preview is a standalone visual playground.
    // Comments are stripped first: a license or doc comment may precede the directive.
    name: "route pages are server components",
    match: (file) => /^app\/(.+\/)?(page|layout)\.tsx$/.test(file.path) && /^\s*["']use client["']/.test(file.code),
    allow: (file) => file.path === "app/(app)/(project)/preview/image-loading/page.tsx",
  },
  {
    // File names are kebab-case with no extra dot segments (`admin-types.ts`, not
    // `admin.types.ts`). Tool-recognized suffixes (`*.test.ts`, `*.spec.tsx`,
    // `*.config.ts`) are the only allowed dot segments.
    name: "kebab-case file name without dot segments",
    match: (file) =>
      !/^[a-z0-9]+(-[a-z0-9]+)*(\.(test|spec))?\.tsx?$|^[a-z0-9]+(-[a-z0-9]+)*\.config\.ts$/.test(
        file.path.split("/").at(-1),
      ),
    allow: () => false,
  },
  {
    // Components render; hooks own requests, loading/error state and toasts. A component
    // value-importing a request module is a request call living in the UI layer. Type-only
    // imports and the URL/error helpers of http-client are not requests.
    name: "components do not call request functions",
    match: (file) => file.path.endsWith(".tsx") && requestValueImports(file).length > 0,
    allow: () => false,
  },
  {
    // `unknown` data (API payloads, storage, events, JSON) is narrowed with guards from
    // shared/lib/type-guards, not asserted. The few assertions a library typing gap forces
    // must carry a `// Type assertion: <reason>` comment on the same or preceding line.
    // Vendored registry code (animate-ui, reactbits) is exempt.
    name: "no unexplained type assertions",
    match: (file) =>
      !/^components\/(animate-ui|reactbits)\//.test(file.path) && unexplainedAssertionLines(file).length > 0,
    allow: () => false,
    detail: (file) => `line ${unexplainedAssertionLines(file).join(", ")}`,
  },
];

// `@/entities/<name>/...` targets of a file outside that entity; empty when it complies.
function deepEntityImports(file) {
  const own = /^entities\/([^/]+)\//.exec(file.path)?.[1];
  return file.resolvedImports.filter((target) => {
    const match = /^entities\/([^/]+)\/(.+)$/.exec(target);
    return match !== null && match[1] !== own;
  });
}

// Bare `import "x"` specifiers of a file that are not stylesheets.
function sideEffectScriptImports(file) {
  return [...file.code.matchAll(/^\s*import\s*(["'])([^"'\n]+)\1/gm)]
    .map((match) => match[2])
    .filter((specifier) => !/\.css$/.test(specifier));
}

// Cycles in the entity -> entity import graph, as "a -> b -> a" strings.
function entityCycles() {
  const edges = new Map();
  for (const file of files) {
    const own = /^entities\/([^/]+)\//.exec(file.path)?.[1];
    if (own === undefined) continue;
    for (const target of file.resolvedImports) {
      const other = /^entities\/([^/]+)/.exec(target)?.[1];
      if (other === undefined || other === own) continue;
      if (!edges.has(own)) edges.set(own, new Set());
      edges.get(own).add(other);
    }
  }
  const cycles = new Set();
  const visit = (node, path) => {
    const start = path.indexOf(node);
    if (start !== -1) {
      cycles.add([...path.slice(start), node].join(" -> "));
      return;
    }
    for (const next of edges.get(node) ?? []) visit(next, [...path, node]);
  };
  for (const node of edges.keys()) visit(node, []);
  // Each cycle is found once per member; report it once, starting from its smallest name.
  const unique = new Map();
  for (const cycle of cycles) {
    const members = cycle.split(" -> ").slice(0, -1);
    const key = [...members].sort().join(",");
    if (!unique.has(key)) unique.set(key, cycle);
  }
  return [...unique.values()].map((cycle) => `entity import cycle ${cycle}`);
}

// "knowledge-base" -> "KnowledgeBase".
function pascalCase(kebab) {
  return kebab
    .split("-")
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join("");
}

// Naming problems of a features/<f>/hooks/ or entities/<e>/hooks/ file; empty when it complies.
function hookNamingProblems(file) {
  const [layer, owner, , name] = file.path.split("/");
  const base = name.replace(/\.tsx?$/, "");
  const domain = layer === "entities" ? owner : FEATURE_DOMAINS[owner];
  if (domain === undefined) return [`feature "${owner}" is not registered in FEATURE_DOMAINS`];
  let prefix = `use-${domain}`;
  if (owner === "admin" && layer === "features" && !ADMIN_CROSS_SECTION_HOOKS.has(base)) {
    const sections = readdirSync(join(root, ADMIN_SECTIONS_DIR));
    const section = sections
      .filter((candidate) => base === `use-admin-${candidate}` || base.startsWith(`use-admin-${candidate}-`))
      .sort((a, b) => b.length - a.length)[0];
    if (section === undefined) return [`expected use-admin-<section>-<purpose>, <section> one of ${sections.join(", ")}`];
    prefix = `use-admin-${section}`;
  }
  if (base !== prefix && !base.startsWith(`${prefix}-`)) return [`expected ${prefix}-<purpose>`];
  const identifier = `use${pascalCase(prefix.slice("use-".length))}`;
  const exported = [...file.code.matchAll(/\bexport\s+(?:async\s+)?(?:function\s+|const\s+)(use[A-Z]\w*)/g)].map((m) => m[1]);
  return exported.filter((hook) => !hook.startsWith(identifier)).map((hook) => `${hook} should start with ${identifier}`);
}

function walk(dir, out = []) {
  for (const entry of readdirSync(dir)) {
    if (entry === "node_modules" || entry === ".next" || entry === "out" || entry.startsWith(".")) continue;
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) walk(full, out);
    else if (/\.(ts|tsx)$/.test(entry) && !entry.endsWith(".d.ts")) out.push(full);
  }
  return out;
}

// Removes // and /* */ comments while keeping string and template literals intact, so
// import-like text inside comments does not trigger rules. Outside strings a backslash only
// occurs in regex literals, where the escaped character is skipped so `\/*` is not a comment.
function stripComments(source) {
  let out = "";
  let quote = null;
  for (let i = 0; i < source.length; i++) {
    const char = source[i];
    const next = source[i + 1];
    if (char === "\\") {
      out += char + (next ?? "");
      i++;
      continue;
    }
    if (quote) {
      if (char === quote || (char === "\n" && quote !== "`")) quote = null;
      out += char;
      continue;
    }
    if (char === '"' || char === "'" || char === "`") {
      quote = char;
      out += char;
      continue;
    }
    if (char === "/" && next === "/") {
      while (i < source.length && source[i] !== "\n") i++;
      out += "\n";
      continue;
    }
    if (char === "/" && next === "*") {
      const end = source.indexOf("*/", i + 2);
      const comment = end === -1 ? source.slice(i) : source.slice(i, end + 2);
      // Keep line breaks so `^` anchors and line-based checks still line up.
      out += comment.replace(/[^\n]/g, "");
      i = end === -1 ? source.length : end + 1;
      continue;
    }
    out += char;
  }
  return out;
}

// Static, side-effect, dynamic and re-export specifiers: `from "x"`, `import "x"`,
// `import("x")`, `require("x")`.
function collectImports(code) {
  const specifiers = [];
  for (const match of code.matchAll(/(?:\bfrom\s*|\bimport\s*\(\s*|\bimport\s+|\brequire\s*\(\s*)(["'])([^"'\n]+)\1/g)) {
    specifiers.push(match[2]);
  }
  return specifiers;
}

// Maps `@/x` and relative specifiers to root-relative paths without extension; bare package
// specifiers are dropped.
function resolveImport(filePath, specifier) {
  let target;
  if (specifier.startsWith("@/")) target = specifier.slice(2);
  else if (specifier.startsWith("./") || specifier.startsWith("../")) {
    target = posix.normalize(posix.join(posix.dirname(filePath), specifier));
  } else return null;
  return target.replace(/\/$/, "").replace(/\.(tsx?|mjs|js)$/, "");
}

// Request modules: every non-types module under shared/api and features/<x>/api.
function isRequestModule(target) {
  return (
    (/^shared\/api\/[^/]+$/.test(target) || /^features\/[^/]+\/api(\/[^/]+)?$/.test(target)) &&
    !/-types$/.test(target)
  );
}

// Value exports of request modules that are not requests: URL builders and error classes.
const NON_REQUEST_EXPORTS = new Set(["ApiError", "ApiNetworkError", "resolveApiBaseURL"]);

// Named value imports a file takes from request modules (type-only imports excluded).
function requestValueImports(file) {
  const names = [];
  for (const match of file.code.matchAll(/\bimport\s+(type\s+)?([^;]*?)\s+from\s*(["'])([^"'\n]+)\3/g)) {
    if (match[1]) continue;
    const target = resolveImport(file.path, match[4]);
    if (target === null || !isRequestModule(target)) continue;
    const clause = match[2];
    const braces = /\{([^}]*)\}/.exec(clause);
    const defaultOrNamespace = clause.replace(/\{[^}]*\}/, "").replace(/,/g, "").trim();
    if (defaultOrNamespace !== "") names.push(defaultOrNamespace);
    for (const raw of braces?.[1].split(",") ?? []) {
      const name = raw.trim().split(/\s+as\s+/)[0];
      if (name !== "" && !name.startsWith("type ") && !NON_REQUEST_EXPORTS.has(name)) names.push(name);
    }
  }
  return names;
}

// Blanks string and template literal contents (keeping newlines) so prose such as
// "such as /chat" is not mistaken for an assertion. Runs on comment-stripped code.
function blankStrings(code) {
  let out = "";
  let quote = null;
  for (let i = 0; i < code.length; i++) {
    const char = code[i];
    if (quote) {
      if (char === "\\") {
        out += "  ";
        i++;
        continue;
      }
      if (char === quote || (char === "\n" && quote !== "`")) {
        quote = null;
        out += char;
        continue;
      }
      out += char === "\n" ? "\n" : " ";
      continue;
    }
    if (char === '"' || char === "'" || char === "`") quote = char;
    out += char;
  }
  return out;
}

// 1-based line numbers of `x as T` assertions that are not `as const`, not import/export
// aliases (`{ a as b }`, `* as ns`) and not explained by a `// Type assertion:` comment.
function unexplainedAssertionLines(file) {
  const code = blankStrings(file.code)
    .replace(/\b(import|export)(\s+type)?\s*\{[^}]*\}/g, (m) => m.replace(/[^\n]/g, " "))
    .replace(/\*\s+as\s+[A-Za-z_$][\w$]*/g, (m) => m.replace(/[^\n]/g, " "));
  const sourceLines = file.source.split("\n");
  const lines = [];
  for (const match of code.matchAll(/(?<![\w$.])as\s+(?!const\b)(?=[A-Za-z_$({[`"'])/g)) {
    const line = code.slice(0, match.index).split("\n").length;
    // The explanation may be a trailing comment or anywhere in the comment block right above.
    let explained = (sourceLines[line - 1] ?? "").includes("Type assertion:");
    for (let i = line - 2; !explained && i >= 0 && /^\s*\/\//.test(sourceLines[i]); i--) {
      explained = sourceLines[i].includes("Type assertion:");
    }
    if (!explained) lines.push(line);
  }
  return [...new Set(lines)];
}

const files = walk(root).map((full) => {
  const path = relative(root, full).split(sep).join("/");
  const source = readFileSync(full, "utf8");
  const code = stripComments(source);
  const imports = collectImports(code);
  const resolvedImports = imports.map((specifier) => resolveImport(path, specifier)).filter((target) => target !== null);
  return { path, source, code, imports, resolvedImports };
});

const violations = [];
for (const rule of rules) {
  // Project-level rules inspect the tree once instead of file by file.
  if (rule.check) {
    for (const problem of rule.check()) violations.push(`${rule.name}: ${problem}`);
    continue;
  }
  for (const file of files) {
    if (rule.match(file) && !rule.allow(file)) {
      violations.push(`${rule.name}: ${file.path}${rule.detail ? ` (${rule.detail(file)})` : ""}`);
    }
  }
}

if (violations.length > 0) {
  console.error("Architecture violations:");
  for (const v of violations) console.error(`  - ${v}`);
  process.exit(1);
}
console.log(`Architecture rules OK (${rules.length} rules, ${files.length} files)`);
for (const rule of rules) console.log(`  - ${rule.name}`);
