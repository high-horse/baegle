import type { EditorView } from '@codemirror/view';

export interface ConsoleViewport {
    top: number;
    left: number;
    anchor: number;
    head: number;
}

export function trackConsoleViewport(view: EditorView, initial: ConsoleViewport | undefined, save: (value: ConsoleViewport) => void) {
    let snapshot = initial;
    let active = true;
    let restoring = false;
    let destroyed = false;
    function capture() {
        if (!active || restoring || destroyed || !view.dom.isConnected) return;
        const {anchor, head} = view.state.selection.main;
        const top = view.scrollDOM.scrollTop, left = view.scrollDOM.scrollLeft;
        if (snapshot?.top === top && snapshot.left === left && snapshot.anchor === anchor && snapshot.head === head) return;
        snapshot = {top, left, anchor, head};
        save(snapshot);
    }
    function restore() {
        active = true;
        if (!snapshot || destroyed) return;
        restoring = true;
        const saved = snapshot;
        const length = view.state.doc.length;
        const anchor = Math.min(saved.anchor, length), head = Math.min(saved.head, length);
        if (view.state.selection.main.anchor !== anchor || view.state.selection.main.head !== head) {
            view.dispatch({selection: {anchor, head}});
        }
        view.requestMeasure({
            key: view,
            read: () => saved,
            write: value => {
                if (destroyed || !active) return;
                view.scrollDOM.scrollTop = value.top;
                view.scrollDOM.scrollLeft = value.left;
                restoring = false;
            },
        });
    }
    view.scrollDOM.addEventListener('scroll', capture, {passive: true});
    return {
        capture,
        restore,
        deactivate() { active = false; },
        destroy() { destroyed = true; view.scrollDOM.removeEventListener('scroll', capture); },
    };
}
