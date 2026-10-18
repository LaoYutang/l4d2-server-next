// Runs a temporary script copy with guarded paths and mocked external effects.
// No real Docker, installer, network, service or symlink command is executed.
const assert = require('node:assert/strict');
const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { test } = require('node:test');

const source = fs.readFileSync(path.join(__dirname, '..', 'manage.sh'), 'utf8');
const bash = process.env.BASH_TEST_EXECUTABLE || 'bash';
const tempRoot = process.env.L4D2_TEST_TMPDIR || os.tmpdir();
const quote = value => "'" + value.replace(/'/g, "'\\''") + "'";
const shellPath = value => value.replace(/\\/g, '/');
const prefix = '9\nn\nTEST_PASSWORD\nTEST_PASSWORD\n\n\n\n';
const mocks = String.raw`
id() { printf '%s\n' "$TEST_UID"; }
clear() { printf 'TRACE_CLEAR\n'; }
print_banner() { :; }
print_menu_item() { :; }
try_import_existing() { :; }
create_symlinks() { printf 'MOCK_SYMLINKS\n'; }
install_docker() { printf 'UNEXPECTED_INSTALL\n' >&2; exit 97; }
curl() { printf 'UNEXPECTED_NETWORK\n' >&2; exit 97; }
systemctl() { printf 'UNEXPECTED_SERVICE\n' >&2; exit 97; }
tr() { printf 'TEST_RCON_ONLY'; }
mkdir() {
    if [[ "$TEST_FAILURE" == init || ( "$TEST_FAILURE" == bind && "$*" == *bind-data* ) ]]; then
        printf 'MOCK_MKDIR_PERMISSION_DENIED\n' >&2
        return 1
    fi
    command mkdir "$@"
}
cd() {
    if [[ "$TEST_FAILURE" == cd && "$1" == "$BASE_DIR" ]]; then
        printf 'MOCK_CD_PERMISSION_DENIED\n' >&2
        return 1
    fi
    builtin cd "$@"
}
docker() {
    case "$*" in
        --version) printf 'Docker version mock, build mock\n' ;;
        'compose up -d'|'compose restart l4d2 l4d2-manager')
            printf 'MOCK_DOCKER_DEPLOY\n'
            if [[ "$TEST_FAILURE" == docker ]]; then
                printf 'MOCK_COMPOSE_ORIGINAL_ERROR_EXIT_42\n' >&2
                return 42
            fi ;;
        *) printf 'UNEXPECTED_DOCKER_COMMAND\n' >&2; exit 97 ;;
    esac
}
seed_instance() {
    ensure_dirs
    GAME_PORT=27015 MANAGER_PORT=27020 ADMIN_PASSWORD=TEST_ONLY RCON_PASSWORD=TEST_ONLY
    TICK=100 VAC=false STEAM_API_KEY=''
    save_instance_config l4d2
}
`;

function run({ input = '', entry = 'main', failure = '', uid = '0', fixture = '' } = {}) {
  const root = fs.mkdtempSync(path.join(tempRoot, 'l4d2-manage-test-'));
  const base = path.join(root, 'base');
  try {
    assert.equal((source.match(/^readonly BASE_DIR=.*$/gm) || []).length, 1);
    assert.match(source, /\nmain "\$@"\s*$/);
    let script = source.replace(/^readonly BASE_DIR=.*$/m,
      'readonly BASE_DIR=' + quote(shellPath(base)));
    script = script.replace(/\nmain "\$@"\s*$/, '\n' + mocks + '\n' + entry + '\n');
    // The real script's /data path and its automatic entry are never executed.
    assert.ok(!script.includes('readonly BASE_DIR="/data/l4d2"'));
    const scriptPath = path.join(root, 'harness.sh');
    fs.writeFileSync(scriptPath, script);
    if (fixture === 'config_directory') {
      fs.mkdirSync(path.join(base, '.config', 'instances', 'l4d2.conf'), { recursive: true });
    } else if (fixture === 'compose_directory') {
      fs.mkdirSync(path.join(base, 'docker-compose.yaml'), { recursive: true });
    }
    const env = { ...process.env, BASH_ENV: '/dev/null', ENV: '/dev/null',
      LC_ALL: 'C.UTF-8', TEST_FAILURE: failure, TEST_UID: uid };
    delete env.SHELLOPTS;
    const result = spawnSync(bash, ['--noprofile', '--norc', shellPath(scriptPath)], {
      cwd: root, env, input: Buffer.from(input), encoding: 'utf8', timeout: 10000,
      maxBuffer: 1024 * 1024, windowsHide: true
    });
    assert.ifError(result.error);
    assert.doesNotMatch(result.stderr, /UNEXPECTED_/);
    return { ...result, baseExists: fs.existsSync(base),
      configExists: fs.existsSync(path.join(base, '.config', 'instances', 'l4d2.conf')),
      deployCalls: (result.stdout.match(/MOCK_DOCKER_DEPLOY/g) || []).length,
      clearCalls: (result.stdout.match(/TRACE_CLEAR/g) || []).length };
  } finally {
    // Delete only this newly created, verified test directory.
    assert.equal(path.dirname(fs.realpathSync(root)), fs.realpathSync(tempRoot));
    assert.ok(path.basename(root).startsWith('l4d2-manage-test-'));
    fs.rmSync(root, { recursive: true, force: true });
  }
}

test('script version is 1.1.0', () => {
  assert.match(source, /^readonly SCRIPT_VERSION="1\.1\.0"$/m);
});

for (const [label, input, defaultValue, accepted] of [
  ['lowercase y', 'y\n', 'y', true], ['uppercase Y', 'Y\n', 'y', true],
  ['empty default yes', '\n', 'y', true], ['empty default no', '\n', 'n', false],
  ['CRLF y', 'y\r\n', 'y', true], ['literal yes', 'yes\n', 'y', false],
  ['explicit no', 'n\n', 'y', false]
]) {
  test('confirmation: ' + label, () => {
    const result = run({ input,
      entry: `if confirm test ${defaultValue}; then echo ACCEPT; else echo REJECT; fi` });
    assert.equal(result.status, 0);
    assert.match(result.stdout, accepted ? /ACCEPT/ : /REJECT/);
  });
}

for (const input of ['', 'y']) {
  test('confirmation rejects EOF, including an unterminated y: ' + JSON.stringify(input), () => {
    const result = run({ input, entry: 'if confirm test y; then echo ACCEPT; fi\necho AFTER' });
    assert.equal(result.status, 1);
    assert.doesNotMatch(result.stdout, /ACCEPT|AFTER/);
    assert.match(result.stderr, /输入读取失败/);
  });
}

test('non-root fails before directory creation or menu clearing', () => {
  const result = run({ uid: '1000' });
  assert.equal(result.status, 1);
  assert.equal(result.baseExists, false);
  assert.equal(result.clearCalls, 0);
  assert.match(result.stderr, /root.*sudo -i/);
});

for (const answer of ['y', 'Y', '']) {
  test('quick setup succeeds with ' + JSON.stringify(answer), () => {
    const result = run({ input: prefix + answer + '\n\n0\n' });
    assert.equal(result.status, 0, result.stderr);
    assert.equal(result.deployCalls, 1);
    assert.equal(result.configExists, true);
    assert.match(result.stdout, /安装完成/);
  });
}

test('explicit cancellation does not save or deploy', () => {
  const result = run({ input: prefix + 'n\n\n0\n' });
  assert.equal(result.status, 0);
  assert.equal(result.deployCalls, 0);
  assert.equal(result.configExists, false);
});

test('directory initialization failure stops before prompts', () => {
  const result = run({ failure: 'init', input: prefix + 'y\n' });
  assert.equal(result.status, 1);
  assert.equal(result.deployCalls, 0);
  assert.equal(result.clearCalls, 0);
  assert.match(result.stderr, /MOCK_MKDIR_PERMISSION_DENIED[\s\S]*无法创建配置目录/);
});

for (const [fixture, error, falseSuccess] of [
  ['config_directory', /保存实例配置失败/, /配置已保存|已生成|安装完成/],
  ['compose_directory', /生成 Compose 配置失败/, /已生成|安装完成/]
]) {
  test(fixture + ' failure stops before Docker and does not claim success', () => {
    const result = run({ fixture, input: prefix + 'y\n\n0\n' });
    assert.equal(result.status, 1);
    assert.equal(result.deployCalls, 0);
    assert.equal(result.clearCalls, 2);
    assert.match(result.stderr, error);
    assert.doesNotMatch(result.stdout, falseSuccess);
  });
}

test('failed cd retains its error and never deploys or clears the menu again', () => {
  const result = run({ failure: 'cd', input: prefix + 'y\n\n0\n' });
  assert.equal(result.status, 1);
  assert.equal(result.deployCalls, 0);
  assert.equal(result.clearCalls, 2);
  assert.match(result.stderr, /MOCK_CD_PERMISSION_DENIED[\s\S]*无法进入部署目录/);
});

for (const remainingInput of ['\n0\n', '']) {
  test('Docker failure retains original stderr with queued Enter or EOF: ' + JSON.stringify(remainingInput), () => {
    const result = run({ failure: 'docker', input: prefix + 'y\n' + remainingInput });
    assert.equal(result.status, 1);
    assert.equal(result.deployCalls, 1);
    assert.equal(result.clearCalls, 2);
    assert.match(result.stderr, /MOCK_COMPOSE_ORIGINAL_ERROR_EXIT_42[\s\S]*启动失败/);
    assert.doesNotMatch(result.stdout, /安装完成/);
  });
}

for (const [label, input] of [
  ['menu', ''], ['mirror confirmation', '9\n'], ['password', '9\nn\n'],
  ['game port', '9\nn\nTEST_ONLY\nTEST_ONLY\n'], ['final confirmation', prefix]
]) {
  test('EOF during ' + label + ' stops without deploying', () => {
    const result = run({ input });
    assert.equal(result.status, 1);
    assert.equal(result.deployCalls, 0);
    assert.match(result.stderr, /输入读取失败/);
  });
}

test('add instance also stops on Docker failure', () => {
  const result = run({ entry: 'ensure_dirs\nadd_instance', failure: 'docker',
    input: 'TEST_ONLY\nTEST_ONLY\n\n\n\nn\n\nn\ny\n' });
  assert.equal(result.status, 1);
  assert.equal(result.deployCalls, 1);
  assert.match(result.stderr, /MOCK_COMPOSE_ORIGINAL_ERROR_EXIT_42[\s\S]*启动失败/);
  assert.doesNotMatch(result.stdout, /已创建并启动/);
});

test('bind directory failure stops before saving or deploying', () => {
  const result = run({ entry: 'ensure_dirs\nadd_instance', failure: 'bind',
    input: 'TEST_ONLY\nTEST_ONLY\n\n\n\nn\n\ny\n./bind-data\n./bind-plugins\ny\n' });
  assert.equal(result.status, 1);
  assert.equal(result.deployCalls, 0);
  assert.equal(result.configExists, false);
  assert.match(result.stderr, /MOCK_MKDIR_PERMISSION_DENIED[\s\S]*bind 挂载目录失败/);
});

test('edit-and-restart retains deployment errors', () => {
  const result = run({ entry: 'seed_instance\nselect_instance() { echo l4d2; }\nedit_instance',
    failure: 'docker', input: 's\ny\n\n0\n' });
  assert.equal(result.status, 1);
  assert.equal(result.deployCalls, 1);
  assert.match(result.stderr, /MOCK_COMPOSE_ORIGINAL_ERROR_EXIT_42[\s\S]*重启失败/);
  assert.doesNotMatch(result.stdout, /已更新并重启/);
});

test('rebuild retains deployment errors without returning to the menu', () => {
  const result = run({ entry: 'seed_instance\nmenu_images', failure: 'docker', input: '6\n\n0\n' });
  assert.equal(result.status, 1);
  assert.equal(result.deployCalls, 1);
  assert.equal(result.clearCalls, 1);
  assert.match(result.stderr, /MOCK_COMPOSE_ORIGINAL_ERROR_EXIT_42[\s\S]*重建失败/);
  assert.doesNotMatch(result.stdout, /所有实例已重建/);
});
