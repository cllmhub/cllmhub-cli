#!/usr/bin/env node

const { execSync } = require("child_process");
const fs = require("fs");
const https = require("https");
const path = require("path");
const os = require("os");

const REPO = "cllmhub/cllmhub-cli";
const BINARY = "cllmhub";

function getPlatform() {
  const platform = os.platform();
  const platformMap = { darwin: "darwin", linux: "linux", win32: "windows" };
  const mapped = platformMap[platform];
  if (!mapped) {
    console.error(`Unsupported platform: ${platform}`);
    process.exit(1);
  }
  return mapped;
}

function getArch() {
  const arch = os.arch();
  const archMap = { x64: "amd64", arm64: "arm64" };
  const mapped = archMap[arch];
  if (!mapped) {
    console.error(`Unsupported architecture: ${arch}`);
    process.exit(1);
  }
  return mapped;
}

function getVersion() {
  const pkg = require("./package.json");
  return `v${pkg.version}`;
}

function getBinaryName(platform, arch) {
  const name = `${BINARY}-${platform}-${arch}`;
  return platform === "windows" ? `${name}.exe` : name;
}

function getInstallPath() {
  const ext = os.platform() === "win32" ? ".exe" : "";
  return path.join(__dirname, `${BINARY}${ext}`);
}

function download(url) {
  return new Promise((resolve, reject) => {
    https
      .get(url, (res) => {
        if (res.statusCode === 302 || res.statusCode === 301) {
          return download(res.headers.location).then(resolve).catch(reject);
        }
        if (res.statusCode !== 200) {
          return reject(new Error(`Download failed: HTTP ${res.statusCode}`));
        }
        const chunks = [];
        res.on("data", (chunk) => chunks.push(chunk));
        res.on("end", () => resolve(Buffer.concat(chunks)));
        res.on("error", reject);
      })
      .on("error", reject);
  });
}

async function main() {
  const platform = getPlatform();
  const arch = getArch();
  const version = getVersion();
  const binaryName = getBinaryName(platform, arch);
  const installPath = getInstallPath();

  const url = `https://github.com/${REPO}/releases/download/${version}/${binaryName}`;

  console.log(`Downloading ${BINARY} ${version} for ${platform}/${arch}...`);

  try {
    const data = await download(url);
    fs.writeFileSync(installPath, data);
    fs.chmodSync(installPath, 0o755);
    console.log(`Installed ${BINARY} to ${installPath}`);
  } catch (err) {
    console.error(`Failed to download ${BINARY}: ${err.message}`);
    process.exit(1);
  }
}

main();
