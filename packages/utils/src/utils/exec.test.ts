import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import { execSequence } from './exec.js';

describe('execSequence', () => {
    it('annotates the failing command in the thrown error', () => {
        assert.throws(
            () =>
                execSequence(
                    [['definitely-not-a-real-binary-xyz', ['--nope']]],
                    { verbose: false }
                ),
            (err: Error) => {
                assert.match(
                    err.message,
                    /Command failed: definitely-not-a-real-binary-xyz --nope/
                );
                return true;
            }
        );
    });

    it('reports the specific command that failed in a multi-step sequence', () => {
        assert.throws(
            () =>
                execSequence(
                    [
                        ['node', ['--version']], // succeeds
                        ['node', ['--this-flag-does-not-exist']], // fails
                    ],
                    { verbose: false, stdio: 'ignore' }
                ),
            (err: Error) => {
                assert.match(
                    err.message,
                    /Command failed: node --this-flag-does-not-exist/
                );
                return true;
            }
        );
    });
});
