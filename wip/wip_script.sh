#!/bin/bash

set -euo pipefail

echo "🚧 Starting MCP demo environment setup..."

##############################################
## 1. Clone and launch custom Dagger engine ##
##############################################

if [ ! -d "./dagger" ]; then
  echo "🔄 Cloning custom Dagger engine..."
  git clone https://github.com/grouville/dagger.git
  cd dagger
  git checkout rebased-for-mcp
  echo "▶️ Launching ./hack/dev..."
  ./hack/dev &
  cd ..
else
  echo "✅ Dagger repo already exists, skipping clone"
fi

##############################################
## 2. Clone dagger-to-agents at demo commit ##
##############################################

if [ ! -d "./dagger-to-agents" ]; then
  echo "🔄 Cloning dagger-to-agents demo repo..."
  git clone https://github.com/grouville/dagger-to-agents.git
  cd dagger-to-agents
  git checkout f357683433f6df01de20f0ff0157cf0759c068bd
else
  echo "✅ dagger-to-agents repo already exists, skipping clone"
  cd dagger-to-agents
fi

echo "📁 Entering hello-dagger"
cd hello-dagger

##################################################
## 3. Reset state: stop processes and clear cache
##################################################

echo "🧼 Cleaning up previous state..."

pkill goose || true
pkill mcp || true

docker volume rm dagger-engine.dev || true

#################################
## 4. Setup Goose configuration ##
#################################

GOOSE_CONFIG_DIR="${HOME}/.config/goose"
mkdir -p "$GOOSE_CONFIG_DIR"

echo "📄 Writing permission.yaml..."
cat > "${GOOSE_CONFIG_DIR}/permission.yaml" <<'EOF'
user:
  always_allow:
    - dagger__HelloDagger_buildEnv
    - dagger__HelloDagger_publish
    - dagger__Trivy_scanImage
    - dagger__Trivy_scanContainer
    - dagger__Trivy_base
  ask_before: []
  never_allow: []
EOF

echo "📄 Writing config.yaml..."
cat > "${GOOSE_CONFIG_DIR}/config.yaml" <<'EOF'
GOOSE_PROVIDER: openai
OPENAI_HOST: https://api.openai.com
extensions:
  dagger:
    args: []
    cmd: /tmp/mcp.sh
    description: null
    enabled: true
    envs: {}
    name: dagger
    timeout: 300
    type: stdio
GOOSE_CLI_MIN_PRIORITY: 0.0
OPENAI_BASE_PATH: v1/chat/completions
GOOSE_MODEL: gpt-4.1
GOOSE_MODE: auto
EOF

#################################
## 5. Final message and prompts ##
#################################

echo ""
echo "🎉 Environment is ready!"
echo ""
echo "👇 Suggested prompt sequence:"
echo ""
echo "  1. publish the hello dagger app"
echo "  2. check for its vulnerabilities"
echo "  3. summarize the result and give me action items"
echo ""
echo "🧠 Tip: Paste these one at a time to see MCP in action."