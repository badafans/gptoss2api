# Cloudflare Workers AI OpenAI Compatible API

这是一个兼容 OpenAI API 格式的代理服务器，用于访问 Cloudflare Workers AI 服务。它允许您使用标准的 OpenAI API 调用方式与 Cloudflare 提供的 AI 模型进行交互。

## 功能

- **OpenAI API 兼容**: 实现了 `/v1/chat/completions` 和 `/v1/models` 接口，与 OpenAI API 格式兼容
- **Cloudflare Workers AI 集成**: 将 OpenAI 格式的请求转换为 Cloudflare Workers AI API 请求
- **流式响应支持**: 支持 OpenAI 的流式响应格式 (text/event-stream)
- **模型配置**: 可通过命令行参数配置 Cloudflare Account ID、模型名称、认证令牌等
- **客户端认证**: 支持可选的客户端密钥认证
- **响应格式转换**: 自动将 Cloudflare 响应转换为 OpenAI 格式
- **日志记录**: 记录用户请求和 Cloudflare 原始响应，便于调试

## 使用方法

```bash
go run openai.go -id=<account_id> -model=<model_name> -token=<auth_token> -port=<port> -key=<client_key>
```

## 接口

- `POST /v1/chat/completions` - 聊天完成接口
- `GET /v1/models` - 获取模型列表

## 许可证

MIT License

Copyright (c) 2025

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
