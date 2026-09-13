#!/usr/bin/env python3
"""Query image compatibility before choosing a Windows Venus memory backend."""
import argparse
import json
import sys

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--vulkan-python-path')
options = parser.parse_args()
if sys.platform != 'win32':
    parser.error('Run on the Windows graphics host')
if options.vulkan_python_path:
    sys.path.insert(0, options.vulkan_python_path)
import vulkan as v

instance = v.vkCreateInstance(v.VkInstanceCreateInfo(pApplicationInfo=v.VkApplicationInfo(
    pApplicationName='Try Omarchy image memory compatibility', apiVersion=v.VK_MAKE_VERSION(1, 1, 0))), None)
results = []
try:
    for physical in v.vkEnumeratePhysicalDevices(instance):
        extensions = {e.extensionName for e in v.vkEnumerateDeviceExtensionProperties(physical, None)}
        enabled = [e for e in ('VK_KHR_external_memory_win32', 'VK_EXT_external_memory_host') if e in extensions]
        kinds = [('ordinary', 0)]
        if 'VK_KHR_external_memory_win32' in enabled:
            kinds.append(('opaque_win32', v.VK_EXTERNAL_MEMORY_HANDLE_TYPE_OPAQUE_WIN32_BIT))
        if 'VK_EXT_external_memory_host' in enabled:
            kinds.append(('host_allocation', v.VK_EXTERNAL_MEMORY_HANDLE_TYPE_HOST_ALLOCATION_BIT_EXT))
        queue = next(i for i, q in enumerate(v.vkGetPhysicalDeviceQueueFamilyProperties(physical)) if q.queueFlags & v.VK_QUEUE_GRAPHICS_BIT)
        device = v.vkCreateDevice(physical, v.VkDeviceCreateInfo(
            pQueueCreateInfos=[v.VkDeviceQueueCreateInfo(queueFamilyIndex=queue, queueCount=1, pQueuePriorities=[1.0])],
            ppEnabledExtensionNames=enabled), None)
        memory = v.vkGetPhysicalDeviceMemoryProperties(physical)
        rows = []
        try:
            for tiling_name, tiling in [('linear', v.VK_IMAGE_TILING_LINEAR), ('optimal', v.VK_IMAGE_TILING_OPTIMAL)]:
                for kind, handle in kinds:
                    row = {'tiling': tiling_name, 'kind': kind}
                    external = v.VkExternalImageFormatProperties() if handle else None
                    properties = v.VkImageFormatProperties2(pNext=external)
                    usage = v.VK_IMAGE_USAGE_TRANSFER_SRC_BIT | v.VK_IMAGE_USAGE_TRANSFER_DST_BIT | v.VK_IMAGE_USAGE_SAMPLED_BIT
                    try:
                        v.vkGetPhysicalDeviceImageFormatProperties2(physical, v.VkPhysicalDeviceImageFormatInfo2(
                            pNext=v.VkPhysicalDeviceExternalImageFormatInfo(handleType=handle) if handle else None,
                            format=v.VK_FORMAT_R8G8B8A8_UNORM, type=v.VK_IMAGE_TYPE_2D, tiling=tiling, usage=usage), properties)
                    except v.VkErrorFormatNotSupported:
                        row['supported'] = False
                        rows.append(row)
                        continue
                    row['supported'] = True
                    if external is not None:
                        row['externalFeatures'] = hex(external.externalMemoryProperties.externalMemoryFeatures)
                        row['compatibleHandleTypes'] = hex(external.externalMemoryProperties.compatibleHandleTypes)
                    image = v.vkCreateImage(device, v.VkImageCreateInfo(
                        pNext=v.VkExternalMemoryImageCreateInfo(handleTypes=handle) if handle else None,
                        imageType=v.VK_IMAGE_TYPE_2D, format=v.VK_FORMAT_R8G8B8A8_UNORM,
                        extent=v.VkExtent3D(width=64, height=64, depth=1), mipLevels=1, arrayLayers=1,
                        samples=v.VK_SAMPLE_COUNT_1_BIT, tiling=tiling, usage=usage,
                        sharingMode=v.VK_SHARING_MODE_EXCLUSIVE, initialLayout=v.VK_IMAGE_LAYOUT_UNDEFINED), None)
                    try:
                        requirements = v.vkGetImageMemoryRequirements(device, image)
                        row['memoryTypeBits'] = hex(requirements.memoryTypeBits)
                        row['hostVisibleTypes'] = [i for i in range(memory.memoryTypeCount)
                            if requirements.memoryTypeBits & (1 << i) and memory.memoryTypes[i].propertyFlags & v.VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT]
                    finally:
                        v.vkDestroyImage(device, image, None)
                    rows.append(row)
        finally:
            v.vkDestroyDevice(device, None)
        results.append({'device': v.vkGetPhysicalDeviceProperties(physical).deviceName,
                        'scope': 'native RGBA8 image creation and memory requirements; no guest or allocation pass', 'images': rows})
finally:
    v.vkDestroyInstance(instance, None)
print(json.dumps(results, indent=2))
