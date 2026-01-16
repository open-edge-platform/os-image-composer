#!/bin/bash
# REMOVED set -e to allow the loop to continue even if curl fails

TARGET="ollama.com"
MAX_ATTEMPTS=600
COUNT=0

echo "Starting Network Waiter for $TARGET..."

while [ $COUNT -lt $MAX_ATTEMPTS ]; do
    # Check if we can reach the target. 
    # We use '|| true' to prevent the script from exiting on a network error.
    HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 5 "https://$TARGET" || echo "000")

    if [ "$HTTP_STATUS" == "200" ]; then
        echo "✓ Network connection detected. Proceeding..."
        exit 0
    fi

    echo "Network unreachable (Status: $HTTP_STATUS). Waiting... (Attempt $COUNT/$MAX_ATTEMPTS)"
    
    sleep 10
    ((COUNT++))
done

echo "Error: Network timeout reached."
exit 1