const assert = require('node:assert');
const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const process = require('node:process');

const launcherPath = path.resolve(__dirname, 'bin', 'run.js');
const claudeLauncherPath = path.resolve(__dirname, 'claude', 'bin', 'run.js');

console.log('--- Testing Node Launcher ---');

// 1. Argument forwarding & exit code 0
{
  const res = spawnSync(process.execPath, [launcherPath, '--help'], { encoding: 'utf-8' });
  assert.strictEqual(res.status, 0, 'Expected exit status 0 for --help');
  assert.ok(res.stdout.includes('Agent SFX'), 'Expected stdout to contain Agent SFX');
  console.log('✓ Argument forwarding and exit status 0 passed');
}

// 2. Exit code preservation on invalid command (exit 1)
{
  const res = spawnSync(process.execPath, [launcherPath, 'nonexistent_command'], { encoding: 'utf-8' });
  assert.strictEqual(res.status, 1, 'Expected exit status 1 for unknown command');
  assert.ok(res.stderr.includes('Unknown command'), 'Expected stderr to mention unknown command');
  console.log('✓ Exit code 1 preservation passed');
}

// 3. Platform check: Windows ARM64 rejection
{
  const testScript = `
    Object.defineProperty(process, 'platform', { value: 'win32' });
    Object.defineProperty(process, 'arch', { value: 'arm64' });
    require(${JSON.stringify(launcherPath)});
  `;
  const res = spawnSync(process.execPath, ['-e', testScript], { encoding: 'utf-8' });
  assert.strictEqual(res.status, 1, 'Expected exit status 1 for win32/arm64');
  assert.ok(res.stderr.includes('Windows ARM64 is not supported'), 'Expected Windows ARM64 rejection message');
  console.log('✓ Windows ARM64 platform rejection passed');
}

// 4. Platform check: Windows x64 binary selection
{
  // Test that win32/x64 targets windows-amd64/agent-sfx.exe (which exists)
  const testScript = `
    const cp = require('node:child_process');
    let capturedBin = '';
    cp.spawnSync = function(bin, args, opts) {
      capturedBin = bin;
      return { status: 0, error: null };
    };
    Object.defineProperty(process, 'platform', { value: 'win32' });
    Object.defineProperty(process, 'arch', { value: 'x64' });
    require(${JSON.stringify(launcherPath)});
    if (!capturedBin.endsWith(require('node:path').join('windows-amd64', 'agent-sfx.exe'))) {
      console.error('Wrong binary targeted: ' + capturedBin);
      process.exit(2);
    }
  `;
  const res = spawnSync(process.execPath, ['-e', testScript], { encoding: 'utf-8' });
  assert.strictEqual(res.status, 0, 'Expected win32/x64 to successfully locate windows-amd64/agent-sfx.exe');
  console.log('✓ Windows x64 binary selection passed');
}

// 5. Platform check: Claude plugin launcher Windows x64 binary selection
{
  const testScript = `
    const cp = require('node:child_process');
    let capturedBin = '';
    cp.spawnSync = function(bin, args, opts) {
      capturedBin = bin;
      return { status: 0, error: null };
    };
    Object.defineProperty(process, 'platform', { value: 'win32' });
    Object.defineProperty(process, 'arch', { value: 'x64' });
    require(${JSON.stringify(claudeLauncherPath)});
    if (!capturedBin.endsWith(require('node:path').join('windows-amd64', 'agent-sfx.exe'))) {
      console.error('Wrong binary targeted: ' + capturedBin);
      process.exit(2);
    }
  `;
  const res = spawnSync(process.execPath, ['-e', testScript], { encoding: 'utf-8' });
  assert.strictEqual(res.status, 0, 'Expected claude launcher win32/x64 to locate windows-amd64/agent-sfx.exe');
  console.log('✓ Claude plugin launcher Windows x64 binary selection passed');
}

// 6. Platform check: Linux rejection
{
  const testScript = `
    Object.defineProperty(process, 'platform', { value: 'linux' });
    require(${JSON.stringify(launcherPath)});
  `;
  const res = spawnSync(process.execPath, ['-e', testScript], { encoding: 'utf-8' });
  assert.strictEqual(res.status, 1, 'Expected exit status 1 for linux platform');
  assert.ok(res.stderr.includes('Operating system "linux" is not supported'), 'Expected Linux error message');
  console.log('✓ Linux platform rejection passed');
}

// 7. Architecture check: unsupported arch
{
  const testScript = `
    Object.defineProperty(process, 'arch', { value: 's390x' });
    require(${JSON.stringify(launcherPath)});
  `;
  const res = spawnSync(process.execPath, ['-e', testScript], { encoding: 'utf-8' });
  assert.strictEqual(res.status, 1, 'Expected exit status 1 for unsupported arch');
  assert.ok(res.stderr.includes('Architecture "s390x" is not supported'), 'Expected arch error message');
  console.log('✓ Unsupported architecture rejection passed');
}

// 8. Path with spaces test
{
  const tmpBase = path.join(require('node:os').tmpdir(), 'sfx space test dir');
  fs.rmSync(tmpBase, { recursive: true, force: true });
  fs.mkdirSync(tmpBase, { recursive: true });

  const pkgDir = path.resolve(__dirname);
  fs.cpSync(pkgDir, tmpBase, { recursive: true });

  const spaceLauncher = path.join(tmpBase, 'bin', 'run.js');
  const res = spawnSync(process.execPath, [spaceLauncher, '--help'], { encoding: 'utf-8' });
  assert.strictEqual(res.status, 0, 'Expected exit status 0 for launcher in path with spaces');
  assert.ok(res.stdout.includes('Agent SFX'), 'Expected stdout from space path launcher');

  fs.rmSync(tmpBase, { recursive: true, force: true });
  console.log('✓ Paths with spaces execution passed');
}

// 9. Relocated self-contained Claude plugin test
{
  const tmpClaude = path.join(require('node:os').tmpdir(), `sfx-claude-relocated-${Date.now()}`);
  fs.rmSync(tmpClaude, { recursive: true, force: true });
  fs.mkdirSync(tmpClaude, { recursive: true });

  const srcClaude = path.resolve(__dirname, 'claude');
  fs.cpSync(srcClaude, tmpClaude, { recursive: true });

  const relocatedLauncher = path.join(tmpClaude, 'bin', 'run.js');

  // Verify help works from relocated path
  const resHelp = spawnSync(process.execPath, [relocatedLauncher, '--help'], { encoding: 'utf-8' });
  assert.strictEqual(resHelp.status, 0, 'Expected exit status 0 for relocated claude launcher --help');
  assert.ok(resHelp.stdout.includes('Agent SFX'), 'Expected stdout from relocated claude launcher');

  // Verify hook claude executes neutrally from relocated path with no external sibling dependency
  const resHook = spawnSync(process.execPath, [relocatedLauncher, 'hook', 'claude'], {
    input: '{"type":"Stop"}\n',
    encoding: 'utf-8',
  });
  assert.strictEqual(resHook.status, 0, 'Expected exit status 0 for relocated claude hook');
  assert.strictEqual(resHook.stdout.trim(), '{}', 'Expected neutral JSON stdout from relocated claude hook');

  fs.rmSync(tmpClaude, { recursive: true, force: true });
  console.log('✓ Relocated self-contained Claude plugin execution passed');
}

console.log('All launcher tests passed successfully!');
