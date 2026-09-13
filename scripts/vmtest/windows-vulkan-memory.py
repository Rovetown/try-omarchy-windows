#!/usr/bin/env python3
"""Compare native buffer memory types with and without Win32 external handles."""
import sys,json
import argparse
parser = argparse.ArgumentParser(description='Native Windows Vulkan acceptance diagnostic; does not modify guest settings.')
parser.add_argument('--vulkan-python-path', help='Optional isolated directory containing the Python vulkan package')
options = parser.parse_args()
if sys.platform != 'win32':
    parser.error('Run this diagnostic on the Windows graphics host')
if options.vulkan_python_path:
    sys.path.insert(0, options.vulkan_python_path)
import vulkan as v
instance=v.vkCreateInstance(v.VkInstanceCreateInfo(pApplicationInfo=v.VkApplicationInfo(pApplicationName='Try Omarchy acceptance memory probe',apiVersion=v.VK_MAKE_VERSION(1,1,0))),None)
results=[]
try:
 for pd in v.vkEnumeratePhysicalDevices(instance):
  props=v.vkGetPhysicalDeviceProperties(pd)
  mem=v.vkGetPhysicalDeviceMemoryProperties(pd)
  item={'device':props.deviceName,'memoryTypes':[{'index':i,'flags':int(mem.memoryTypes[i].propertyFlags),'heap':int(mem.memoryTypes[i].heapIndex)} for i in range(mem.memoryTypeCount)],'buffers':[]}
  exts=[x.extensionName for x in v.vkEnumerateDeviceExtensionProperties(pd,None)]
  if 'VK_KHR_external_memory_win32' not in exts:
   item['skip']='No VK_KHR_external_memory_win32';results.append(item);continue
  queues=v.vkGetPhysicalDeviceQueueFamilyProperties(pd)
  qi=next(i for i,q in enumerate(queues) if q.queueFlags & v.VK_QUEUE_GRAPHICS_BIT)
  dev=v.vkCreateDevice(pd,v.VkDeviceCreateInfo(pQueueCreateInfos=[v.VkDeviceQueueCreateInfo(queueFamilyIndex=qi,queueCount=1,pQueuePriorities=[1.0])],ppEnabledExtensionNames=['VK_KHR_external_memory_win32']),None)
  try:
   for label,handle in [('ordinary',0),('opaque_win32',v.VK_EXTERNAL_MEMORY_HANDLE_TYPE_OPAQUE_WIN32_BIT)]:
    ext=v.VkExternalMemoryBufferCreateInfo(handleTypes=handle) if handle else None
    buf=v.vkCreateBuffer(dev,v.VkBufferCreateInfo(pNext=ext,size=65536,usage=v.VK_BUFFER_USAGE_TRANSFER_SRC_BIT,sharingMode=v.VK_SHARING_MODE_EXCLUSIVE),None)
    req=v.vkGetBufferMemoryRequirements(dev,buf)
    item['buffers'].append({'kind':label,'size':int(req.size),'alignment':int(req.alignment),'memoryTypeBits':hex(req.memoryTypeBits),'hostVisibleTypes':[i for i in range(mem.memoryTypeCount) if req.memoryTypeBits & (1<<i) and mem.memoryTypes[i].propertyFlags & v.VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT]})
    v.vkDestroyBuffer(dev,buf,None)
  finally:v.vkDestroyDevice(dev,None)
  results.append(item)
finally:v.vkDestroyInstance(instance,None)
print(json.dumps(results,indent=2))
