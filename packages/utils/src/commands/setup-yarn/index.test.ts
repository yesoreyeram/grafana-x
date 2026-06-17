import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import { DEFAULT_YARN_VERSION } from '../../constants.js';
import { isValidYarnVersion, setupYarn, type SetupYarnDeps } from './index.js';

type RunCall = Parameters<SetupYarnDeps['run']>[0];

function makeDeps(overrides: Partial<SetupYarnDeps> = {}) {
    const calls: {
        run: RunCall[];
        log: string[];
        error: string[];
        exit: number[];
    } = {
        run: [],
        log: [],
        error: [],
        exit: [],
    };
    const deps: SetupYarnDeps = {
        run: (commands) => {
            calls.run.push(commands);
        },
        log: (m) => calls.log.push(m),
        error: (m) => calls.error.push(m),
        exit: ((code: number) => {
            calls.exit.push(code);
        }) as SetupYarnDeps['exit'],
        ...overrides,
    };
    return { deps, calls };
}

describe('isValidYarnVersion', () => {
    it('accepts plain semver', () => {
        assert.equal(isValidYarnVersion('4.16.0'), true);
    });

    it('accepts prerelease semver', () => {
        assert.equal(isValidYarnVersion('4.16.0-rc.1'), true);
    });

    it('rejects non-semver strings', () => {
        assert.equal(isValidYarnVersion('latest'), false);
        assert.equal(isValidYarnVersion('4.16'), false);
        assert.equal(isValidYarnVersion(''), false);
        assert.equal(isValidYarnVersion('4.16.0 && rm -rf /'), false);
    });
});

describe('setupYarn', () => {
    it('runs the corepack/yarn sequence with the given version', () => {
        const { deps, calls } = makeDeps();
        setupYarn('4.20.0', deps);

        assert.equal(calls.run.length, 1);
        const commands = calls.run[0];
        assert.deepEqual(commands, [
            ['corepack', ['enable']],
            ['corepack', ['prepare', 'yarn@4.20.0', '--activate']],
            ['yarn', ['install']],
            ['yarn', ['set', 'version', '4.20.0', '--yarn-path']],
        ]);
        assert.equal(calls.exit.length, 0);
    });

    it('uses the provided version, not the default', () => {
        const { deps, calls } = makeDeps();
        setupYarn(DEFAULT_YARN_VERSION, deps);
        const commands = calls.run[0]!;
        assert.deepEqual(commands[1], [
            'corepack',
            ['prepare', `yarn@${DEFAULT_YARN_VERSION}`, '--activate'],
        ]);
    });

    it('rejects invalid versions without running anything', () => {
        const { deps, calls } = makeDeps();
        setupYarn('latest', deps);

        assert.equal(calls.run.length, 0);
        assert.deepEqual(calls.exit, [1]);
        assert.match(calls.error.join('\n'), /Invalid Yarn version/);
    });

    it('exits non-zero with a clear message when a command fails', () => {
        const { deps, calls } = makeDeps({
            run: () => {
                throw new Error('Command failed: yarn install\n  boom');
            },
        });
        setupYarn('4.16.0', deps);

        assert.deepEqual(calls.exit, [1]);
        assert.match(calls.error.join('\n'), /Setup failed/);
        assert.match(calls.error.join('\n'), /yarn install/);
    });
});
