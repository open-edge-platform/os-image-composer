# Understanding and Using Templates in OS Image Composer

Templates in the OS Image Composer tool are YAML files that deliver a straightforward way to customize, standardize, and reuse image configurations. This document explains the template system and how to use it to streamline your image-creation workflow.

## Contents

- [Understanding and Using Templates in OS Image Composer](#understanding-and-using-templates-in-os-image-composer)
  - [Contents](#contents)
  - [What Are Templates and How Do They Work?](#what-are-templates-and-how-do-they-work)
    - [Template Structure](#template-structure)
    - [Variable Substitution](#variable-substitution)
  - [Using Templates to Build Images](#using-templates-to-build-images)
  - [Template Storage](#template-storage)
  - [Template Variables](#template-variables)
  - [Best Practices](#best-practices)
    - [Template Organization](#template-organization)
    - [Template Design](#template-design)
    - [Template Sharing](#template-sharing)
  - [Conclusion](#conclusion)
  - [Related Documentation](#related-documentation)



## What Are Templates and How Do They Work?

Templates are predefined build specifications that serve as a foundation for building operating system images. Here's what templates empower you to do:

- Create standardized baseline configurations.
- Impose consistency across multiple images.
- Reduce duplication of effort.
- Share and reuse common configurations with your team.

The OS Image Composer provides default image templates on a per-distribution
basis and image type (RAW vs. ISO) that can be used directly to build an operating system
from those defaults. You can override these default templates by providing your
own template and configure or override the settings and values you want. The tool will internally merge the two to create the final
template used for image composition.

![image-templates](./assets/template.drawio.svg)

Validation is performed both on the provided user template and the
default template for the particular distribution and image type you are building.
It is not recommended to directly modify the default templates.

The generic path pattern to the default OS templates is as follows:

```bash

osv/<distribution>/imageconfig/defaultconfigs/default-<type>-<arch>.yml

```

In the pattern, <type> indicates the image type you are building (ISO vs. RAW) and
<arch> defines the architecture you are building for.

When building your own custom image, it is unnecessary to start an image
template from scratch. The `image-templates` directory contains user-templates
that can be used as starting points for your own custom images.

### Template Structure

A template includes standard build specification sections with variables where
customization is needed:

```yaml
image:
  name: emt3-x86_64-edge
  version: "1.0.0"

target:
  os: edge-microvisor-toolkit # Target OS name
  dist: emt3 # Target OS distribution
  arch: x86_64 # Target OS architecture
  imageType: raw # Image type, valid value: [raw, iso].

# System configuration
systemConfig:
  name: edge
  description: Default yml configuration for edge image

  immutability:
    enabled: false # default is true

  # Package Configuration
  packages:
  # Additional packages beyond the base system
    - cloud-init
    - rsyslog

  # Kernel Configuration
  kernel:
    version: "6.12"
    cmdline: "console=ttyS0,115200 console=tty0 loglevel=7"
```

To learn about patterns that work well as templates, see [Common Build Patterns](./os-image-composer-build-process.md#common-build-patterns).

### Variable Substitution

Templates support simple variable substitution using the `${variable_name}`
syntax. When building an image from a template, you can provide values for these variables. See the [Build Specification File](./os-image-composer-cli-specification.md#build-specification-file) in the [command-line reference](./os-image-composer-cli-specification.md) for the complete structure of build specifications.

## Using Templates to Build Images

The OS `os-image-composer build` command creates custom operating system images from an image template file. With templates, you can customize OS images to fulfill your requirements. You can also define variables in a separate YAML file and override variables when you run a command. With the `os-image-composer template render` command, you generate a specification file to review or modify it before building it.

```bash
# Build an image using a template
os-image-composer build azl3-x86_64-edge-raw.yml

```

See the [Build Command](./os-image-composer-cli-specification.md#build-command) in the command-line reference.

## Template Storage

Templates in the OS Image Composer tool are stored in two main locations:

1. System Templates: `/etc/os-image-composer/templates/`
2. User Templates: `~/.config/os-image-composer/templates/`

## Template Variables

To find out how variables affect each build stage, see [Build Stages in Detail](./os-image-composer-build-process.md#build-stages-in-detail).

For details on customizations that you can apply, see the [Configuration Stage](./os-image-composer-build-process.md#4-configuration-stage) of the build process.

## Best Practices

### Template Organization

1. **Keep templates simple**: Focus on common configurations that are likely to
be reused.
2. **Use descriptive names**: Name templates according to their purpose.
3. **Document variables**: Provide clear descriptions for all the variables.

### Template Design

1. **Parameterize wisely**: Make variables out of settings that are likely to
change.
2. **Provide defaults**: Always include sensible default values for variables.
3. **Minimize complexity**: Keep templates straightforward and focused.

### Template Sharing

1. **Version control**: Store templates in a Git repository.
2. **Documentation**: Maintain a simple catalog of your templates.
3. **Standardization**: Use templates to enforce your standards.

To understand the role templates play in improving the efficiency of builds, see [Build Performance Optimization](./os-image-composer-build-process.md#build-performance-optimization).


## Conclusion

With templates in the OS Image Composer tool, you can standardize the creation of images and reduce repetitive work. By defining common configurations once and reusing them with different variables, you can:

1. **Save time**: Avoid recreating similar configurations.
2. **Ensure consistency**: Maintain standardized environments.
3. **Simplify onboarding**: Make it easier for new team members to create proper
images.

The template system is designed to be simple yet effective, focusing on
practical reuse rather than complex inheritance or versioning schemes.

## Related Documentation

- [Understanding the Build Process](./os-image-composer-build-process.md)
- [Multiple Package Repository Support](./os-image-composer-multi-repo-support.md)
- [OS Image Composer CLI Reference](./os-image-composer-cli-specification.md)

<!--hide_directive
:::{toctree}
:hidden:

os-image-composer-multi-repo-support
:::
hide_directive-->
