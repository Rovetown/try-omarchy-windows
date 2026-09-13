/* Guest-side Vulkan memory regression. Run with an external timeout. */
#include <vulkan/vulkan.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#define CHECK(call) do { VkResult r=(call); printf("%s: %d\n",#call,r); fflush(stdout); if(r!=VK_SUCCESS) return 1; } while(0)

int main(int argc, char **argv) {
 (void)argv;
 VkInstance instance;
 VkApplicationInfo app={.sType=VK_STRUCTURE_TYPE_APPLICATION_INFO,.pApplicationName="Venus memory acceptance",.apiVersion=VK_API_VERSION_1_3};
 VkInstanceCreateInfo ici={.sType=VK_STRUCTURE_TYPE_INSTANCE_CREATE_INFO,.pApplicationInfo=&app};
 CHECK(vkCreateInstance(&ici,NULL,&instance));
 uint32_t count=0; VkPhysicalDevice physical=VK_NULL_HANDLE;
 CHECK(vkEnumeratePhysicalDevices(instance,&count,NULL));
 VkPhysicalDevice *devices=calloc(count,sizeof(*devices));
 CHECK(vkEnumeratePhysicalDevices(instance,&count,devices));
 for(uint32_t i=0;i<count;i++){
  VkPhysicalDeviceProperties candidate;vkGetPhysicalDeviceProperties(devices[i],&candidate);
  if(candidate.deviceType!=VK_PHYSICAL_DEVICE_TYPE_CPU){physical=devices[i];break;}
 }
 free(devices);if(!physical){puts("FAIL: no hardware Vulkan device");return 1;}
 VkPhysicalDeviceProperties props; vkGetPhysicalDeviceProperties(physical,&props);
 printf("device: %s\n",props.deviceName);
 count=0; vkGetPhysicalDeviceQueueFamilyProperties(physical,&count,NULL);
 VkQueueFamilyProperties *families=calloc(count,sizeof(*families));
 vkGetPhysicalDeviceQueueFamilyProperties(physical,&count,families);
 uint32_t family=0; while(family<count && !(families[family].queueFlags&VK_QUEUE_GRAPHICS_BIT))family++;
 free(families); if(family==count)return 1;
 float priority=1;
 VkDeviceQueueCreateInfo qci={.sType=VK_STRUCTURE_TYPE_DEVICE_QUEUE_CREATE_INFO,.queueFamilyIndex=family,.queueCount=1,.pQueuePriorities=&priority};
 VkDeviceCreateInfo dci={.sType=VK_STRUCTURE_TYPE_DEVICE_CREATE_INFO,.queueCreateInfoCount=1,.pQueueCreateInfos=&qci};
 VkDevice device; CHECK(vkCreateDevice(physical,&dci,NULL,&device));
 VkBufferCreateInfo bci={.sType=VK_STRUCTURE_TYPE_BUFFER_CREATE_INFO,.size=65536,.usage=VK_BUFFER_USAGE_TRANSFER_DST_BIT,.sharingMode=VK_SHARING_MODE_EXCLUSIVE};
 VkBuffer buffer; CHECK(vkCreateBuffer(device,&bci,NULL,&buffer));
 VkMemoryRequirements requirements; vkGetBufferMemoryRequirements(device,buffer,&requirements);
 VkDeviceBufferMemoryRequirements dbmr={.sType=VK_STRUCTURE_TYPE_DEVICE_BUFFER_MEMORY_REQUIREMENTS,.pCreateInfo=&bci};
 VkMemoryRequirements2 req2={.sType=VK_STRUCTURE_TYPE_MEMORY_REQUIREMENTS_2};
 vkGetDeviceBufferMemoryRequirements(device,&dbmr,&req2);
 printf("type masks: buffer=%x device-query=%x\n",requirements.memoryTypeBits,req2.memoryRequirements.memoryTypeBits);fflush(stdout);
 if(requirements.memoryTypeBits!=req2.memoryRequirements.memoryTypeBits)return 1;
 VkPhysicalDeviceMemoryProperties memory;vkGetPhysicalDeviceMemoryProperties(physical,&memory);
 uint32_t type=0,flags=VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT|VK_MEMORY_PROPERTY_HOST_COHERENT_BIT;
 while(type<memory.memoryTypeCount && (!(requirements.memoryTypeBits&(1u<<type)) || (memory.memoryTypes[type].propertyFlags&flags)!=flags))type++;
 if(type==memory.memoryTypeCount)return 1;
 VkMemoryAllocateInfo mai={.sType=VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO,.allocationSize=requirements.size,.memoryTypeIndex=type};
 VkDeviceMemory allocation; CHECK(vkAllocateMemory(device,&mai,NULL,&allocation));
 if(argc>3){
  VkBindBufferMemoryInfo bind={.sType=VK_STRUCTURE_TYPE_BIND_BUFFER_MEMORY_INFO,.buffer=buffer,.memory=allocation};
  CHECK(vkBindBufferMemory2(device,1,&bind));
 }else CHECK(vkBindBufferMemory(device,buffer,allocation,0));
 void *mapped; CHECK(vkMapMemory(device,allocation,0,VK_WHOLE_SIZE,0,&mapped));
 memset(mapped,0x5a,65536);
 VkImage image=VK_NULL_HANDLE;VkDeviceMemory image_memory=VK_NULL_HANDLE;
 if(argc>1){
  VkImageCreateInfo image_info={.sType=VK_STRUCTURE_TYPE_IMAGE_CREATE_INFO,.imageType=VK_IMAGE_TYPE_2D,
   .format=VK_FORMAT_R8G8B8A8_UNORM,.extent={128,128,1},.mipLevels=1,.arrayLayers=1,
   .samples=VK_SAMPLE_COUNT_1_BIT,.tiling=VK_IMAGE_TILING_OPTIMAL,
   .usage=VK_IMAGE_USAGE_TRANSFER_SRC_BIT|VK_IMAGE_USAGE_TRANSFER_DST_BIT|VK_IMAGE_USAGE_SAMPLED_BIT,
   .sharingMode=VK_SHARING_MODE_EXCLUSIVE,.initialLayout=VK_IMAGE_LAYOUT_UNDEFINED};
  CHECK(vkCreateImage(device,&image_info,NULL,&image));
  VkMemoryRequirements image_req;vkGetImageMemoryRequirements(device,image,&image_req);
  VkDeviceImageMemoryRequirements imr={.sType=VK_STRUCTURE_TYPE_DEVICE_IMAGE_MEMORY_REQUIREMENTS,.pCreateInfo=&image_info};
  VkMemoryRequirements2 imr2={.sType=VK_STRUCTURE_TYPE_MEMORY_REQUIREMENTS_2};
  vkGetDeviceImageMemoryRequirements(device,&imr,&imr2);
  printf("image type masks: image=%x device-query=%x\n",image_req.memoryTypeBits,imr2.memoryRequirements.memoryTypeBits);fflush(stdout);
  if(image_req.memoryTypeBits!=imr2.memoryRequirements.memoryTypeBits || !(image_req.memoryTypeBits&(1u<<type)))return 1;
  uint32_t image_type=type;
  if(argc>2){
   image_type=0;
   while(image_type<memory.memoryTypeCount && (!(image_req.memoryTypeBits&(1u<<image_type)) ||
    !(memory.memoryTypes[image_type].propertyFlags&VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT)))image_type++;
   if(image_type==memory.memoryTypeCount){puts("FAIL: device-local image memory lost");return 1;}
  }
  printf("image memory type: %u\n",image_type);fflush(stdout);
  VkMemoryDedicatedAllocateInfo dedicated={.sType=VK_STRUCTURE_TYPE_MEMORY_DEDICATED_ALLOCATE_INFO,.image=image};
  VkMemoryAllocateInfo image_alloc={.sType=VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO,.pNext=&dedicated,.allocationSize=image_req.size,.memoryTypeIndex=image_type};
  CHECK(vkAllocateMemory(device,&image_alloc,NULL,&image_memory));
  if(argc>3){
   VkBindImageMemoryInfo bind={.sType=VK_STRUCTURE_TYPE_BIND_IMAGE_MEMORY_INFO,.image=image,.memory=image_memory};
   CHECK(vkBindImageMemory2(device,1,&bind));
  }else CHECK(vkBindImageMemory(device,image,image_memory,0));
 }
 VkCommandPoolCreateInfo pci={.sType=VK_STRUCTURE_TYPE_COMMAND_POOL_CREATE_INFO,.queueFamilyIndex=family};
 VkCommandPool pool;CHECK(vkCreateCommandPool(device,&pci,NULL,&pool));
 VkCommandBufferAllocateInfo cai={.sType=VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO,.commandPool=pool,.level=VK_COMMAND_BUFFER_LEVEL_PRIMARY,.commandBufferCount=1};
 VkCommandBuffer cmd;CHECK(vkAllocateCommandBuffers(device,&cai,&cmd));
 VkCommandBufferBeginInfo begin={.sType=VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO,.flags=VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT};
 CHECK(vkBeginCommandBuffer(cmd,&begin));
 uint32_t expected=0x1234abcd;
 if(!image)vkCmdFillBuffer(cmd,buffer,0,65536,expected);
 else{
  VkImageSubresourceRange range={VK_IMAGE_ASPECT_COLOR_BIT,0,1,0,1};
  VkImageMemoryBarrier ib={.sType=VK_STRUCTURE_TYPE_IMAGE_MEMORY_BARRIER,.dstAccessMask=VK_ACCESS_TRANSFER_WRITE_BIT,
   .oldLayout=VK_IMAGE_LAYOUT_UNDEFINED,.newLayout=VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL,
   .srcQueueFamilyIndex=VK_QUEUE_FAMILY_IGNORED,.dstQueueFamilyIndex=VK_QUEUE_FAMILY_IGNORED,.image=image,.subresourceRange=range};
  vkCmdPipelineBarrier(cmd,VK_PIPELINE_STAGE_TOP_OF_PIPE_BIT,VK_PIPELINE_STAGE_TRANSFER_BIT,0,0,NULL,0,NULL,1,&ib);
  VkClearColorValue color={.float32={1,0,1,1}};
  vkCmdClearColorImage(cmd,image,VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL,&color,1,&range);
  ib.srcAccessMask=VK_ACCESS_TRANSFER_WRITE_BIT;ib.dstAccessMask=VK_ACCESS_TRANSFER_READ_BIT;
  ib.oldLayout=VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL;ib.newLayout=VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL;
  vkCmdPipelineBarrier(cmd,VK_PIPELINE_STAGE_TRANSFER_BIT,VK_PIPELINE_STAGE_TRANSFER_BIT,0,0,NULL,0,NULL,1,&ib);
  VkBufferImageCopy copy={.imageSubresource={VK_IMAGE_ASPECT_COLOR_BIT,0,0,1},.imageExtent={128,128,1}};
  vkCmdCopyImageToBuffer(cmd,image,VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL,buffer,1,&copy);
  expected=0xffff00ff;
 }
 VkMemoryBarrier barrier={.sType=VK_STRUCTURE_TYPE_MEMORY_BARRIER,.srcAccessMask=VK_ACCESS_TRANSFER_WRITE_BIT,.dstAccessMask=VK_ACCESS_HOST_READ_BIT};
 vkCmdPipelineBarrier(cmd,VK_PIPELINE_STAGE_TRANSFER_BIT,VK_PIPELINE_STAGE_HOST_BIT,0,1,&barrier,0,NULL,0,NULL);
 CHECK(vkEndCommandBuffer(cmd));
 VkFenceCreateInfo fci={.sType=VK_STRUCTURE_TYPE_FENCE_CREATE_INFO};
 VkFence fence;CHECK(vkCreateFence(device,&fci,NULL,&fence));
 VkQueue queue;vkGetDeviceQueue(device,family,0,&queue);
 VkSubmitInfo submit={.sType=VK_STRUCTURE_TYPE_SUBMIT_INFO,.commandBufferCount=1,.pCommandBuffers=&cmd};
 CHECK(vkQueueSubmit(queue,1,&submit,fence));
 CHECK(vkWaitForFences(device,1,&fence,VK_TRUE,5000000000ull));
 for(unsigned i=0;i<16384;i++)if(((uint32_t*)mapped)[i]!=expected){printf("FAIL at word %u\n",i);return 1;}
 vkUnmapMemory(device,allocation);
 vkDestroyFence(device,fence,NULL);vkDestroyCommandPool(device,pool,NULL);
 vkDestroyBuffer(device,buffer,NULL);vkFreeMemory(device,allocation,NULL);
 if(image){vkDestroyImage(device,image,NULL);vkFreeMemory(device,image_memory,NULL);}
 vkDestroyDevice(device,NULL);vkDestroyInstance(instance,NULL);
 puts("PASS: 65536 bytes written by host GPU and read in guest; matching requirements; teardown complete");
 return 0;
}
