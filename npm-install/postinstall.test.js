import assert from "node:assert/strict";
import { mkdir, mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";

import AdmZip from "adm-zip";

import { archiveName, extractArchive } from "./postinstall.js";

test("windows archive names match GoReleaser zip assets", () => {
  assert.equal(
    archiveName("0.0.13", "windows", "x86_64"),
    "printing-press_0.0.13_windows_x86_64.zip",
  );
});

test("windows archives are extracted as zip files", async () => {
  const tempDir = await mkdtemp(join(tmpdir(), "ppress-postinstall-"));

  try {
    const archivePath = join(tempDir, "printing-press.zip");
    const extractDir = join(tempDir, "extract");
    const zip = new AdmZip();

    zip.addFile("ppress.exe", Buffer.from("binary"));
    zip.writeZip(archivePath);

    await mkdir(extractDir);
    await extractArchive(archivePath, extractDir, "windows");

    assert.equal(await readFile(join(extractDir, "ppress.exe"), "utf8"), "binary");
  } finally {
    await rm(tempDir, { force: true, recursive: true });
  }
});
