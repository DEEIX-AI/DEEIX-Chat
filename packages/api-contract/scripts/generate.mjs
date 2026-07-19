import { spawnSync } from "node:child_process";
import {
  copyFileSync,
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { createRequire } from "node:module";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const packageDir = dirname(dirname(fileURLToPath(import.meta.url)));
const workspaceDir = resolve(packageDir, "../..");
const backendDir = join(workspaceDir, "backend");
const committedSwaggerDir = join(backendDir, "docs");
const targetFile = join(packageDir, "src/types.generated.ts");
const outputName = "types.generated.ts";
const swaggerOutputNames = ["docs.go", "swagger.json", "swagger.yaml"];
const checkOnly = process.argv.includes("--check");
const unknownArguments = process.argv.slice(2).filter((argument) => argument !== "--check");

if (unknownArguments.length > 0) {
  throw new Error(`Unknown arguments: ${unknownArguments.join(", ")}`);
}

function shortDefinitionName(name) {
  return name.split(".").at(-1);
}

function namespaceName(name) {
  const namespace = name.slice(0, name.lastIndexOf("."));
  const httpMarker = "internal_transport_http_";
  const segment = namespace.includes(httpMarker)
    ? namespace.slice(namespace.lastIndexOf(httpMarker) + httpMarker.length)
    : namespace.split("_").at(-1);
  const aliases = {
    promptpreset: "PromptPreset",
    usersettings: "UserSettings",
  };

  if (aliases[segment]) {
    return aliases[segment];
  }

  return segment
    .split(/[^A-Za-z0-9]+/u)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join("");
}

function definitionNameMap(definitions) {
  const groups = new Map();

  for (const name of Object.keys(definitions)) {
    const shortName = shortDefinitionName(name);
    const names = groups.get(shortName) ?? [];
    names.push(name);
    groups.set(shortName, names);
  }

  const mapping = new Map();
  const generatedNames = new Set();

  for (const [shortName, names] of groups) {
    for (const name of names) {
      const generatedName = names.length === 1 ? shortName : `${namespaceName(name)}${shortName}`;
      if (generatedNames.has(generatedName)) {
        throw new Error(`Definition name collision after normalization: ${generatedName}`);
      }
      mapping.set(name, generatedName);
      generatedNames.add(generatedName);
    }
  }

  return mapping;
}

function rewriteReferences(value, mapping) {
  if (Array.isArray(value)) {
    return value.map((item) => rewriteReferences(item, mapping));
  }
  if (!value || typeof value !== "object") {
    return value;
  }

  const rewritten = {};
  for (const [key, item] of Object.entries(value)) {
    if (key === "$ref" && typeof item === "string" && item.startsWith("#/definitions/")) {
      const originalName = item.slice("#/definitions/".length);
      const generatedName = mapping.get(originalName);
      if (!generatedName) {
        throw new Error(`Unknown Swagger definition reference: ${originalName}`);
      }
      rewritten[key] = `#/definitions/${generatedName}`;
    } else {
      rewritten[key] = rewriteReferences(item, mapping);
    }
  }
  return rewritten;
}

function normalizeSwagger(swagger) {
  if (swagger.swagger !== "2.0") {
    throw new Error(`Expected Swagger 2.0, received ${swagger.swagger ?? "an unknown version"}`);
  }

  const definitions = swagger.definitions ?? {};
  const mapping = definitionNameMap(definitions);
  const normalizedDefinitions = {};

  const definitionNames = Object.keys(definitions).sort((left, right) => {
    const leftName = mapping.get(left);
    const rightName = mapping.get(right);
    return leftName < rightName ? -1 : leftName > rightName ? 1 : 0;
  });
  for (const originalName of definitionNames) {
    normalizedDefinitions[mapping.get(originalName)] = definitions[originalName];
  }

  return rewriteReferences({ ...swagger, definitions: normalizedDefinitions }, mapping);
}

function normalizeText(value) {
  return value.replaceAll("\r\n", "\n");
}

function rewriteGeneratedDefinitionNames(value, mapping) {
  let rewritten = value;
  const replacements = [...mapping.entries()]
    .filter(([originalName, generatedName]) => originalName !== generatedName)
    .sort(([left], [right]) => right.length - left.length);

  for (const [originalName, generatedName] of replacements) {
    rewritten = rewritten.replaceAll(originalName, generatedName);
  }
  return rewritten;
}

function replaceDocsTemplateDefinitions(value, definitions) {
  const marker = '    "definitions": ';
  const propertyStart = value.indexOf(marker);
  if (propertyStart < 0) {
    throw new Error("Unable to locate definitions in generated docs.go");
  }
  const valueStart = propertyStart + marker.length;
  if (value[valueStart] !== "{") {
    throw new Error("Expected generated docs.go definitions to be an object");
  }

  let depth = 0;
  let inString = false;
  let escaped = false;
  let valueEnd = -1;
  for (let index = valueStart; index < value.length; index += 1) {
    const character = value[index];
    if (inString) {
      if (escaped) {
        escaped = false;
      } else if (character === "\\") {
        escaped = true;
      } else if (character === '"') {
        inString = false;
      }
      continue;
    }
    if (character === '"') {
      inString = true;
      continue;
    }
    if (character === "{") {
      depth += 1;
    } else if (character === "}") {
      depth -= 1;
      if (depth === 0) {
        valueEnd = index + 1;
        break;
      }
    }
  }
  if (valueEnd < 0) {
    throw new Error("Unable to parse generated docs.go definitions object");
  }

  const formattedDefinitions = JSON.stringify(definitions, null, 4)
    .split("\n")
    .map((line, index) => (index === 0 ? `${marker}${line}` : `    ${line}`))
    .join("\n");
  return `${value.slice(0, propertyStart)}${formattedDefinitions}${value.slice(valueEnd)}`;
}

function sortYAMLDefinitions(value) {
  const marker = "definitions:\n";
  const sectionStart = value.indexOf(marker);
  if (sectionStart < 0) {
    throw new Error("Unable to locate definitions in generated Swagger YAML");
  }
  const bodyStart = sectionStart + marker.length;
  const nextSection = value.slice(bodyStart).search(/^\S/mu);
  const bodyEnd = nextSection < 0 ? value.length : bodyStart + nextSection;
  const body = value.slice(bodyStart, bodyEnd);
  const blockStarts = [...body.matchAll(/^ {2}([^\s][^:]*):\n/gmu)];
  if (blockStarts.length === 0) {
    return value;
  }

  const blocks = blockStarts.map((match, index) => {
    const start = match.index;
    const end = blockStarts[index + 1]?.index ?? body.length;
    return { name: match[1], content: body.slice(start, end) };
  });
  blocks.sort((left, right) => (left.name < right.name ? -1 : left.name > right.name ? 1 : 0));
  return `${value.slice(0, bodyStart)}${blocks.map((block) => block.content).join("")}${value.slice(bodyEnd)}`;
}

function runCommand(executable, arguments_, cwd) {
  const result = spawnSync(executable, arguments_, {
    cwd,
    encoding: "utf8",
    stdio: "inherit",
  });

  if (result.error) {
    throw result.error;
  }
  if (result.status !== 0) {
    throw new Error(`${executable} exited with status ${result.status}`);
  }
}

function syncVersions() {
  const arguments_ = [join(workspaceDir, "scripts/sync-version.mjs")];
  if (checkOnly) {
    arguments_.push("--check");
  }
  arguments_.push("all");
  runCommand(process.execPath, arguments_, workspaceDir);
}

function generateSwagger(outputDir) {
  const goExecutable = process.platform === "win32" ? "go.exe" : "go";
  runCommand(
    goExecutable,
    [
      "tool",
      "swag",
      "init",
      "-g",
      "cmd/server/main.go",
      "-o",
      outputDir,
      "--packageName",
      "docs",
      "--parseDependency",
      "--parseInternal",
      "--requiredByDefault",
      "--quiet",
    ],
    backendDir,
  );
}

function normalizeGeneratedSwagger(outputDir) {
  const docsFile = join(outputDir, "docs.go");
  const jsonFile = join(outputDir, "swagger.json");
  const yamlFile = join(outputDir, "swagger.yaml");
  const swagger = JSON.parse(readFileSync(jsonFile, "utf8"));
  const version = swagger.info?.version;
  const mapping = definitionNameMap(swagger.definitions ?? {});
  const normalizedSwagger = normalizeSwagger(swagger);

  writeFileSync(jsonFile, `${JSON.stringify(normalizedSwagger, null, 4)}\n`);
  const docs = rewriteGeneratedDefinitionNames(readFileSync(docsFile, "utf8"), mapping);
  writeFileSync(
    docsFile,
    replaceDocsTemplateDefinitions(docs, normalizedSwagger.definitions ?? {}),
  );

  let yaml = sortYAMLDefinitions(
    rewriteGeneratedDefinitionNames(readFileSync(yamlFile, "utf8"), mapping),
  );
  if (typeof version === "string" && version !== "") {
    const normalizedYaml = yaml.replace(/^ {2}version: .+$/mu, `  version: "${version}"`);
    if (normalizedYaml === yaml && !yaml.includes(`  version: "${version}"`)) {
      throw new Error("Unable to normalize generated Swagger YAML version");
    }
    yaml = normalizedYaml;
  }
  writeFileSync(yamlFile, yaml);
}

function swaggerTypescriptCLI() {
  const require = createRequire(import.meta.url);
  const packageJSONFile = require.resolve("swagger-typescript-api/package.json");
  const packageJSON = JSON.parse(readFileSync(packageJSONFile, "utf8"));
  const binPath = packageJSON.bin?.["swagger-typescript-api"];
  if (typeof binPath !== "string" || binPath.trim() === "") {
    throw new Error("Unable to resolve swagger-typescript-api CLI entrypoint");
  }
  return resolve(dirname(packageJSONFile), binPath);
}

function assertGeneratedFilesMatch(expectedFiles) {
  const staleFiles = [];
  for (const [committedFile, generatedFile] of expectedFiles) {
    if (!existsSync(committedFile)) {
      staleFiles.push({ committedFile, detail: "committed file is missing" });
      continue;
    }
    const committed = normalizeText(readFileSync(committedFile, "utf8"));
    const expected = normalizeText(readFileSync(generatedFile, "utf8"));
    if (committed !== expected) {
      const committedLines = committed.split("\n");
      const expectedLines = expected.split("\n");
      const lineCount = Math.max(committedLines.length, expectedLines.length);
      let detail = "content differs";
      for (let index = 0; index < lineCount; index += 1) {
        if (committedLines[index] === expectedLines[index]) {
          continue;
        }
        const committedLine = (committedLines[index] ?? "<missing>").slice(0, 160);
        const expectedLine = (expectedLines[index] ?? "<missing>").slice(0, 160);
        detail = `line ${index + 1}: committed ${JSON.stringify(committedLine)}, generated ${JSON.stringify(expectedLine)}`;
        break;
      }
      staleFiles.push({ committedFile, detail });
    }
  }

  if (staleFiles.length > 0) {
    const relativeFiles = staleFiles.map(
      ({ committedFile, detail }) => `- ${committedFile.slice(workspaceDir.length + 1)} (${detail})`,
    );
    throw new Error(
      [
        "Generated API contract is stale:",
        ...relativeFiles,
        "Run `pnpm api:generate` from the workspace root and commit the result.",
      ].join("\n"),
    );
  }
}

const temporaryDir = mkdtempSync(join(tmpdir(), "deeix-api-contract-"));

try {
  syncVersions();

  const generatedSwaggerDir = join(temporaryDir, "swagger");
  const normalizedSwaggerFile = join(temporaryDir, "swagger.normalized.json");
  const generatedDir = join(temporaryDir, "generated");
  const generatedFile = join(generatedDir, outputName);
  generateSwagger(generatedSwaggerDir);
  normalizeGeneratedSwagger(generatedSwaggerDir);

  const generatedSwaggerFile = join(generatedSwaggerDir, "swagger.json");
  const swagger = JSON.parse(readFileSync(generatedSwaggerFile, "utf8"));

  mkdirSync(generatedDir, { recursive: true });
  writeFileSync(normalizedSwaggerFile, `${JSON.stringify(normalizeSwagger(swagger), null, 2)}\n`);

  runCommand(
    process.execPath,
    [
      swaggerTypescriptCLI(),
      "generate",
      "--path",
      normalizedSwaggerFile,
      "--output",
      generatedDir,
      "--name",
      outputName,
      "--no-client",
      "--route-types",
      "--sort-types",
      "--silent",
    ],
    packageDir,
  );

  const generated = normalizeText(readFileSync(generatedFile, "utf8")).replace("// @ts-nocheck\n", "");
  const banner = [
    "// Generated from backend/docs/swagger.json. Do not edit manually.",
    "// Run `pnpm api:generate` from the workspace root to regenerate.",
    "",
  ].join("\n");
  writeFileSync(generatedFile, `${banner}${generated}`);

  if (checkOnly) {
    assertGeneratedFilesMatch([
      ...swaggerOutputNames.map((name) => [join(committedSwaggerDir, name), join(generatedSwaggerDir, name)]),
      [targetFile, generatedFile],
    ]);
    console.log("Swagger and TypeScript API contracts are up to date.");
  } else {
    for (const name of swaggerOutputNames) {
      copyFileSync(join(generatedSwaggerDir, name), join(committedSwaggerDir, name));
    }
    mkdirSync(dirname(targetFile), { recursive: true });
    copyFileSync(generatedFile, targetFile);
    console.log("Generated backend Swagger and TypeScript API contracts.");
  }
} finally {
  rmSync(temporaryDir, { force: true, recursive: true });
}
