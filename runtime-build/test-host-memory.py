#!/usr/bin/env python3
"""Exercise the actual Windows host allocation helpers with a Vulkan stub."""
import argparse
from pathlib import Path
import subprocess
import tempfile

parser = argparse.ArgumentParser()
parser.add_argument('source', type=Path)
args = parser.parse_args()
source = (args.source / 'src/venus/vkr_device_memory.c').read_text(encoding='utf-8')

def function(name, result):
    start = source.index('\n' + name + '(') + 1
    brace = source.index('{', start)
    depth, end = 1, brace + 1
    while depth:
        depth += (source[end] == '{') - (source[end] == '}')
        end += 1
    return result + ' ' + source[start:end] + '\n'

harness = r'''
#include <windows.h>
#include <vulkan/vulkan.h>
#include <assert.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>
#define MAX2(a,b) ((a)>(b)?(a):(b))
struct vkr_host_memory { struct vkr_host_memory *next; int fd; void *mapping; };
struct physical { uint64_t min_host_pointer_alignment; };
struct vkr_device {
 struct physical *physical_device;
 struct { struct { VkDevice device; } handle; } base;
 PFN_vkGetMemoryHostPointerPropertiesEXT GetMemoryHostPointerPropertiesEXT;
 struct vkr_host_memory *host_memories;
};
static HANDLE owned;
static int wrap_failure;
static uint32_t supported = 2;
static VkResult result = VK_SUCCESS;
static int getpagesize(void) { return 4096; }
static int os_wrap_win32_handle(HANDLE h) {
 if (wrap_failure) return -1;
 assert(!owned); owned=h; return 100;
}
static int os_close_fd(int fd) { assert(fd==100 && owned); CloseHandle(owned); owned=NULL; return 0; }
static VKAPI_ATTR VkResult VKAPI_CALL properties(VkDevice device,
 VkExternalMemoryHandleTypeFlagBits type, const void *ptr, VkMemoryHostPointerPropertiesEXT *props) {
 (void)device;
 assert(type==VK_EXTERNAL_MEMORY_HANDLE_TYPE_HOST_ALLOCATION_BIT_EXT);
 assert(ptr && !((uintptr_t)ptr % 4096));
 props->memoryTypeBits=supported; return result;
}
'''
harness += function('vkr_host_memory_destroy', 'static void')
harness += function('vkr_device_memory_release_host_backings', 'void')
harness += function('vkr_host_memory_create', 'static VkResult')
harness += r'''
#define TRACE_FUNC() ((void)0)
struct vkr_device_memory { struct vkr_device *device; int base; };
struct vn_dispatch_context { void *data; };
struct vn_command_vkFreeMemory { struct vkr_device_memory *memory; };
static int driver_freed, backing_released, object_removed;
static struct vkr_device_memory *vkr_device_memory_from_handle(struct vkr_device_memory *mem) { return mem; }
static void vkr_device_memory_destroy_driver_handle(void *ctx, struct vn_command_vkFreeMemory *args) {
 (void)ctx; assert(args->memory && !backing_released); driver_freed=1; args->memory=NULL;
}
static void vkr_device_memory_release(struct vkr_device_memory *mem) {
 assert(mem && driver_freed); backing_released=1;
}
static void vkr_device_remove_object(void *ctx, struct vkr_device *dev, int *base) {
 (void)ctx; assert(dev && base && backing_released); object_removed=1;
}
'''
harness += function('vkr_dispatch_vkFreeMemory', 'static void')
harness += r'''
int main(void) {
 struct physical physical={4096};
 struct vkr_device dev={.physical_device=&physical,.GetMemoryHostPointerPropertiesEXT=properties};
 VkMemoryAllocateFlagsInfo flags={.sType=VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_FLAGS_INFO};
 VkMemoryAllocateInfo original={.sType=VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO,
  .pNext=&flags,.allocationSize=17,.memoryTypeIndex=1};
 VkMemoryAllocateInfo info=original;
 VkImportMemoryHostPointerInfoEXT import;
 struct vkr_host_memory *host=NULL;
 DWORD before,after;
 GetProcessHandleCount(GetCurrentProcess(),&before);
 assert(vkr_host_memory_create(&dev,&info,&host,&import)==VK_SUCCESS);
 assert(host && host->mapping && host->fd==100 && info.allocationSize==4096);
 assert(info.pNext==&import && import.pNext==&flags && import.pHostPointer==host->mapping);
 void *view=MapViewOfFile(owned,FILE_MAP_READ|FILE_MAP_WRITE,0,0,4096);
 assert(view && view!=host->mapping);
 memset(host->mapping,0x5a,4096);
 assert(!memcmp(view,host->mapping,4096));
 dev.host_memories=host;
 vkr_device_memory_release_host_backings(&dev);
 assert(!dev.host_memories && !owned);
 assert(((unsigned char *)view)[4095]==0x5a); // Other owners retain their own view.
 UnmapViewOfFile(view);

 host=NULL; info=original; supported=1;
 assert(vkr_host_memory_create(&dev,&info,&host,&import)==VK_ERROR_INVALID_EXTERNAL_HANDLE);
 assert(!host && !owned && info.pNext==&flags);
 supported=2; result=VK_ERROR_DEVICE_LOST; info=original;
 assert(vkr_host_memory_create(&dev,&info,&host,&import)==VK_ERROR_DEVICE_LOST);
 assert(!host && !owned);
 result=VK_SUCCESS; wrap_failure=1; info=original;
 assert(vkr_host_memory_create(&dev,&info,&host,&import)==VK_ERROR_OUT_OF_HOST_MEMORY);
 assert(!host && !owned);
 wrap_failure=0; info=original; info.allocationSize=UINT64_MAX;
 assert(vkr_host_memory_create(&dev,&info,&host,&import)==VK_ERROR_OUT_OF_HOST_MEMORY);
 info=original; info.allocationSize=0;
 assert(vkr_host_memory_create(&dev,&info,&host,&import)==VK_ERROR_OUT_OF_HOST_MEMORY);
 struct vn_dispatch_context dispatch={0};
 struct vkr_device_memory memory={.device=&dev};
 struct vn_command_vkFreeMemory free_args={.memory=&memory};
 vkr_dispatch_vkFreeMemory(&dispatch,&free_args);
 assert(driver_freed && backing_released && object_removed);
 GetProcessHandleCount(GetCurrentProcess(),&after);
 assert(before==after);
 puts("PASS: host allocation alignment, shared ownership, incompatible types, overflow and failure cleanup");
}
'''
with tempfile.TemporaryDirectory(prefix='venus-host-memory-') as tmp:
    src = Path(tmp) / 'test.c'
    exe = Path(tmp) / 'test.exe'
    src.write_text(harness, encoding='utf-8')
    subprocess.run(['cc', '-std=c11', '-Wall', '-Wextra', str(src), '-o', str(exe)], check=True)
    subprocess.run([str(exe)], check=True)
