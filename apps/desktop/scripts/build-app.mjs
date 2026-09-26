// Fast local build: the .app only, with LTO relaxed for link time. The shipped
// profile is `pnpm build` / `pnpm build:signed` (see Cargo.toml).
import { spawnSync } from "node:child_process";

const result = spawnSync("pnpm", ["tauri", "build", "--bundles", "app"], {
  stdio: "inherit",
  env: {
    ...process.env,
    CARGO_PROFILE_RELEASE_LTO: "thin",
    CARGO_PROFILE_RELEASE_CODEGEN_UNITS: "16",
    CARGO_PROFILE_RELEASE_OPT_LEVEL: "2",
  },
});
process.exit(result.status ?? 1);
