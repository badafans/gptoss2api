package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type Config struct {
	AccountID string
	Model     string
	AuthToken string
	Port      string
	ClientKey string
}

type OpenAIRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Stream      bool      `json:"stream,omitempty"`
	Temperature *float64  `json:"temperature,omitempty"`
	TopP        *float64  `json:"top_p,omitempty"`
}

type Message struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type OpenAIResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type CloudflareRequest struct {
	Model       string      `json:"model"`
	Input       interface{} `json:"input"`
	Temperature *float64    `json:"temperature,omitempty"`
	TopP        *float64    `json:"top_p,omitempty"`
}

type CloudflareResponse struct {
	ID      string                 `json:"id"`
	Created int64                  `json:"created_at"`
	Model   string                 `json:"model"`
	Object  string                 `json:"object"`
	Output  []CloudflareOutputItem `json:"output"`
	Usage   CloudflareUsage        `json:"usage"`
}

type CloudflareOutputItem struct {
	ID      string                  `json:"id"`
	Content []CloudflareContentItem `json:"content"`
	Role    string                  `json:"role,omitempty"`
	Type    string                  `json:"type"`
	Status  string                  `json:"status,omitempty"`
}

type CloudflareContentItem struct {
	Text string `json:"text"`
	Type string `json:"type"`
}

type CloudflareUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

var config Config

func main() {
	flag.StringVar(&config.AccountID, "id", "", "Cloudflare Account ID")
	flag.StringVar(&config.Model, "model", "@cf/openai/gpt-oss-120b", "Cloudflare Model")
	flag.StringVar(&config.AuthToken, "token", "", "Cloudflare Auth Token")
	flag.StringVar(&config.Port, "port", "10000", "Server Port")
	flag.StringVar(&config.ClientKey, "key", "", "Client Authorization Key")
	flag.Parse()

	if config.AuthToken == "" {
		log.Fatal("请提供 auth-token 参数")
	}

	http.HandleFunc("/v1/chat/completions", handleChatCompletions)
	http.HandleFunc("/v1/models", handleModels)

	fmt.Printf("服务器启动在端口 %s\n", config.Port)
	log.Fatal(http.ListenAndServe(":"+config.Port, nil))
}

func authorizeClient(r *http.Request) bool {
	if config.ClientKey == "" {
		return true
	}
	return r.Header.Get("Authorization") == "Bearer "+config.ClientKey
}

func handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if !authorizeClient(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, _ := io.ReadAll(r.Body)
	log.Printf("用户请求 JSON: %s", string(body))

	var openaiReq OpenAIRequest
	if err := json.Unmarshal(body, &openaiReq); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	cfReq := convertToCloudflareRequest(openaiReq)

	// 调用 Cloudflare API（保留原始响应字符串）
	cfResp, rawCFJSON, err := callCloudflareAPI(cfReq, r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("Cloudflare API error: %v", err), http.StatusInternalServerError)
		return
	}

	// 打印 Cloudflare 原始响应（不转义）
	log.Printf("Cloudflare 原始响应: %s", rawCFJSON)

	openaiResp := convertToOpenAIResponse(cfResp)

	if openaiReq.Stream {
		// SSE 流式返回，符合 OpenAI 兼容格式
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		fullContent := openaiResp.Choices[0].Message.Content.(string)
		// 为了防止在多字节 UTF-8 字符中间切断，我们按字符而不是字节分割
		runes := []rune(fullContent)

		// 发送开始标记
		startEvent := map[string]interface{}{
			"id":      openaiResp.ID,
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   openaiResp.Model,
			"choices": []map[string]interface{}{
				{
					"delta": map[string]interface{}{
						"role": "assistant",
					},
					"index":         0,
					"finish_reason": nil,
				},
			},
		}
		w.Write([]byte("data: "))
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		enc.Encode(startEvent)
		w.Write([]byte("\n"))
		w.(http.Flusher).Flush()

		// 逐字符发送内容
		for _, r := range runes {
			event := map[string]interface{}{
				"id":      openaiResp.ID,
				"object":  "chat.completion.chunk",
				"created": time.Now().Unix(),
				"model":   openaiResp.Model,
				"choices": []map[string]interface{}{
					{
						"delta": map[string]interface{}{
							"content": string(r),
						},
						"index":         0,
						"finish_reason": nil,
					},
				},
			}
			w.Write([]byte("data: "))
			enc := json.NewEncoder(w)
			enc.SetEscapeHTML(false)
			enc.Encode(event)
			w.Write([]byte("\n"))
			w.(http.Flusher).Flush()
		}

		// 发送结束标记，包含 usage 信息
		endEvent := map[string]interface{}{
			"id":      openaiResp.ID,
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   openaiResp.Model,
			"choices": []map[string]interface{}{
				{
					"delta":         map[string]interface{}{},
					"index":         0,
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     openaiResp.Usage.PromptTokens,
				"completion_tokens": openaiResp.Usage.CompletionTokens,
				"total_tokens":      openaiResp.Usage.TotalTokens,
			},
		}
		w.Write([]byte("data: "))
		enc = json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		enc.Encode(endEvent)
		w.Write([]byte("\n\n"))

		// 发送 [DONE] 标记
		w.Write([]byte("data: [DONE]\n\n"))
		w.(http.Flusher).Flush()
	} else {
		// 普通返回，禁止转义
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		enc.Encode(openaiResp)
	}
}

