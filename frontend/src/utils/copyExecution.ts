import { advancedCopy, type CopySelection, type AdvancedCopyOptions } from './gridConvenience';

type Reply = {ok: true; text: string} | {ok: false; error: string};
export interface CopyWorker {
    postMessage(value: {selection: CopySelection; options: AdvancedCopyOptions}): void;
    addEventListener(type: string, listener: (event: any) => void): void;
    terminate(): void;
}
export function createCopyExecutor(createWorker: () => CopyWorker | undefined) {
    return async (selection: CopySelection, options: AdvancedCopyOptions): Promise<string> => {
        if (selection.rows.length * selection.columns.length < 20000) return advancedCopy(selection, options);
        let worker: CopyWorker | undefined;
        try { worker = createWorker(); } catch { /* Older webviews can use the synchronous formatter. */ }
        if (!worker) return advancedCopy(selection, options);
        return new Promise<string>((resolve, reject) => {
            let finished = false;
            function finish(task: () => string) {
                if (finished) return;
                finished = true;
                worker!.terminate();
                try {resolve(task());} catch (error) {reject(error);}
            }
            worker!.addEventListener('message', (event: MessageEvent<Reply>) => {
                finish(() => {
                    if (!event.data.ok) throw new Error(event.data.error);
                    return event.data.text;
                });
            });
            const fallback = () => finish(() => advancedCopy(selection, options));
            worker!.addEventListener('error', fallback);
            worker!.addEventListener('messageerror', fallback);
            try {worker!.postMessage({selection, options});} catch {fallback();}
        });
    };
}
