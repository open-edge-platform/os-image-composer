# Introduction

Templates in Image-Composer provide a straightforward way to standardize and reuse image configurations. This document explains the template system and how to use it to streamline your image creation workflow.

## What Are Templates?

Templates are pre-defined image specifications that serve as a foundation for building OS images. They allow you to:

- Create standardized baseline configurations
- Ensure consistency across multiple images
- Reduce duplication of effort
- Share common configurations with your team

## How Templates Work

Templates are simply YAML files with a structure similar to regular image specifications, but with added variable placeholders that can be customized when used.

### Template Structure

A template includes standard image specification sections with variables where customization is needed:

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
    - platform
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
  
  # Other standard configuration...
```

### Variable Substitution

Templates support simple variable substitution using the `${variable_name}` syntax. When building an image from a template, you can provide values for these variables.

## Managing Templates via CLI

Image-Composer provides straightforward commands to manage templates:

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

## Using Templates to Build Images

### Basic Usage

```bash
# Build an image using a template
image-composer build --template ubuntu-server my-output-image.yml

# Build with variable overrides
image-composer build --template ubuntu-server --set "ubuntu_version=22.04" --set "hostname=web-01" my-output-image.yml
```

### Variable Definition Files

You can define variables for templates in separate files:

```yaml
# variables.yml
ubuntu_version: "22.04"
hostname: "production-web-01"
enable_firewall: true
```

Then use them with:

```bash
image-composer build --template ubuntu-server --variables variables.yml my-output-image.yml
```

### Generating Spec Files from Templates

You can generate a specification file from a template to review or modify before building:

```bash
# Generate spec file from template
image-composer template render ubuntu-server --output my-spec.yml

# Generate with variable overrides
image-composer template render ubuntu-server --set "ubuntu_version=22.04" --output my-spec.yml
```

## Template Storage

Templates in Image-Composer are stored in two main locations:

1. **System Templates**: `/etc/image-composer/templates/`
2. **User Templates**: `~/.config/image-composer/templates/`

## Template Variables

Templates support three simple variable types:

1. **Strings**: Text values (e.g., hostnames, versions)
2. **Numbers**: Numeric values (e.g., port numbers, sizes)
3. **Booleans**: True/false values (e.g., feature flags)

Example variable definitions:

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

## Template Examples

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

## Best Practices

### Template Organization

1. **Keep Templates Simple**: Focus on common configurations that are likely to be reused
2. **Use Descriptive Names**: Name templates according to their purpose
3. **Document Variables**: Provide clear descriptions for all variables

### Template Design

1. **Parameterize Wisely**: Make variables out of settings that are likely to change
2. **Provide Defaults**: Always include sensible default values for variables
3. **Minimize Complexity**: Keep templates straightforward and focused

### Template Sharing

1. **Version Control**: Store templates in a Git repository
2. **Documentation**: Maintain a simple catalog of available templates
3. **Standardization**: Use templates to enforce organizational standards

## Conclusion

Templates in Image-Composer provide a straightforward way to standardize image creation and reduce repetitive work. By defining common configurations once and reusing them with different variables, you can:

1. **Save Time**: Avoid recreating similar configurations
2. **Ensure Consistency**: Maintain standardized environments
3. **Simplify Onboarding**: Make it easier for new team members to create proper images

The template system is designed to be simple yet effective, focusing on practical reuse rather than complex inheritance or versioning schemes.
