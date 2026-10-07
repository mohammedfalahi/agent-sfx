#!/usr/bin/env node

const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const process = require('node:process');

function main() {
  const platform = process.platform;
  const arch = process.arch;

  let archDir;
  let binaryName = 'agent-sfx';

  if (platform === 'darwin') {
    if (arch === 'arm64') {
      archDir = 'darwin-arm64';
    } else if (arch === 'x64') {
      archDir = 'darwin-x64';
    } else {
      console.error(`Agent SFX: Architecture "${arch}" is not supported on macOS. Supported architectures: arm64, x64.`);
      process.exit(1);
    }
  } else if (platform === 'win32') {
    if (arch === 'x64') {
      archDir = 'windows-amd64';
      binaryName = 'agent-sfx.exe';
    } else if (arch === 'arm64') {
      console.error('Agent SFX: Windows ARM64 is not supported in this release. Supported Windows architecture: x64.');
      process.exit(1);
    } else {
      console.error(`Agent SFX: Architecture "${arch}" is not supported on Windows. Supported Windows architecture: x64.`);
      process.exit(1);
    }
  } else {
    console.error(`Agent SFX: Operating system "${platform}" is not supported in this release. Supported platforms: macOS (darwin-arm64, darwin-x64), Windows (windows-amd64).`);
    process.exit(1);
  }

  const packageDir = path.resolve(__dirname, '..');
  const binaryPath = path.join(packageDir, 'bin', archDir, binaryName);

  if (!fs.existsSync(binaryPath)) {
    console.error(`Agent SFX: Prebuilt binary not found at ${binaryPath}`);
    process.exit(1);
  }

  // Ensure executable permissions on Unix platforms
  if (platform !== 'win32') {
    try {
      const stats = fs.statSync(binaryPath);
      if ((stats.mode & 0o111) === 0) {
        fs.chmodSync(binaryPath, 0o755);
      }
    } catch (err) {
      // Non-fatal permission check
    }
  }

  // Pass bundled sounds directory via environment variable if not already set,
  // allowing the binary to locate its bundled assets without relying on the current working directory.
  const env = { ...process.env };
  if (!env.AGENT_SFX_BUNDLED_SOUNDS_DIR) {
    env.AGENT_SFX_BUNDLED_SOUNDS_DIR = path.join(packageDir, 'sounds');
  }

  const result = spawnSync(binaryPath, process.argv.slice(2), {
    stdio: 'inherit',
    env,
  });

  if (result.error) {
    console.error(`Agent SFX: Failed to execute binary: ${result.error.message}`);
    process.exit(1);
  }

  if (result.status !== null) {
    process.exit(result.status);
  }

  if (result.signal) {
    process.kill(process.pid, result.signal);
  }
}

main();
