#!/usr/bin/env python3
"""Verify native host-pointer import and synchronized GPU-to-host buffer writes."""
import ctypes, json, sys
import argparse
parser = argparse.ArgumentParser(description='Native Windows Vulkan acceptance diagnostic; does not modify guest settings.')
parser.add_argument('--vulkan-python-path', help='Optional isolated directory containing the Python vulkan package')
options = parser.parse_args()
if sys.platform != 'win32':
    parser.error('Run this diagnostic on the Windows graphics host')
if options.vulkan_python_path:
    sys.path.insert(0, options.vulkan_python_path)
import vulkan as v

class SharedBacking:
    """Two independent views of a pagefile-backed Windows section, no disk file."""
    def __init__(self, size, alignment):
        from ctypes import wintypes as w
        self.api = ctypes.WinDLL('kernel32', use_last_error=True)
        self.api.CreateFileMappingW.argtypes = [w.HANDLE, ctypes.c_void_p, w.DWORD, w.DWORD, w.DWORD, w.LPCWSTR]
        self.api.CreateFileMappingW.restype = w.HANDLE
        self.api.MapViewOfFile.argtypes = [w.HANDLE, w.DWORD, w.DWORD, w.DWORD, ctypes.c_size_t]
        self.api.MapViewOfFile.restype = ctypes.c_void_p
        self.api.UnmapViewOfFile.argtypes = [ctypes.c_void_p]
        self.api.UnmapViewOfFile.restype = w.BOOL
        self.api.CloseHandle.argtypes = [w.HANDLE]
        self.api.CloseHandle.restype = w.BOOL
        self.views = []
        self.section = self.api.CreateFileMappingW(w.HANDLE(-1), None, 4, size >> 32, size & 0xffffffff, None)
        if not self.section:
            raise ctypes.WinError(ctypes.get_last_error())
        try:
            for _ in range(2):
                address = self.api.MapViewOfFile(self.section, 6, 0, 0, size)
                if not address:
                    raise ctypes.WinError(ctypes.get_last_error())
                self.views.append(address)
                if address % alignment:
                    raise RuntimeError('Section view does not meet Vulkan host pointer alignment')
            assert self.views[0] != self.views[1]
            # Views retain the section after the original handle is closed.
            if not self.api.CloseHandle(self.section):
                raise ctypes.WinError(ctypes.get_last_error())
            self.section = None
        except BaseException:
            self.close()
            raise

    def close(self):
        for address in reversed(self.views):
            self.api.UnmapViewOfFile(address)
        self.views.clear()
        if self.section:
            self.api.CloseHandle(self.section)
            self.section = None

instance = v.vkCreateInstance(v.VkInstanceCreateInfo(pApplicationInfo=v.VkApplicationInfo(
    pApplicationName='Try Omarchy host import transfer acceptance',
    apiVersion=v.VK_MAKE_VERSION(1, 1, 0))), None)