func handleModels(w http.ResponseWriter, r *http.Request) {
	if !authorizeClient(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	modelsResp := map[string]interface{}{
		"object": "list",
		"data": []map[string]interface{}{
			{
				"id":       config.Model,
				"object":   "model",
				"created":  time.Now().Unix(),
				"owned_by": "openai",
			},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(modelsResp)
}

func convertToCloudflareRequest(openaiReq OpenAIRequest) CloudflareRequest {
	var cfMessages []map[string]interface{}
	for _, msg := range openaiReq.Messages {
		cfMessages = append(cfMessages, map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}

	cfReq := CloudflareRequest{
		Model: config.Model,
		Input: cfMessages,
	}

	if openaiReq.Temperature != nil {
		cfReq.Temperature = openaiReq.Temperature
	}
	if openaiReq.TopP != nil {
		cfReq.TopP = openaiReq.TopP
	}

	return cfReq
}

// 修改：返回 CloudflareResponse 和 原始 JSON 字符串
func callCloudflareAPI(req CloudflareRequest, ctx context.Context) (*CloudflareResponse, string, error) {
	reqBody, _ := json.Marshal(req)
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/ai/v1/responses", config.AccountID)

	httpReq, _ := http.NewRequestWithContext(ctx, "POST", url, io.NopCloser(strings.NewReader(string(reqBody))))
	httpReq.Header.Set("Authorization", "Bearer "+config.AuthToken)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, string(body), fmt.Errorf("API request failed: %s", string(body))
	}

	var cloudflareResp CloudflareResponse
	if err := json.Unmarshal(body, &cloudflareResp); err != nil {
		return nil, string(body), err
	}
	return &cloudflareResp, string(body), nil
}

func convertToOpenAIResponse(cloudflareResp *CloudflareResponse) OpenAIResponse {
	var reasoningText string
	var assistantMessage string

	for _, output := range cloudflareResp.Output {
		if output.Type == "reasoning" {
			for _, content := range output.Content {
				if content.Type == "reasoning_text" {
					reasoningText = content.Text
				}
			}
		}
		if output.Type == "message" && output.Role == "assistant" {
			for _, content := range output.Content {
				if content.Type == "output_text" {
					assistantMessage = content.Text
				}
			}
		}
	}

	finalMessage := ""
	if reasoningText != "" {
		finalMessage += fmt.Sprintf("<think>%s</think>\n", reasoningText)
	}
	finalMessage += assistantMessage

	return OpenAIResponse{
		ID:      cloudflareResp.ID,
		Object:  "chat.completion",
		Created: cloudflareResp.Created,
		Model:   cloudflareResp.Model,
		Choices: []Choice{
			{
				Index: 0,
				Message: Message{
					Role:    "assistant",
					Content: finalMessage,
				},
				FinishReason: "stop",
			},
		},
		Usage: Usage{
			PromptTokens:     cloudflareResp.Usage.PromptTokens,
			CompletionTokens: cloudflareResp.Usage.CompletionTokens,
			TotalTokens:      cloudflareResp.Usage.TotalTokens,
		},
	}
}
