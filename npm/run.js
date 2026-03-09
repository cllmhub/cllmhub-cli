#!/usr/bin/env node

const { execFileSync } = require("child_process");
const path = require("path");
const os = require("os");

const ext = os.platform() === "win32" ? ".exe" : "";
const binPath = path.join(__dirname, `cllmhub${ext}`);

try {
  execFileSync(binPath, process.argv.slice(2), { stdio: "inherit" });
} catch (err) {
  if (err.status !== undefined) {
    process.exit(err.status);
  }
  console.error(`Failed to run cllmhub: ${err.message}`);
  process.exit(1);
}
