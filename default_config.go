package main

const defaultConfigYAML = `listen: 127.0.0.1:4141

providers:
  anthropic:
    base_url: https://api.anthropic.com
  cerebras:
    base_url: https://api.cerebras.ai/v1
  cloudflare-ai-gateway:
    base_url: https://gateway.ai.cloudflare.com/v1/{ACCOUNT_ID}/{GATEWAY_ID}
  deepinfra:
    base_url: https://api.deepinfra.com/v1/openai
  deepseek:
    base_url: https://api.deepseek.com/v1
  fireworks:
    base_url: https://api.fireworks.ai/inference/v1
  google:
    base_url: https://generativelanguage.googleapis.com/v1beta
  github-copilot:
    base_url: https://api.githubcopilot.com
  groq:
    base_url: https://api.groq.com/openai/v1
  helicone:
    base_url: https://ai-gateway.helicone.ai
  huggingface:
    base_url: https://api-inference.huggingface.co/v1
  minimax:
    base_url: https://api.minimax.chat/v1
  moonshot:
    base_url: https://api.moonshot.cn/v1
  openai:
    base_url: https://api.openai.com/v1
  opencode:
    base_url: https://opencode.ai/zen/v1
  openrouter:
    base_url: https://openrouter.ai/api/v1
  together:
    base_url: https://api.together.xyz/v1
  vercel:
    base_url: https://api.ai.vercel.com/v1
  volcengine:
    base_url: https://ark.cn-beijing.volces.com/api/coding/v3
  xai:
    base_url: https://api.x.ai/v1
`
