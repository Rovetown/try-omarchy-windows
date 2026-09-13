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
        buf = memory = pool = fence = None
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
            backing = ctypes.create_string_buffer(size + alignment)
            address = ((ctypes.addressof(backing) + alignment - 1) // alignment) * alignment
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
            results.append({'device': props.deviceName, 'result': 'pass', 'alignment': alignment,
                            'bytesVerified': 65536, 'memoryType': choices[0],
                            'scope': 'native host pointer import, bind, GPU fill and coherent CPU read; not guest Venus integration'})
        finally:
            v.vkDeviceWaitIdle(dev)
            if fence is not None: v.vkDestroyFence(dev, fence, None)
            if pool is not None: v.vkDestroyCommandPool(dev, pool, None)
            if buf is not None: v.vkDestroyBuffer(dev, buf, None)
            if memory is not None: v.vkFreeMemory(dev, memory, None)
            v.vkDestroyDevice(dev, None)
finally:
    v.vkDestroyInstance(instance, None)
print(json.dumps(results, indent=2))
