#include "neotags.h"
#include <dirent.h>
#include <inttypes.h>
#include <sys/stat.h>

#define STARTSIZE 1024
#define GUESS 100
#define INC   10

#ifdef DOSISH
#  define restrict __restrict
#endif

#define SAFE_STAT(PATH, ST)                                     \
     do {                                                       \
             if ((stat((PATH), (ST)) != 0))                     \
                     err(1, "Failed to stat file '%s", (PATH)); \
     } while (0)

static const char program_name[] = "tag_highlight";

static bool file_is_reg(const char *filename);


FILE *
safe_fopen(const char *filename, const char *mode)
{
        FILE *fp = fopen(filename, mode);
        if (!fp)
                err(1, "Failed to open file \"%s\"", filename);
        if (!file_is_reg(filename))
                errx(1, "Invalid filetype \"%s\"\n", filename);
        return fp;
}

FILE *
safe_fopen_fmt(const char *const restrict fmt,
               const char *const restrict mode,
               ...)
{
        va_list va;
        va_start(va, mode);
        char buf[PATH_MAX + 1];
        vsnprintf(buf, PATH_MAX + 1, fmt, va);
        va_end(va);

        FILE *fp = fopen(buf, mode);
        if (!fp)
                err(1, "Failed to open file \"%s\"", buf);
        if (!file_is_reg(buf))
                errx(1, "Invalid filetype \"%s\"\n", buf);

        return fp;
}


int
safe_open(const char *const filename, const int flags, const int mode)
{
#ifdef DOSISH
        const int fd = open(filename, flags, _S_IREAD|_S_IWRITE);
#else
        const int fd = open(filename, flags, mode);
#endif
        if (fd == (-1)) {
                fprintf(stderr, "Failed to open file \"%s\": %s\n", filename, strerror(errno));
                abort();
        }
        return fd;
}


int
safe_open_fmt(const char *const restrict fmt,
              const int flags, const int mode, ...)
{
        va_list va;
        va_start(va, mode);
        char buf[PATH_MAX + 1];
        vsnprintf(buf, PATH_MAX + 1, fmt, va);
        va_end(va);

        errno = 0;
#ifdef DOSISH
        const int fd = open(buf, flags, _S_IREAD|_S_IWRITE);
#else
        const int fd = open(buf, flags, mode);
#endif
        if (fd == (-1)) {
                fprintf(stderr, "Failed to open file \"%s\": %s\n", buf, strerror(errno));
                abort();
        }

        return fd;
}


bool
file_is_reg(const char *filename)
{
        struct stat st;
        SAFE_STAT(filename, &st);
        return S_ISREG(st.st_mode) || S_ISFIFO(st.st_mode);
}


#ifdef USE_XMALLOC
void *
xmalloc(const size_t size)
{
        void *tmp = malloc(size);
        if (tmp == NULL)
                err(100, "Malloc call failed - attempted %zu bytes", size);
        return tmp;
}


void *
xcalloc(const int num, const size_t size)
{
        void *tmp = calloc(num, size);
        if (tmp == NULL)
                err(101, "Calloc call failed - attempted %zu bytes", size);
        return tmp;
}
#endif


void *
xrealloc(void *ptr, const size_t size)
{
        void *tmp = realloc(ptr, size);
        if (tmp == NULL)
                err(102, "Realloc call failed - attempted %zu bytes", size);
        return tmp;
}


#ifdef HAVE_REALLOCARRAY
void *
xreallocarray(void *ptr, size_t num, size_t size)
{
        void *tmp = reallocarray(ptr, num, size);
        if (tmp == NULL)
                err(103, "Realloc call failed - attempted %zu bytes", size);
        return tmp;
}
#endif

int64_t
__xatoi(const char *const str, const bool strict)
{
        char *endptr;
        const long long int val = strtol(str, &endptr, 10);

        if ((endptr == str) || (strict && *endptr != '\0'))
                errx(30, "Invalid integer \"%s\".\n", str);

        return (int)val;
}


#ifdef DOSISH
char *
basename(char *path)
{
        assert(path != NULL && *path != '\0');
        const size_t len = strlen(path);
        char *ptr = path + len;
        while (*ptr != '/' && *ptr != '\\' && ptr != path)
                --ptr;
        
        return (*ptr == '/' || *ptr == '\\') ? ptr + 1 : ptr;
}
#endif


/* #ifndef HAVE_ERR */
#define ERRSTACKSIZE (6384)
void
__err(UNUSED const int status, const bool print_err, const char *const __restrict fmt, ...)
{
        va_list ap;
        va_start(ap, fmt);
        char buf[ERRSTACKSIZE];

        if (print_err)
                snprintf(buf, ERRSTACKSIZE, "%s: %s: %s\n", program_name, fmt, strerror(errno));
        else
                snprintf(buf, ERRSTACKSIZE, "%s: %s\n", program_name, fmt);

        vfprintf(stderr, buf, ap);
        va_end(ap);

        abort();
        /* exit(status); */
}


void
__warn(const bool print_err, const char *const __restrict fmt, ...)
{
        va_list ap1, ap2;
        va_start(ap1, fmt);
        va_start(ap2, fmt);
        char buf[ERRSTACKSIZE];

        if (print_err)
                snprintf(buf, ERRSTACKSIZE, "%s: %s: %s\n", program_name, fmt, strerror(errno));
        else
                snprintf(buf, ERRSTACKSIZE, "%s: %s\n", program_name, fmt);

        vfprintf(stderr, buf, ap1);

        va_end(ap1);
        va_end(ap2);
}


int
find_num_cpus(void)
{
#if defined(DOSISH)
        SYSTEM_INFO sysinfo;
        GetSystemInfo(&sysinfo);
        return sysinfo.dwNumberOfProcessors;
#elif defined(MACOS)
        int nm[2];
        size_t len = 4;
        uint32_t count;

        nm[0] = CTL_HW; nm[1] = HW_AVAILCPU;
        sysctl(nm, 2, &count, &len, NULL, 0);

        if (count < 1) {
                nm[1] = HW_NCPU;
                sysctl(nm, 2, &count, &len, NULL, 0);
                if (count < 1) { count = 1; }
        }
        return count;
#elif defined(__unix__) || defined(__linux__) || defined(BSD)
        return sysconf(_SC_NPROCESSORS_ONLN);
#else
#  error "Cannot determine operating system."
#endif
}
