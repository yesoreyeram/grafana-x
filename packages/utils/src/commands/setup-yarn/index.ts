import { Command } from 'commander';
import { DEFAULT_YARN_VERSION } from '../../constants.js';
import { execSequence } from '../../utils/exec.js';

/** Matches a semantic version such as `4.16.0` or `4.16.0-rc.1`. */
const SEMVER_PATTERN = /^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/;

export interface SetupYarnDeps {
    /** Runs the ordered list of commands. Injectable for testing. */
    run: typeof execSequence;
    log: (message: string) => void;
    error: (message: string) => void;
    exit: (code: number) => never;
}

const defaultDeps: SetupYarnDeps = {
    run: execSequence,
    log: (message) => console.log(message),
    error: (message) => console.error(message),
    exit: (code) => process.exit(code),
};

/** Validate a Yarn version string before passing it to corepack. */
export function isValidYarnVersion(version: string): boolean {
    return SEMVER_PATTERN.test(version);
}

export function setupYarn(
    version: string,
    deps: SetupYarnDeps = defaultDeps
): void {
    if (!isValidYarnVersion(version)) {
        deps.error(
            `\n✗ Invalid Yarn version: "${version}". Expected a semantic version like 4.16.0`
        );
        deps.exit(1);
        return;
    }

    deps.log(`Setting up Yarn version ${version}...`);
    try {
        deps.run([
            ['corepack', ['enable']],
            ['corepack', ['prepare', `yarn@${version}`, '--activate']],
            ['yarn', ['install']],
            ['yarn', ['set', 'version', version, '--yarn-path']],
        ]);
        deps.log(`\n✓ Successfully set up Yarn ${version}`);
    } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        deps.error(`\n✗ Setup failed: ${message}`);
        deps.exit(1);
    }
}

export const setupYarnCommand = new Command('setup-yarn')
    .description('Setup and configure Yarn with a specific version')
    .option(
        '-v, --yarn-version <version>',
        'Yarn version to set up',
        DEFAULT_YARN_VERSION
    )
    .action((options: { yarnVersion: string }) => {
        setupYarn(options.yarnVersion);
    });
