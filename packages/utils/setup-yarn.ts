#!/usr/bin/env -S node --import tsx

import { execFileSync } from 'node:child_process';

const YARN_VERSION = '4.16.0';

function run(command: string, args: readonly string[]): void {
    console.log(`\n$ ${command} ${args.join(' ')}`);
    execFileSync(command, args, { stdio: 'inherit' });
}

function main(): void {
    run('corepack', ['enable']);
    run('corepack', ['prepare', `yarn@${YARN_VERSION}`, '--activate']);
    run('yarn', ['install']);
    run('yarn', ['set', 'version', YARN_VERSION, '--yarn-path']);
}

try {
    main();
} catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    console.error(`\nnormalize-datasource failed: ${message}`);
    process.exit(1);
}
