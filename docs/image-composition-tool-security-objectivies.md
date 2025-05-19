# Image Composer Tool Security Objectives Specification

## Introduction

Image Composer Tool security objectives are to allow tool user to build minimal, verifiable, and secure operating system images, focusing on reducing the attack surface and simplifying the adoption of modern boot mechanisms. This includes encrypting partitions, using dm-verity for root protection, and supporting Secure Boot. 

The following are the details of those objectives:

 ### 1. Reduced Attack Surface:

Image Composer Tool allows customization, enabling the inclusion of only necessary kernel features and executables, reducing the overall attack surface as defined by the user. 

### 2. Secure Image Generation:
Image Composer Tool optionally supports the following aspects to generate secure images
  * cryptographic signing of the boot chain (kernel, initial RAM disk, kernel command line) for Secure Boot, ensuring integrity and authenticity. 
  * protect the root filesystem with dm-verity, making offline attacks more difficult. 
  * generate Software Bill of Materials (SBOM)s for images, providing transparency into their components. 
 
### 3. Support of Modern Boot Mechanisms:
Image Composer Tool simplifies the adoption of Unified Kernel Image (UKI), a modern boot mechanism, potentially improving edge node security by supporting secure boot.

### 4. Partition Customization:
Image Composer Tool allows customization of partition layout, including the ability to optionally encrypt root partitions, customize partition sizes, add partitions for security. 

