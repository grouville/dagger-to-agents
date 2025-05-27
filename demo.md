# Demo 1

1. [Follow quickstart](https://docs.dagger.io/quickstart/ci) OR `cd hello-dagger`
2. `dagger_dev`
3. shell mode demo
__Aim__ => show the capabilities of the shell mode with useful example

> dagger install github.com/grouville/daggerverse/trivy
---

## Prompts (to be pasted, as is)
1. publish the hello dagger app
1(BIS). publish it
2. check for its vulnerabilities
3. summarize the result and give me action items
---

### 4.  MCP Mode Demo

> **Working setup at the moment**

```yaml
# Engine version:
# https://github.com/grouville/dagger/commit/04f50b6e5899b5b25adaceaae12f9a5bac31ef18

# Current demo commit:
# https://github.com/grouville/dagger-to-agents/commit/f357683433f6df01de20f0ff0157cf0759c068bd

# Permission file: ~/.config/goose/permission.yaml
user:
  always_allow:
    - dagger__HelloDagger_buildEnv
    - dagger__HelloDagger_publish
    - dagger__Trivy_scanImage
    - dagger__Trivy_scanContainer
    - dagger__Trivy_base
  ask_before: []
  never_allow: []

# Config file: ~/.config/goose/config.yaml
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
```

goose version: v0.18.2

Use the exact same prompt as above