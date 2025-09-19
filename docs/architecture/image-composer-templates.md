# Understanding and Using Templates in OS Image Composer

Templates in the OS Image Composer tool are YAML files that deliver a straightforward way to customize, standardize, and reuse image configurations. This document explains the template system and how to use it to streamline your image-creation workflow.

## Contents

- [What Are Templates and How Do They Work?](#what-are-templates-and-how-do-they-work)
    - [Template Structure](#template-structure)
    - [Variable Substitution](#variable-substitution)
- [Managing Templates with the Command-Line Interface](#managing-templates-with-the-command-line-interface)
    - [Listing Templates](#listing-templates)
    - [Viewing Template Details](#viewing-template-details)
    - [Creating Templates](#creating-templates)
    - [Exporting Templates](#exporting-templates)
    - [Importing Templates](#importing-templates)
- [Using Templates to Build Images](#using-templates-to-build-images)
    - [Basic Usage](#basic-usage)
    - [Variable Definition Files](#variable-definition-files)
    - [Generating Spec Files from Templates](#generating-spec-files-from-templates)
- [Template Storage](#template-storage)
- [Template Variables](#template-variables)
- [Template Examples](#template-examples)
    - [Web Server Template](#web-server-template)
    - [Database Server Template](#database-server-template)
- [Best Practices](#best-practices)
    - [Template Organization](#template-organization)
    - [Template Design](#template-design)
    - [Template Sharing](#template-sharing)
- [Default Templates](#default-templates)
- [Conclusion](#conclusion)
- [Related Documentation](#related-documentation)


## What Are Templates and How Do They Work?

Templates are predefined build specifications that serve as a foundation for building operating system images. Here's what templates empower you to do:

- Create standardized baseline configurations.
- Impose consistency across multiple images.
- Reduce duplication of effort.
- Share and reuse common configurations with your team.

Templates take the form of YAML files with a structure similar to regular build
specifications, but with added variable placeholders that can be customized when
used. To learn about patterns that work well as templates, see [Common Build Patterns](./image-composer-build-process.md#common-build-patterns). 

### Template Structure

A template includes standard build specification sections with variables where
customization is needed:

```yaml
# Example template: ubuntu-server.yml
template:
  name: ubuntu-server
  description: Standard Ubuntu server image

image:
  name: ${hostname}-server
  base:
    os: ubuntu
    version: ${ubuntu_version}
    type: server

build:
  cache:
    use_package_cache: true
    use_image_cache: true
  stages:
    - base
    - packages
    - configuration
    - finalize

customizations:
  packages:
    install:
      - openssh-server
      - ca-certificates
      - curl
    remove:
      - snapd
  services:
    enabled:
      - ssh
  
  # Other standard configurations ...
```

### Variable Substitution

Templates support simple variable substitution using the `${variable_name}`
syntax. When building an image from a template, you can provide values for these
variables. See the [Build Specification File](./image-composer-cli-specification.md#build-specification-file) in the [command-line reference](./image-composer-cli-specification.md) for the complete structure of build specifications.

## Managing Templates with the Command-Line Interface

The OS Image Composer tool enables you to manage templates with straightforward commands. 

### Listing Templates

```bash
# List all available templates
image-composer template list
```

### Viewing Template Details

```bash
# Show template details including available variables
image-composer template show ubuntu-server
```

### Creating Templates

```bash
# Create a new template from an existing spec file
image-composer template create --name my-server-template my-image-spec.yml
```

### Exporting Templates

```bash
# Export a template to a file for sharing
image-composer template export ubuntu-server ./ubuntu-server-template.yml
```

### Importing Templates

```bash
# Import a template from a file
image-composer template import ./ubuntu-server-template.yml
```

For a list of all the template-management commands, see  [Template Command](./image-composer-cli-specification.md#template-command) in the [command-line reference](./image-composer-cli-specification.md).


## Using Templates to Build Images

The OS `image-composer build` command creates custom operating system images from an image template file. With templates, you can customize OS images to fulfill your requirements. You can also define variables in a separate YAML file and override variables when you run a command. With the `image-composer template render` command, you generate a specification file to review or modify it before building it.

### Basic Usage

```bash
# Build an image using a template
image-composer build --template ubuntu-server my-output-image.yml

# Build with variable overrides
image-composer build --template ubuntu-server --set "ubuntu_version=22.04" --set
"hostname=web-01" my-output-image.yml
```

### Variable Definition Files

You can define variables for templates in separate YAML files:

```yaml
# variables.yml
ubuntu_version: "22.04"
hostname: "production-web-01"
enable_firewall: true
```

And then you can use a YAML file containing your variables with the `image-composer build` command:

```bash
image-composer build --template ubuntu-server --variables variables.yml
my-output-image.yml
```

### Generating Spec Files from Templates

You can generate a specification file from a template to review or modify before
building:

```bash
# Generate spec file from template
image-composer template render ubuntu-server --output my-spec.yml

# Generate with variable overrides
image-composer template render ubuntu-server --set "ubuntu_version=22.04"
--output my-spec.yml
```

See the [Build Command](./image-composer-cli-specification.md#build-command) in the command-line reference. 


## Template Storage

Templates in the OS Image Composer tool are stored in two main locations:

1. System Templates: `/etc/image-composer/templates/`
2. User Templates: `~/.config/image-composer/templates/`

## Template Variables

Templates support three simple variable types:

1. Strings: Text values (e.g., hostnames, versions)
2. Numbers: Numeric values (e.g., port numbers, sizes)
3. Booleans: True/false values (e.g., feature flags)

Here are some example variable definitions:

```yaml
variables:
  hostname:
    default: "ubuntu-server"
    description: "System hostname"
  
  ubuntu_version:
    default: "22.04"
    description: "Ubuntu version to use"
  
  enable_firewall:
    default: true
    description: "Whether to enable the firewall"
```

To find out how variables affect each build stage, see [Build Stages in Detail](./image-composer-build-process.md#build-stages-in-detail).

## Template Examples

Here are examples of templates. 

### Web Server Template

```yaml
template:
  name: web-server
  description: Basic web server image

variables:
  hostname:
    default: "web-server"
    description: "Server hostname"
  
  ubuntu_version:
    default: "22.04"
    description: "Ubuntu version to use"
  
  http_port:
    default: 80
    description: "HTTP port for web server"

image:
  name: ${hostname}
  base:
    os: ubuntu
    version: ${ubuntu_version}
    type: server

customizations:
  packages:
    install:
      - nginx
      - apache2-utils
  services:
    enabled:
      - nginx
  files:
    - source: ./files/nginx.conf
      destination: /etc/nginx/nginx.conf
      permissions: "0644"
```

### Database Server Template

```yaml
template:
  name: db-server
  description: Basic database server image

variables:
  hostname:
    default: "db-server"
    description: "Server hostname"
  
  ubuntu_version:
    default: "22.04"
    description: "Ubuntu version to use"

image:
  name: ${hostname}
  base:
    os: ubuntu
    version: ${ubuntu_version}
    type: server

customizations:
  packages:
    install:
      - postgresql
      - postgresql-client
  services:
    enabled:
      - postgresql
```

For details on customizations that you can apply, see the [Configuration Stage](./image-composer-build-process.md#4-configuration-stage) of the build process.

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

To understand the role templates play in improving the efficiency of builds, see [Build Performance Optimization](./image-composer-build-process.md#build-performance-optimization).

## Default Templates

The OS Image Composer GitHub repository includes several [default image template YAML files](https://github.com/open-edge-platform/image-composer/blob/main/config/osv/) that serve as the basis for creating images with Edge Microvisor Toolkit, Azure Linux, and Wind River eLxr.

## Conclusion

With templates in the OS Image Composer tool, you can standardize the creation of images and reduce repetitive work. By defining common configurations once and
reusing them with different variables, you can:

1. **Save time**: Avoid recreating similar configurations.
2. **Ensure consistency**: Maintain standardized environments.
3. **Simplify onboarding**: Make it easier for new team members to create proper
images.

The template system is designed to be simple yet effective, focusing on
practical reuse rather than complex inheritance or versioning schemes.

## Related Documentation

- [Understanding the Build Process](./image-composer-build-process.md)
- [Multiple Package Repository Support](./image-composer-multi-repo-support.md)
- [OS Image Composer CLI Reference](./image-composer-cli-specification.md)
