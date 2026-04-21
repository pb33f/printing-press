import { createHash } from "node:crypto";
import { existsSync } from "node:fs";
import { chmodSync, createWriteStream } from "node:fs";
import * as fs from "node:fs/promises";
import { tmpdir } from "node:os";
import { basename, dirname, join } from "node:path";
import { Readable } from "node:stream";
import { pipeline } from "node:stream/promises";
import { fileURLToPath } from "node:url";

import AdmZip from "adm-zip";
import * as tar from "tar";

import {
  ARCH_MAPPING,
  CONFIG,
  EXTENSION_MAPPING,
  PLATFORM_MAPPING,
} from "./config.js";

const rootDir = join(dirname(fileURLToPath(import.meta.url)), "..");

function fail(message) {
  throw new Error(message);
}

function normalizeVersion(version) {
  return String(version || "").trim().replace(/^v/, "");
}

function resolveVersion(packageJson) {
  const requested = process.env.PP_INSTALL_VERSION || packageJson.version;
  const normalized = normalizeVersion(requested);
  if (!normalized) {
    fail("Missing version in package.json");
  }
  return normalized;
}

function detectPlatform() {
  const mapped = PLATFORM_MAPPING[process.platform];
  if (!mapped) {
    fail(
      `${process.platform} is not supported by this installer. Please open an issue: ${CONFIG.issueUrl}`,
    );
  }
  return mapped;
}

function detectArch() {
  const mapped = ARCH_MAPPING[process.arch];
  if (!mapped) {
    fail(
      `${process.arch} is not supported by this installer. Please open an issue: ${CONFIG.issueUrl}`,
    );
  }
  return mapped;
}

function archiveExtension(platform) {
  const extension = EXTENSION_MAPPING[platform];
  if (!extension) {
    fail(`No archive extension mapping found for ${platform}`);
  }
  return extension;
}

function archiveName(version, platform, arch) {
  return `${CONFIG.archiveName}_${version}_${platform}_${arch}.${archiveExtension(platform)}`;
}

function downloadUrl(tag, assetName) {
  return `https://github.com/${CONFIG.repo}/releases/download/${tag}/${assetName}`;
}

function checksumsUrl(tag) {
  return `https://github.com/${CONFIG.repo}/releases/download/${tag}/checksums.txt`;
}

async function downloadFile(url, destination) {
  const response = await fetch(url, { redirect: "follow" });
  if (!response.ok || !response.body) {
    fail(`Failed downloading ${url}: ${response.status} ${response.statusText}`);
  }
  await pipeline(Readable.fromWeb(response.body), createWriteStream(destination));
}

async function sha256(filePath) {
  const hash = createHash("sha256");
  const stream = await fs.open(filePath, "r");
  try {
    for await (const chunk of stream.createReadStream()) {
      hash.update(chunk);
    }
  } finally {
    await stream.close();
  }
  return hash.digest("hex");
}

async function verifyChecksum(filePath, checksumsFile) {
  const checksums = await fs.readFile(checksumsFile, "utf8");
  const fileName = basename(filePath);
  const expected = checksums
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line) => line.split(/\s+/))
    .find((parts) => parts[1] === fileName)?.[0];

  if (!expected) {
    fail(`Unable to find checksum entry for ${fileName}`);
  }

  const actual = await sha256(filePath);
  if (actual !== expected) {
    fail(`Checksum mismatch for ${fileName}`);
  }
}

async function extractArchive(archivePath, destination, platform) {
  if (platform === "win32") {
    const zip = new AdmZip(archivePath);
    zip.extractAllTo(destination, true);
    return;
  }
  await tar.x({
    cwd: destination,
    file: archivePath,
  });
}

async function install() {
  if (process.env.PP_SKIP_POSTINSTALL === "1") {
    console.log("Skipping printing-press postinstall because PP_SKIP_POSTINSTALL=1");
    return;
  }

  const packageJson = JSON.parse(await fs.readFile(join(rootDir, "package.json"), "utf8"));
  const version = resolveVersion(packageJson);
  const tag = `v${version}`;
  const platform = detectPlatform();
  const arch = detectArch();
  const asset = archiveName(version, platform, arch);
  const tempDir = await fs.mkdtemp(join(tmpdir(), "pp-npm-"));
  const archivePath = join(tempDir, asset);
  const checksumsPath = join(tempDir, "checksums.txt");
  const extractDir = join(tempDir, "extract");
  const installDir = join(rootDir, CONFIG.installDir);
  const installedBinary = join(
    installDir,
    process.platform === "win32" ? `${CONFIG.binaryName}.exe` : CONFIG.binaryName,
  );
  const extractedBinary = join(
    extractDir,
    process.platform === "win32" ? `${CONFIG.binaryName}.exe` : CONFIG.binaryName,
  );

  try {
    await fs.mkdir(extractDir, { recursive: true });
    await fs.mkdir(installDir, { recursive: true });

    console.log(`Installing ${CONFIG.binaryName} ${version}`);
    await downloadFile(downloadUrl(tag, asset), archivePath);
    await downloadFile(checksumsUrl(tag), checksumsPath);
    await verifyChecksum(archivePath, checksumsPath);
    await extractArchive(archivePath, extractDir, process.platform);

    if (!existsSync(extractedBinary)) {
      fail(`Archive did not contain ${basename(extractedBinary)}`);
    }

    await fs.copyFile(extractedBinary, installedBinary);
    if (process.platform !== "win32") {
      chmodSync(installedBinary, 0o755);
    }
  } finally {
    await fs.rm(tempDir, { force: true, recursive: true });
  }
}

install().catch((error) => {
  console.error(error.message);
  process.exit(1);
});
