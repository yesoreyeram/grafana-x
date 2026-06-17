import { execFileSync, ExecFileSyncOptions } from 'node:child_process';

export interface ExecOptions extends ExecFileSyncOptions {
    verbose?: boolean;
}

/**
 * Execute a command and optionally display it
 * @param command - The command to execute
 * @param args - Arguments to pass to the command
 * @param options - Execution options
 */
export function exec(
    command: string,
    args: readonly string[],
    options: ExecOptions = {}
): void {
    const { verbose = true, ...execOptions } = options;

    if (verbose) {
        console.log(`\n$ ${command} ${args.join(' ')}`);
    }

    execFileSync(command, args, {
        stdio: 'inherit',
        ...execOptions,
    });
}

/**
 * Execute multiple commands in sequence. If a command fails, the thrown error
 * is annotated with the failing command so callers can report it precisely.
 * @param commands - Array of [command, args] tuples
 * @param options - Execution options
 */
export function execSequence(
    commands: Array<[string, readonly string[]]>,
    options: ExecOptions = {}
): void {
    for (const [command, args] of commands) {
        try {
            exec(command, args, options);
        } catch (err) {
            const original = err instanceof Error ? err.message : String(err);
            throw new Error(
                `Command failed: ${command} ${args.join(' ')}\n  ${original}`
            );
        }
    }
}
