#!/bin/sh
set -eu

input="${1:-}"

if [ -z "$input" ]; then
    echo "dev"
    exit 0
fi

# Replace disallowed characters with hyphen
# Allowed characters per Debian version policy: [0-9A-Za-z.+~:-]
# See: https://www.debian.org/doc/debian-policy/ch-controlfields.html#version
sanitized=$(printf '%s' "$input" | tr -c '0-9A-Za-z.+~:-' '-')
# Collapse repeated hyphens and trim from ends
sanitized=$(printf '%s' "$sanitized" | sed -E 's/^-+//; s/-+$//; s/-{2,}/-/g')

if [ -z "$sanitized" ]; then
    sanitized="dev"
fi

echo "$sanitized"
