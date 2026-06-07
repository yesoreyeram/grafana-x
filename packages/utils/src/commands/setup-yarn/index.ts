import { Command } from 'commander';
import { execSequence } from '../../utils/exec.js';

const DEFAULT_YARN_VERSION = '4.16.0';

function setupYarn(version: string): void {
    console.log(`Setting up Yarn version ${version}...`);
    try {
        execSequence([
            ['corepack', ['enable']],
            ['corepack', ['prepare', `yarn@${version}`, '--activate']],
            ['yarn', ['install']],
            ['yarn', ['set', 'version', version, '--yarn-path']],
        ]);
        console.log(`\n✓ Successfully set up Yarn ${version}`);
    } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        console.error(`\n✗ Setup failed: ${message}`);
        process.exit(1);
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
