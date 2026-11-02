import { Plugin } from '@opencode/plugin';
import { createV2Observer } from './v2.js';
import type { V2NativePorts } from './v2.js';

// Type-only seam. A production binding must be supplied by native qualification;
// these declarations do not manufacture Context methods or form authority.
declare const native: V2NativePorts;
declare const location: string;
declare const runtimeEligibility: (version: string) => 'supported' | 'unverified' | 'unsupported';
export const plugin = Plugin.define({
  id: 'observer-contract-check',
  setup(ctx) {
    const observer = createV2Observer({ context: ctx, native, location, runtimeEligibility, emit() {} });
    observer.start();
    return () => observer.dispose();
  },
});
