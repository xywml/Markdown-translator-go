package translator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io" // 导入 io 包
	"log"
	"net/http"
	"text/template"
)

const (
	// Claude API (Messages API) 的默认端点
	defaultClaudeEndpoint = "https://api.anthropic.com/v1/messages"
	// 默认使用的 Claude 模型 (请根据需要更新为最新或合适的模型)
	defaultClaudeModel = "claude-3-sonnet-20240229"
	// Claude API 要求指定的版本 Header
	claudeAPIVersion = "2023-06-01"
)

// ClaudeClient 结构体实现了 Translator 接口，用于与 Anthropic Claude API 交互。
type ClaudeClient struct {
	httpClient  *http.Client
	apiKey      string
	apiEndpoint string
	model       string
	promptTmpl  *template.Template
}

// NewClaudeClient 创建一个新的 Claude 客户端实例。
func NewClaudeClient(client *http.Client, apiKey, apiEndpoint, model string, promptTmpl *template.Template) (*ClaudeClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Claude API 密钥不能为空")
	}
	if apiEndpoint == "" {
		apiEndpoint = defaultClaudeEndpoint
	}
	if model == "" {
		model = defaultClaudeModel
	}
	log.Printf("初始化 Claude 客户端: Endpoint=%s, Model=%s, APIVersion=%s\n", apiEndpoint, model, claudeAPIVersion)
	return &ClaudeClient{
		httpClient:  client,
		apiKey:      apiKey,
		apiEndpoint: apiEndpoint,
		model:       model,
		promptTmpl:  promptTmpl,
	}, nil
}

// --- Claude API 特有的请求和响应结构体 (Messages API) ---
type claudeRequest struct {
	Model     string           `json:"model"`              // 模型名称
	Messages  []claudeMessage  `json:"messages"`           // 对话消息列表
	System    string           `json:"system,omitempty"`   // Claude 使用独立的 system prompt 字段
	MaxTokens int              `json:"max_tokens"`           // Claude API 要求此字段
	Temperature float64        `json:"temperature,omitempty"` // 可选参数
}

type claudeMessage struct {
	Role    string `json:"role"`    // 角色，通常是 "user"
	Content string `json:"content"` // 消息内容
}

// Claude 的响应结构与 OpenAI 不同
type claudeContentBlock struct {
	Type string `json:"type"` // 预期是 "text"
	Text string `json:"text"` // 翻译内容在这里
}

type claudeErrorDetail struct { // 基于 Claude 文档可能出现的错误结构
    Type    string `json:"type"`
    Message string `json:"message"`
}

type claudeResponse struct {
	Content      []claudeContentBlock `json:"content"`       // 模型生成的内容块列表
	StopReason   string               `json:"stop_reason"`   // 完成原因，如 "end_turn", "max_tokens"
	Usage        map[string]int       `json:"usage"`         // token 使用情况
    Error        *claudeErrorDetail   `json:"error,omitempty"` // Claude 的错误结构
}

// Translate 方法实现了 Translator 接口，用于 Claude。
// !!! 重要: 此实现基于 Claude Messages API 文档，务必进行实际测试和调整 !!!
func (c *ClaudeClient) Translate(ctx context.Context, markdownContent string) (string, error) {
	// 步骤 1: 渲染 Prompt
	// Claude 的 Messages API 可以接受独立的 System Prompt。
	// 为了简化，我们暂时将所有内容放入 User Message，但最佳实践可能是
	// 从模板中解析出系统级指令和用户内容。
	var promptBuf bytes.Buffer
	templateData := map[string]string{"Content": markdownContent}
	if err := c.promptTmpl.Execute(&promptBuf, templateData); err != nil {
		return "", fmt.Errorf("Claude: 执行 Prompt 模板失败: %w", err)
	}
	fullUserPrompt := promptBuf.String()
	// systemPrompt := "You are a translation assistant..." // 理想情况下从模板提取

	// 步骤 2: 构建 Claude API 请求体
	apiRequest := claudeRequest{
		Model: c.model,
		// System: systemPrompt, // 如果提取了 System Prompt，在这里设置
		Messages: []claudeMessage{
			{Role: "user", Content: fullUserPrompt},
		},
		MaxTokens: 4000, // 必须设置 MaxTokens，根据需要调整值
		// Temperature: 0.7,
	}

	reqBodyBytes, err := json.Marshal(apiRequest)
	if err != nil {
		return "", fmt.Errorf("Claude: 序列化 API 请求失败: %w", err)
	}

	// 步骤 3: 创建并发送 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiEndpoint, bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return "", fmt.Errorf("Claude: 创建 API 请求失败: %w", err)
	}

	// 设置 Claude 特有的 HTTP Headers
	req.Header.Set("x-api-key", c.apiKey)                // API Key Header
	req.Header.Set("anthropic-version", claudeAPIVersion) // API 版本 Header
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "application/json")

	log.Printf("Claude: 发送请求到 %s (模型: %s)\n", c.apiEndpoint, c.model)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Claude: API 请求执行失败: %w", err)
	}
	defer resp.Body.Close()

    // 步骤 4: 读取并解码响应体
	respBodyBytes, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", fmt.Errorf("Claude: 读取 API 响应体失败 (状态码 %d): %w", resp.StatusCode, err)
    }

	// 步骤 5: 处理响应状态码和内容
	var apiResponse claudeResponse
	if err := json.Unmarshal(respBodyBytes, &apiResponse); err != nil {
		preview := string(respBodyBytes)
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		return "", fmt.Errorf("Claude: 解码 API 响应失败 (状态码 %d): %w. 响应体预览: %s", resp.StatusCode, err, preview)
	}

    // 检查响应体中的错误信息 (根据 Claude 文档调整结构)
    if apiResponse.Error != nil {
        return "", fmt.Errorf("Claude: API 返回错误: %s (类型: %s)", apiResponse.Error.Message, apiResponse.Error.Type)
    }

	// 检查 HTTP 状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        errMsg := fmt.Sprintf("Claude: API 返回非成功状态码 %d", resp.StatusCode)
        if apiResponse.Error != nil {
             errMsg = fmt.Sprintf("%s - %s", errMsg, apiResponse.Error.Message)
		} else {
			 preview := string(respBodyBytes)
			 if len(preview) > 500 {
				 preview = preview[:500] + "..."
			 }
			 errMsg = fmt.Sprintf("%s. 响应体预览: %s", errMsg, preview)
		}
		return "", fmt.Errorf(errMsg)
	}

	// 步骤 6: 提取翻译结果
	// Claude 的响应内容是一个列表，通常第一个是 text 类型
	if len(apiResponse.Content) == 0 || apiResponse.Content[0].Type != "text" || apiResponse.Content[0].Text == "" {
		log.Printf("Claude: API 响应不包含有效文本内容。停止原因: %s\n", apiResponse.StopReason)
		return "", fmt.Errorf("Claude: API 响应未包含有效翻译内容 (停止原因: %s)", apiResponse.StopReason)
	}

	translatedText := apiResponse.Content[0].Text
	log.Printf("Claude: 成功接收并解析响应。\n")

	return translatedText, nil
}
