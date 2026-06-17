#!/usr/bin/env node

import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { Command } from 'commander';
import { setupYarnCommand } from './commands/setup-yarn/index.js';

const pkg = JSON.parse(
    readFileSync(
        fileURLToPath(new URL('../package.json', import.meta.url)),
        'utf8'
    )
) as { version: string };

const program = new Command();

program
    .name('grafana-utils')
    .description('CLI tool for managing Grafana datasources and utilities')
    .version(pkg.version);

// Register commands
program.addCommand(setupYarnCommand);

// Add help for main command
program.on('--help', () => {
    console.log('');
    console.log('Examples:');
    console.log('  $ grafana-utils setup-yarn --yarn-version 4.16.0');
    console.log('  $ grafana-utils setup-yarn -v 4.16.0');
    console.log('  $ grafana-utils setup-yarn');
});

program.parse(process.argv);

// Show help if no command provided
if (!process.argv.slice(2).length) {
    program.outputHelp();
}
