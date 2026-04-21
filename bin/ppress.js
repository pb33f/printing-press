#!/usr/bin/env node

import { spawn } from "node:child_process";
import { existsSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const rootDir = join(dirname(fileURLToPath(import.meta.url)), "..");
const binaryName = process.platform === "win32" ? "ppress.exe" : "ppress";
const binaryPath = join(rootDir, "npm-install", "bin", binaryName);

if (!existsSync(binaryPath)) {
  console.error(
    "The printing-press binary is missing. Re-run `npm install -g @pb33f/printing-press` or `npm rebuild @pb33f/printing-press`.",
  );
  process.exit(1);
}

const child = spawn(binaryPath, process.argv.slice(2), {
  stdio: "inherit",
});

child.on("error", (error) => {
  console.error(`Failed to launch ${binaryName}: ${error.message}`);
  process.exit(1);
});

child.on("exit", (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal);
    return;
  }
  process.exit(code ?? 1);
});
