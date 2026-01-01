#!/bin/bash
set -e

export PUBSUB_EMULATOR_HOST=localhost:8085
PROJECT_ID=${PROJECT_ID:-test-project}
TOPIC_ID=${TOPIC_ID:-buildkite-events}

echo "Creating topic: $TOPIC_ID"
curl -X PUT "http://localhost:8085/v1/projects/$PROJECT_ID/topics/$TOPIC_ID"

echo ""
echo "Creating test subscription"
curl -X PUT "http://localhost:8085/v1/projects/$PROJECT_ID/subscriptions/test-sub" \
  -H "Content-Type: application/json" \
  -d "{
    \"topic\": \"projects/$PROJECT_ID/topics/$TOPIC_ID\"
  }"

echo ""
echo "✅ Emulator initialized!"
