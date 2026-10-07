/* Distribution WebKit builds ignore WEBKIT_EXEC_PATH outside developer mode.
 * Relocate their GIO launch arguments without changing the packaged library.
 * See WebKit's ProcessExecutablePathGLib.cpp and ProcessLauncherGLib.cpp.
 */
#define _GNU_SOURCE
#include <dlfcn.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

typedef void *(*Spawn)(void *, const char *const *, void **);

static char *relocate(const char *argument, const char *directory)
{
    static const char *prefixes[] = {
        "/usr/lib/x86_64-linux-gnu/webkit2gtk-4.1",
        "/usr/lib64/webkit2gtk-4.1",
        "/usr/libexec/webkit2gtk-4.1",
        "/usr/lib/webkit2gtk-4.1",
    };
    for (size_t i = 0; i < sizeof(prefixes) / sizeof(prefixes[0]); ++i) {
        size_t length = strlen(prefixes[i]);
        if (strncmp(argument, prefixes[i], length) != 0)
            continue;
        const char *suffix = argument + length;
        if (*suffix && strcmp(suffix, "/WebKitNetworkProcess") &&
            strcmp(suffix, "/WebKitWebProcess") && strcmp(suffix, "/WebKitGPUProcess"))
            continue;
        char *result = malloc(strlen(directory) + strlen(suffix) + 1);
        if (!result)
            abort();
        strcpy(result, directory);
        strcat(result, suffix);
        return result;
    }
    return NULL;
}

/* Opaque GIO types keep this shim independent of distribution GLib headers. */
void *g_subprocess_launcher_spawnv(void *launcher, const char *const *argv, void **error)
{
    Spawn spawn = (Spawn)dlsym(RTLD_NEXT, "g_subprocess_launcher_spawnv");
    if (!spawn) {
        fputs("42 TAP: cannot resolve the GIO process launcher\n", stderr);
        abort();
    }
    const char *directory = getenv("TAP_WEBKIT_EXEC_PATH");
    if (!directory || !*directory || !argv)
        return spawn(launcher, argv, error);
    size_t count = 0;
    while (argv[count])
        ++count;
    const char **local = calloc(count + 1, sizeof(*local));
    if (!local)
        abort();
    for (size_t i = 0; i < count; ++i) {
        char *replacement = relocate(argv[i], directory);
        local[i] = replacement ? replacement : argv[i];
    }
    void *process = spawn(launcher, local, error);
    for (size_t i = 0; i < count; ++i)
        if (local[i] != argv[i])
            free((void *)local[i]);
    free(local);
    return process;
}
