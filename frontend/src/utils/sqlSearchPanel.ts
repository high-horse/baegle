import { createVNode, render, shallowRef, type AppContext } from "vue";
import { runScopeHandlers, type EditorView, type Panel } from "@codemirror/view";
import { getSearchQuery } from "@codemirror/search";
import SqlSearchPanel from "@/components/workspace/SqlSearchPanel.vue";

// Keep CodeMirror's search state and commands, with the app's Vue/Quasar UI.
export function createSQLSearchPanel(view: EditorView, appContext: AppContext | null): Panel {
    const dom = document.createElement('div');
    const query = shallowRef(getSearchQuery(view.state));
    const readOnly = shallowRef(view.state.readOnly);
    const doc = shallowRef(view.state.doc);
    const selection = shallowRef(view.state.selection);
    const vnode = createVNode(SqlSearchPanel, { view, query, readOnly, doc, selection });
    vnode.appContext = appContext;
    render(vnode, dom);
    // CodeMirror focuses this actual input when Ctrl+F opens the panel again.
    const input = dom.querySelector('input');
    input?.setAttribute('main-field', 'true');
    dom.addEventListener('keydown', event => {
        if (runScopeHandlers(view, event, 'search-panel')) event.preventDefault();
        // Panel keys must not trigger editor actions such as executing SQL.
        event.stopPropagation();
    });
    return {
        dom,
        top: true,
        mount() { input?.select(); },
        update(update) {
            query.value = getSearchQuery(update.state);
            readOnly.value = update.state.readOnly;
            doc.value = update.state.doc;
            selection.value = update.state.selection;
        },
        destroy() { render(null, dom); },
    };
}
