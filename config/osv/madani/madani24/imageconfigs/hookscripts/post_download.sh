#!/bin/bash
# filepath: /data/madani-os-builder/config/osv/madani/madani24/imageconfigs/hookscripts/post_download.sh

# Post-download hook script for package processing
# Expected to output comma-separated list of downloaded packages

# Safety Check: Ensure TARGET_CACHE is defined
if [ -z "$TARGET_CACHE" ]; then
    echo "Error: TARGET_CACHE environment variable is not set." >&2
    echo "Aborting to prevent overwriting host system files." >&2
    exit 1
fi

# Ensure TARGET_CACHE directory exists and is writable
if [ ! -d "$TARGET_CACHE" ] || [ ! -w "$TARGET_CACHE" ]; then
    echo "Error: TARGET_CACHE directory '$TARGET_CACHE' does not exist or is not writable." >&2
    exit 1
fi

# Log execution within the target cache directory
LOG_FILE="$TARGET_CACHE/post_download_executed.txt"
echo "Post-download hook script executed at $(date)" >> "$LOG_FILE"
echo "TARGET_CACHE: $TARGET_CACHE" >> "$LOG_FILE"

# TODO: Replace with actual package processing logic
# For now, returning mock package list
echo "package01-1.0.0,package02-1.0.0"