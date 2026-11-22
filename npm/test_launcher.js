// Tests for the Node launcher used by the Gemini extension and npm package
// (npm/bin/run.js). The Claude Code plugin uses a POSIX sh launcher instead
// (npm/claude/runtime/agent-sfx), tested in internal/packaging.
//
// These tests use fake binaries in temporary package layouts, so they do not
// depend on any built or committed agent-sfx binary. Run with:
//   node npm/test_launcher.js
// The fake binaries are POSIX shell scripts, so the suite runs on macOS/Linux.

const assert = require('node:assert');
const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const geminiLauncher = path.resolve(__dirname, 'bin', 'run.js');

const FAKE_BINARY = `#!/bin/sh
for a in "$@"; do printf 'ARG=%s\\n' "$a"; done
printf 'SOUNDS=%s\\n' "$AGENT_SFX_BUNDLED_SOUNDS_DIR"
exit \${FAKE_EXIT:-0}
`;

const tmpRoots = [];
function tmpDir(name) {
  const dir = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), name)));
  tmpRoots.push(dir);
  return dir;
}

function currentArchDir() {
  if (process.platform === 'darwin') {
    return process.arch === 'arm64' ? 'darwin-arm64' : 'darwin-x64';
  }
  return null;
}

// Builds <root>/<launcherSubdir>/run.js with a fake binary next to it and a
// bundled sounds directory at <root>/sounds. Returns the launcher path.
function makeLayout(root, launcherSubdir, archDir, binaryName = 'agent-sfx') {
  const launcherDir = path.join(root, launcherSubdir);
  fs.mkdirSync(path.join(launcherDir, archDir), { recursive: true });
  fs.mkdirSync(path.join(root, 'sounds'), { recursive: true });
  fs.copyFileSync(geminiLauncher, path.join(launcherDir, 'run.js'));
  fs.writeFileSync(path.join(launcherDir, archDir, binaryName), FAKE_BINARY, { mode: 0o755 });
  return path.join(launcherDir, 'run.js');
}

function run(launcher, args, extraEnv = {}) {
  const env = { ...process.env, ...extraEnv };
  if (!('AGENT_SFX_BUNDLED_SOUNDS_DIR' in extraEnv)) delete env.AGENT_SFX_BUNDLED_SOUNDS_DIR;
  return spawnSync(process.execPath, [launcher, ...args], { encoding: 'utf-8', env });
}

// Runs the launcher with a mocked platform/arch and a stubbed spawnSync.
function runMocked(launcher, platform, arch) {
  const script = `
    const cp = require('node:child_process');
    cp.spawnSync = function(bin) { console.log('BIN=' + bin); return { status: 0, error: null }; };
    Object.defineProperty(process, 'platform', { value: ${JSON.stringify(platform)} });
    Object.defineProperty(process, 'arch', { value: ${JSON.stringify(arch)} });
    require(${JSON.stringify(launcher)});
  `;
  return spawnSync(process.execPath, ['-e', script], { encoding: 'utf-8' });
}

const tests = [];
function test(name, fn) { tests.push({ name, fn }); }

const archDir = currentArchDir();

