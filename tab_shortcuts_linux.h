#include <gtk/gtk.h>
#if GTK_MAJOR_VERSION >= 4
#include <webkit/webkit.h>
#else
#include <webkit2/webkit2.h>
#define GDK_ALT_MASK GDK_MOD1_MASK
#endif

// GTK can use Ctrl+Tab to move focus out of the webview before the DOM sees it.
// Consume it at the window, and deliver exactly one event to the existing DOM
// handler. Plain Tab and shortcuts with unrelated modifiers remain untouched.
static gboolean dispatch_tab_shortcut(WebKitWebView *view, guint key, GdkModifierType state) {
    if ((key != GDK_KEY_Tab && key != GDK_KEY_ISO_Left_Tab) ||
        !(state & GDK_CONTROL_MASK) ||
        (state & (GDK_ALT_MASK | GDK_SUPER_MASK | GDK_META_MASK))) return FALSE;
    gboolean backwards = (state & GDK_SHIFT_MASK) || key == GDK_KEY_ISO_Left_Tab;
    const char *script = backwards
        ? "window.dispatchEvent(new KeyboardEvent('keydown',{key:'Tab',code:'Tab',ctrlKey:true,shiftKey:true,cancelable:true}))"
        : "window.dispatchEvent(new KeyboardEvent('keydown',{key:'Tab',code:'Tab',ctrlKey:true,cancelable:true}))";
    webkit_web_view_evaluate_javascript(view, script, -1, NULL, NULL, NULL, NULL, NULL);
    return TRUE;
}

static WebKitWebView *find_tab_webview(GtkWidget *widget) {
    if (WEBKIT_IS_WEB_VIEW(widget)) return WEBKIT_WEB_VIEW(widget);
#if GTK_MAJOR_VERSION >= 4
    for (GtkWidget *child = gtk_widget_get_first_child(widget); child; child = gtk_widget_get_next_sibling(child)) {
        WebKitWebView *view = find_tab_webview(child);
        if (view) return view;
    }
#else
    if (GTK_IS_CONTAINER(widget)) {
        GList *children = gtk_container_get_children(GTK_CONTAINER(widget));
        WebKitWebView *view = NULL;
        for (GList *child = children; child && !view; child = child->next) view = find_tab_webview(GTK_WIDGET(child->data));
        g_list_free(children);
        if (view) return view;
    }
#endif
    return NULL;
}

#if GTK_MAJOR_VERSION >= 4
static gboolean tab_key_pressed(GtkEventControllerKey *controller, guint key, guint code, GdkModifierType state, WebKitWebView *view) {
    return dispatch_tab_shortcut(view, key, state);
}
#else
static gboolean tab_key_pressed(GtkWidget *widget, GdkEventKey *event, WebKitWebView *view) {
    return dispatch_tab_shortcut(view, event->keyval, event->state);
}
#endif

static void install_tab_shortcuts(GtkWidget *window) {
    if (g_object_get_data(G_OBJECT(window), "db-viewer-tab-shortcuts")) return;
    WebKitWebView *view = find_tab_webview(window);
    if (!view) return;
#if GTK_MAJOR_VERSION >= 4
    GtkEventController *controller = gtk_event_controller_key_new();
    gtk_event_controller_set_propagation_phase(controller, GTK_PHASE_CAPTURE);
    g_signal_connect_object(controller, "key-pressed", G_CALLBACK(tab_key_pressed), view, 0);
    gtk_widget_add_controller(window, controller);
#else
    g_signal_connect_object(window, "key-press-event", G_CALLBACK(tab_key_pressed), view, 0);
#endif
    g_object_set_data(G_OBJECT(window), "db-viewer-tab-shortcuts", GINT_TO_POINTER(1));
}
