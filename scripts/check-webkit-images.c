#include <gdk-pixbuf/gdk-pixbuf.h>

int main(int argc, char **argv)
{
    if (argc != 2)
        return 2;
    GError *error = NULL;
    GdkPixbuf *image = gdk_pixbuf_new_from_file(argv[1], &error);
    if (!image) {
        g_printerr("Chargement de l'icône SVG impossible : %s\n", error->message);
        g_error_free(error);
        return 1;
    }
    g_object_unref(image);
    g_print("Icône SVG GTK chargée avec le SDK local.\n");
    return 0;
}
