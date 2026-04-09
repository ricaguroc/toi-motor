#!/usr/bin/env bash
set -euo pipefail

OLLAMA_URL="${OLLAMA_URL:-http://localhost:11434}"

echo "Waiting for Ollama..."
until curl -sf "$OLLAMA_URL/api/tags" > /dev/null 2>&1; do
  sleep 2
done
echo "Ollama is ready."

echo "Pulling nomic-embed-text..."
curl -sf "$OLLAMA_URL/api/pull" -d '{"name":"nomic-embed-text"}' | tail -1
echo ""
echo "Pulling llama3.1:8b..."
curl -sf "$OLLAMA_URL/api/pull" -d '{"name":"llama3.1:8b"}' | tail -1
echo ""
echo "Models ready."
