import {createCopyExecutor} from './copyExecution';
export const formatCopySelection = createCopyExecutor(() => typeof Worker === 'undefined' ? undefined
    : new Worker(new URL('./copyWorker.ts', import.meta.url), {type: 'module'}));