results = []
try:
    for pd in v.vkEnumeratePhysicalDevices(instance):
        props = v.vkGetPhysicalDeviceProperties(pd)
        exts = [x.extensionName for x in v.vkEnumerateDeviceExtensionProperties(pd, None)]
        if 'VK_EXT_external_memory_host' not in exts:
            results.append({'device': props.deviceName, 'skip': 'external memory host unavailable'})
            continue
        hp = v.VkPhysicalDeviceExternalMemoryHostPropertiesEXT()
        v.vkGetPhysicalDeviceProperties2(pd, v.VkPhysicalDeviceProperties2(pNext=hp))
        alignment = int(hp.minImportedHostPointerAlignment)
        assert alignment > 0
        queues = v.vkGetPhysicalDeviceQueueFamilyProperties(pd)
        qi = next(i for i, q in enumerate(queues) if q.queueFlags & v.VK_QUEUE_GRAPHICS_BIT)
        dev = v.vkCreateDevice(pd, v.VkDeviceCreateInfo(
            pQueueCreateInfos=[v.VkDeviceQueueCreateInfo(queueFamilyIndex=qi, queueCount=1, pQueuePriorities=[1.0])],
            ppEnabledExtensionNames=['VK_EXT_external_memory_host']), None)
        buf = memory = pool = fence = backing = None
        try:
            handle = v.VK_EXTERNAL_MEMORY_HANDLE_TYPE_HOST_ALLOCATION_BIT_EXT
            external = v.vkGetPhysicalDeviceExternalBufferProperties(pd,
                v.VkPhysicalDeviceExternalBufferInfo(usage=v.VK_BUFFER_USAGE_TRANSFER_DST_BIT, handleType=handle))
            assert external.externalMemoryProperties.externalMemoryFeatures & v.VK_EXTERNAL_MEMORY_FEATURE_IMPORTABLE_BIT
            buf = v.vkCreateBuffer(dev, v.VkBufferCreateInfo(
                pNext=v.VkExternalMemoryBufferCreateInfo(handleTypes=handle), size=65536,
                usage=v.VK_BUFFER_USAGE_TRANSFER_DST_BIT, sharingMode=v.VK_SHARING_MODE_EXCLUSIVE), None)
            req = v.vkGetBufferMemoryRequirements(dev, buf)
            size = ((int(req.size) + alignment - 1) // alignment) * alignment
            backing = SharedBacking(size, alignment)
            address, second_address = backing.views
            ctypes.memset(second_address, 0x5a, 65536)
            assert ctypes.string_at(address, 65536) == b'\x5a' * 65536
            pointer = v.ffi.cast('void *', address)
            get_props = v.vkGetDeviceProcAddr(dev, 'vkGetMemoryHostPointerPropertiesEXT')
            pointer_props = get_props(dev, handle, pointer)
            mem = v.vkGetPhysicalDeviceMemoryProperties(pd)
            flags = v.VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | v.VK_MEMORY_PROPERTY_HOST_COHERENT_BIT
            choices = [i for i in range(mem.memoryTypeCount)
                       if req.memoryTypeBits & pointer_props.memoryTypeBits & (1 << i)
                       and mem.memoryTypes[i].propertyFlags & flags == flags]
            assert choices, 'No compatible coherent host-visible memory type'
            memory = v.vkAllocateMemory(dev, v.VkMemoryAllocateInfo(
                pNext=v.VkImportMemoryHostPointerInfoEXT(handleType=handle, pHostPointer=pointer),
                allocationSize=size, memoryTypeIndex=choices[0]), None)
            v.vkBindBufferMemory(dev, buf, memory, 0)
            pool = v.vkCreateCommandPool(dev, v.VkCommandPoolCreateInfo(queueFamilyIndex=qi), None)
            cmd = v.vkAllocateCommandBuffers(dev, v.VkCommandBufferAllocateInfo(
                commandPool=pool, level=v.VK_COMMAND_BUFFER_LEVEL_PRIMARY, commandBufferCount=1))[0]
            v.vkBeginCommandBuffer(cmd, v.VkCommandBufferBeginInfo(flags=v.VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT))
            v.vkCmdFillBuffer(cmd, buf, 0, 65536, 0x1234abcd)
            barrier = v.VkMemoryBarrier(srcAccessMask=v.VK_ACCESS_TRANSFER_WRITE_BIT, dstAccessMask=v.VK_ACCESS_HOST_READ_BIT)
            v.vkCmdPipelineBarrier(cmd, v.VK_PIPELINE_STAGE_TRANSFER_BIT, v.VK_PIPELINE_STAGE_HOST_BIT,
                                   0, 1, [barrier], 0, None, 0, None)
            v.vkEndCommandBuffer(cmd)
            fence = v.vkCreateFence(dev, v.VkFenceCreateInfo(), None)
            queue = v.vkGetDeviceQueue(dev, qi, 0)
            v.vkQueueSubmit(queue, 1, [v.VkSubmitInfo(pCommandBuffers=[cmd])], fence)
            v.vkWaitForFences(dev, 1, [fence], True, 5_000_000_000)
            actual = ctypes.string_at(address, 65536)
            assert actual == bytes.fromhex('cdab3412') * 16384, 'GPU write did not reach imported host memory'
            assert ctypes.string_at(second_address, 65536) == actual, 'GPU write did not reach independent section view'
            results.append({'device': props.deviceName, 'result': 'pass', 'alignment': alignment,
                            'bytesVerified': 65536, 'memoryType': choices[0],
                            'independentViewVerified': True, 'originalSectionHandleClosed': True,
                            'scope': 'native shared section import, bind, GPU fill and coherent reads through two views; not guest Venus integration'})
        finally:
            v.vkDeviceWaitIdle(dev)
            if fence is not None: v.vkDestroyFence(dev, fence, None)
            if pool is not None: v.vkDestroyCommandPool(dev, pool, None)
            if buf is not None: v.vkDestroyBuffer(dev, buf, None)
            if memory is not None: v.vkFreeMemory(dev, memory, None)
            if backing is not None: backing.close()
            v.vkDestroyDevice(dev, None)
finally:
    v.vkDestroyInstance(instance, None)
print(json.dumps(results, indent=2))
