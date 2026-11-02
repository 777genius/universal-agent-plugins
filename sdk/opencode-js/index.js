export { createObserver } from './v1.js';
import { createV2Reducer } from './v2-reducer.js';
/** Published push API: unknown provenance remains unknown. */
export function createV2Observer(options) {
  const reducer = createV2Reducer(options);
  return { observe: reducer.observe, dispose: reducer.dispose };
}
