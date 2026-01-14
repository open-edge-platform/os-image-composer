#!/bin/bash
# /opt/software/wait-for-network.sh

set -e

TARGET="ollama.com"
MAX_ATTEMPTS=60  # 60 attempts * 5 seconds = 5 minutes total
COUNT=0

echo "Starting Network Waiter for $TARGET..."

while ! curl -s --head --request GET "https://$TARGET" | grep "200 OK" > /dev/null; do
    echo "Network unreachable. Waiting for connection... (Attempt $COUNT/$MAX_ATTEMPTS)"
    
    # Check if we've exceeded the timeout
    if [ $COUNT -ge $MAX_ATTEMPTS ]; then
        echo "Error: Network timeout reached. Proceeding without network (installation may fail)."
        exit 1
    fi

    sleep 5
    ((COUNT++))
done

echo "✓ Network connection detected. Proceeding..."
exit 0