if (archDir) {
  test('Gemini extension layout: bin/run.js finds bin/<arch> binary and extension sounds', () => {
    const root = tmpDir('sfx-gemini-');
    const launcher = makeLayout(root, 'bin', archDir);
    const res = run(launcher, ['hook', 'gemini']);
    assert.strictEqual(res.status, 0, res.stderr);
    assert.ok(res.stdout.includes(`SOUNDS=${path.join(root, 'sounds')}\n`), res.stdout);
  });

  test('exit codes are preserved', () => {
    const launcher = makeLayout(tmpDir('sfx-exit-'), 'runtime', archDir);
    assert.strictEqual(run(launcher, [], { FAKE_EXIT: '3' }).status, 3);
  });

  test('arguments with spaces and quotes are forwarded unchanged', () => {
    const launcher = makeLayout(tmpDir('sfx-args-'), 'runtime', archDir);
    const res = run(launcher, ['preview', 'a b "c"']);
    assert.ok(res.stdout.includes('ARG=a b "c"\n'), res.stdout);
  });

  test('works from a path containing spaces', () => {
    const root = path.join(tmpDir('sfx-space-'), 'dir with spaces');
    const launcher = makeLayout(root, 'runtime', archDir);
    const res = run(launcher, ['status']);
    assert.strictEqual(res.status, 0, res.stderr);
    assert.ok(res.stdout.includes(`SOUNDS=${path.join(root, 'sounds')}\n`), res.stdout);
  });

  test('an existing AGENT_SFX_BUNDLED_SOUNDS_DIR is not overwritten', () => {
    const launcher = makeLayout(tmpDir('sfx-env-'), 'runtime', archDir);
    const res = run(launcher, [], { AGENT_SFX_BUNDLED_SOUNDS_DIR: '/custom/bundled' });
    assert.ok(res.stdout.includes('SOUNDS=/custom/bundled\n'), res.stdout);
  });

  test('missing binary fails with a clear message', () => {
    const root = tmpDir('sfx-missing-');
    fs.mkdirSync(path.join(root, 'runtime'), { recursive: true });
    fs.copyFileSync(geminiLauncher, path.join(root, 'runtime', 'run.js'));
    const res = run(path.join(root, 'runtime', 'run.js'), ['status']);
    assert.strictEqual(res.status, 1);
    assert.ok(res.stderr.includes('Prebuilt binary not found'), res.stderr);
  });

  test('restores the executable bit on the binary', () => {
    const root = tmpDir('sfx-chmod-');
    const launcher = makeLayout(root, 'runtime', archDir);
    fs.chmodSync(path.join(root, 'runtime', archDir, 'agent-sfx'), 0o644);
    assert.strictEqual(run(launcher, []).status, 0);
  });
}

test('win32/x64 selects <launcher dir>/windows-amd64/agent-sfx.exe', () => {
  const root = tmpDir('sfx-win-');
  const launcher = makeLayout(root, 'runtime', 'windows-amd64', 'agent-sfx.exe');
  const res = runMocked(launcher, 'win32', 'x64');
  assert.strictEqual(res.status, 0, res.stderr);
  assert.ok(res.stdout.includes(path.join(root, 'runtime', 'windows-amd64', 'agent-sfx.exe')), res.stdout);
});

test('Windows ARM64 is rejected', () => {
  const res = runMocked(geminiLauncher, 'win32', 'arm64');
  assert.strictEqual(res.status, 1);
  assert.ok(res.stderr.includes('Windows ARM64 is not supported'), res.stderr);
});

test('Linux is rejected in this release', () => {
  const res = runMocked(geminiLauncher, 'linux', 'x64');
  assert.strictEqual(res.status, 1);
  assert.ok(res.stderr.includes('Operating system "linux" is not supported'), res.stderr);
});

test('unsupported macOS architecture is rejected', () => {
  const res = runMocked(geminiLauncher, 'darwin', 's390x');
  assert.strictEqual(res.status, 1);
  assert.ok(res.stderr.includes('Architecture "s390x" is not supported'), res.stderr);
});

let failed = 0;
console.log('--- Testing Node launcher ---');
for (const { name, fn } of tests) {
  try {
    fn();
    console.log(`ok   ${name}`);
  } catch (err) {
    failed++;
    console.log(`FAIL ${name}\n     ${err.message.split('\n').join('\n     ')}`);
  }
}
for (const dir of tmpRoots) fs.rmSync(dir, { recursive: true, force: true });
if (!archDir) console.log('(native launch tests skipped: not macOS)');
console.log(failed ? `${failed} of ${tests.length} launcher tests failed` : `All ${tests.length} launcher tests passed`);
process.exit(failed ? 1 : 0);
