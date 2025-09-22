# Understanding Templates in Image-Composer

Templates in Image-Composer provide a straightforward way to standardize and
reuse image configurations. This document explains the template system and how
to use it to streamline your image creation workflow.

## Related Documentation

- [Understanding the Build Process](./image-composer-build-process.md) - Details
on the five-stage build pipeline
- [Understanding Caching in Image-Composer](./image-composer-caching.md) -
Information about package and image caching systems
- [Image-Composer CLI Specification](./image-composer-cli-specification.md) -
Complete command-line reference

## What Are Templates?

Templates are pre-defined build specifications that serve as a foundation for
building OS images. They allow you to:

- Create standardized baseline configurations
- Ensure consistency across multiple images
- Reduce duplication of effort
- Share common configurations with your team

The OS image composer provides default image templates on a per distribution
basis and image type (RAW vs. ISO) that can be used directly to build an OS
from those defaults. You can override these default templates by providing your
own template and configure or override the specific settings and values you would 
like. The tool will internally merge the two to create the final
template used for image composition.

![image-templates](./assets/template.drawio.svg)

Validation is performed both on the provided user template, along with the
default template for the particular distribution and image type you are building.
It is not recommended to directly modify the default templates.

The generic path pattern to the default OS templates can
be found under

```bash

osv/<distribution>/imageconfig/defaultconfigs/default-<type>-<arch>.yml

```

where <type> indicates the image type your are building (ISO vs. RAW) and
<arch> defines the architecture you are building for.

When building your own custom image, it is not necessary to start an image
template from scratch. The `image-templates` directory contains user-templates
that can be used as starting points for your own custom images.

See also:

- [Common Build Patterns](./image-composer-build-process.md#common-build-patterns)
for patterns that work well as templates

## How Templates Work

Templates are simply YAML files with a structure similar to regular build
specifications, but with added variable placeholders that can be customized when
used.

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

### Variable Substitution

Templates support simple variable substitution using the `${variable_name}`
syntax. When building an image from a template, you can provide values for these
variables.

See also:

- [Build Specification File](./image-composer-cli-specification.md#build-specification-file)
for the complete structure of build specifications

See also:

- [Template Command](./image-composer-cli-specification.md#template-command) for
all template management commands

## Using Templates to Build Images

### Basic Usage

```bash
# Build an image using a template
image-composer build azl3-x86_64-edge-raw.yml

```

See also:

- [Build Command](./image-composer-cli-specification.md#build-command) for all
build options with templates

## Template Storage

Templates in Image-Composer are stored in two main locations:

1. **System Templates**: `/etc/image-composer/templates/`
2. **User Templates**: `~/.config/image-composer/templates/`

## Template Variables

See also:

- [Build Stages in Detail](./image-composer-build-process.md#build-stages-in-detail)
for how variables affect each build stage

See also:

- [Configuration Stage](./image-composer-build-process.md#4-configuration-stage)
for details on customizations that can be applied

## Best Practices

### Template Organization

1. **Keep Templates Simple**: Focus on common configurations that are likely to
be reused
2. **Use Descriptive Names**: Name templates according to their purpose
3. **Document Variables**: Provide clear descriptions for all variables

### Template Design

1. **Parameterize Wisely**: Make variables out of settings that are likely to
change
2. **Provide Defaults**: Always include sensible default values for variables
3. **Minimize Complexity**: Keep templates straightforward and focused

### Template Sharing

1. **Version Control**: Store templates in a Git repository
2. **Documentation**: Maintain a simple catalog of available templates
3. **Standardization**: Use templates to enforce organizational standards

See also:
- [Build Performance Optimization](./image-composer-build-process.md#build-performance-optimization)
for how templates can improve build efficiency

## Conclusion

Templates in OS image composer provide a straightforward way to standardize image
creation and reduce repetitive work. By defining common configurations once and
reusing them with different variables, you can:

1. **Save Time**: Avoid recreating similar configurations
2. **Ensure Consistency**: Maintain standardized environments
3. **Simplify Onboarding**: Make it easier for new team members to create proper
images

The template system is designed to be simple yet effective, focusing on
practical reuse rather than complex inheritance or versioning schemes.
