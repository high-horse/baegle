import {advancedCopy, type CopySelection, type AdvancedCopyOptions} from './gridConvenience';
self.addEventListener('message', (event: MessageEvent<{selection: CopySelection; options: AdvancedCopyOptions}>) => {
    try {self.postMessage({ok: true, text: advancedCopy(event.data.selection, event.data.options)});}
    catch (error) {self.postMessage({ok: false, error: error instanceof Error ? error.message : String(error)});}
});
