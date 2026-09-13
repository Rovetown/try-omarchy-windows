#!/usr/bin/env python3
"""Compile the real SDL drop collector against bounded event sequences."""
import argparse
from pathlib import Path
import subprocess
import tempfile

parser = argparse.ArgumentParser()
parser.add_argument('source', type=Path)
args = parser.parse_args()
source = args.source.read_text()
start = source.index('static strList *sdl_drop_files;')
end = source.index('void sdl2_poll_events(', start)
collector = source[start:end]
prefix = r'''
#include <assert.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>
typedef struct strList { char *value; struct strList *next; } strList;
static unsigned allocated, emitted, last_count;
static void *allocate(size_t size) { allocated++; return calloc(1,size); }
#define g_new0(type,count) ((type *)allocate(sizeof(type)*(count)))
#define g_strdup strdup
#define SDL_free free
static bool g_utf8_validate(const char *s,int n,void *end) { return true; }
static bool g_path_is_absolute(const char *s) { return s[0]=='/'; }
static void qapi_free_strList(strList *p) { while(p) { strList *next=p->next;free(p->value);free(p);allocated--;p=next; } }
enum {SDL_DROPBEGIN=1,SDL_DROPFILE,SDL_DROPCOMPLETE};
typedef struct {int type;struct {uint32_t windowID;char *file;} drop;} SDL_Event;
struct sdl2_console {struct {void *con;} dcl;};
static struct sdl2_console console;
static struct sdl2_console *get_scon_from_window(uint32_t window) {return window==1?&console:NULL;}
static int qemu_console_get_index(void *console) {return 0;}
static void qapi_event_send_display_file_drop(int display,strList *files) {
 emitted++;last_count=0;
 for (;files;files=files->next) {assert(files->value[0]=='/');last_count++;}
}
'''
suffix = r'''
static void event(int type,int window,const char *path) {
 SDL_Event ev={.type=type,.drop={.windowID=window,.file=path?strdup(path):NULL}};
 handle_file_drop(&ev);
}
int main(void) {
 event(SDL_DROPBEGIN,1,NULL);event(SDL_DROPFILE,1,"/first");event(SDL_DROPFILE,1,"/second");event(SDL_DROPCOMPLETE,1,NULL);
 assert(emitted==1&&last_count==2&&allocated==0);
 event(SDL_DROPBEGIN,1,NULL);event(SDL_DROPFILE,2,"/other-window");event(SDL_DROPCOMPLETE,1,NULL);assert(emitted==1&&allocated==0);
 event(SDL_DROPBEGIN,1,NULL);for(int i=0;i<1001;i++)event(SDL_DROPFILE,1,"/file");event(SDL_DROPCOMPLETE,1,NULL);assert(emitted==1&&allocated==0);
 event(SDL_DROPBEGIN,1,NULL);event(SDL_DROPFILE,1,"relative");event(SDL_DROPCOMPLETE,1,NULL);assert(emitted==1&&allocated==0);
 char large[32770];memset(large,'x',sizeof(large));large[0]='/';large[sizeof(large)-1]=0;
 event(SDL_DROPBEGIN,1,NULL);event(SDL_DROPFILE,1,large);event(SDL_DROPCOMPLETE,1,NULL);assert(emitted==1&&allocated==0);
 event(SDL_DROPBEGIN,1,NULL);event(SDL_DROPFILE,1,"/discarded");event(SDL_DROPBEGIN,1,NULL);assert(allocated==0);event(SDL_DROPFILE,1,"/kept");event(SDL_DROPCOMPLETE,1,NULL);assert(emitted==2&&last_count==1&&allocated==0);
 event(SDL_DROPBEGIN,1,NULL);large[32768]=0;for(int i=0;i<5;i++)event(SDL_DROPFILE,1,large);event(SDL_DROPCOMPLETE,1,NULL);assert(emitted==2&&allocated==0);
 puts("ok - bounded native SDL file drops");return 0;
}
'''
with tempfile.TemporaryDirectory() as directory:
    root=Path(directory)
    (root/'test.c').write_text(prefix+collector+suffix)
    subprocess.run(['cc', '-std=gnu11', '-Wall', '-Werror', str(root/'test.c'), '-o', str(root/'test')],check=True)
    subprocess.run([str(root/'test')],check=True)
