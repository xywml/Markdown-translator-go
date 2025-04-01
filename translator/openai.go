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
	// OpenAI API 的默认端点 URL
	defaultOpenAIEndpoint = "https://api.openai.com/v1/chat/completions"
	// 默认使用的 OpenAI 模型
	defaultOpenAIModel = "gpt-3.5-turbo" // 或者选择更新的默认模型，如 gpt-4o-mini
)

// OpenAIClient 结构体实现了 Translator 接口，用于与 OpenAI API 进行交互。
type OpenAIClient struct {
	httpClient  *http.Client       // 共享的 HTTP 客户端
	apiKey      string             // OpenAI API 密钥
	apiEndpoint string             // 使用的 API 端点 URL
	model       string             // 使用的模型名称
	promptTmpl  *template.Template // 已解析的 Prompt 模板
}

// NewOpenAIClient 创建一个新的 OpenAI 客户端实例。
func NewOpenAIClient(client *http.Client, apiKey, apiEndpoint, model string, promptTmpl *template.Template) (*OpenAIClient, error) {
	// 校验必需的 API Key
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API 密钥不能为空")
	}
	// 如果用户未提供 Endpoint, 使用默认值
	if apiEndpoint == "" {
		apiEndpoint = defaultOpenAIEndpoint
	}
	// 如果用户未提供 Model, 使用默认值
	if model == "" {
		model = defaultOpenAIModel
	}
	log.Printf("初始化 OpenAI 客户端: Endpoint=%s, Model=%s\n", apiEndpoint, model)
	return &OpenAIClient{
		httpClient:  client,
		apiKey:      apiKey,
		apiEndpoint: apiEndpoint,
		model:       model,
		promptTmpl:  promptTmpl,
	}, nil
}

// --- OpenAI API 特有的请求和响应结构体 ---
type openAIRequest struct {
	Model       string          `json:"model"`                 // 模型名称
	Messages    []openAIMessage `json:"messages"`              // 对话消息列表
	Temperature float64         `json:"temperature,omitempty"` // 可选参数：控制创造性，0 表示更确定性
	MaxTokens   int             `json:"max_tokens,omitempty"`  // 可选参数：限制生成内容的最大长度
}

type openAIMessage struct {
	Role    string `json:"role"`    // 角色: "system", "user", "assistant"
	Content string `json:"content"` // 消息内容
}

type openAIErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   any    `json:"param"` // 可能为 null 或 string
	Code    any    `json:"code"`  // 可能为 null 或 string
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"` // LLM 生成的内容
		} `json:"message"`
		FinishReason string `json:"finish_reason"` // 完成原因，如 "stop", "length"
	} `json:"choices"`
	Error *openAIErrorDetail `json:"error,omitempty"` // API 返回的错误信息结构
}

// Translate 方法实现了 Translator 接口，用于 OpenAI。
func (c *OpenAIClient) Translate(ctx context.Context, markdownContent string) (string, error) {
	// 步骤 1: 使用模板渲染最终的 Prompt
	var promptBuf bytes.Buffer
	templateData := map[string]string{"Content": markdownContent}
	if err := c.promptTmpl.Execute(&promptBuf, templateData); err != nil {
		return "", fmt.Errorf("OpenAI: 执行 Prompt 模板失败: %w", err)
	}
	finalPrompt := promptBuf.String()

	// 步骤 2: 构建 OpenAI API 请求体
	apiRequest := openAIRequest{
		Model: c.model,
		Messages: []openAIMessage{
			// OpenAI 通常将用户的主要输入放在 "user" 角色的消息中
			{Role: "user", Content: finalPrompt},
			// 如果需要，可以在这里添加 "system" 角色的消息
			// {Role: "system", Content: "You are translating markdown pages."},
		},
		// Temperature: 0.7, // 如果需要，在这里设置其他参数
	}

	reqBodyBytes, err := json.Marshal(apiRequest)
	if err != nil {
		return "", fmt.Errorf("OpenAI: 序列化 API 请求失败: %w", err)
	}

	// 步骤 3: 创建并发送 HTTP POST 请求
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiEndpoint, bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return "", fmt.Errorf("OpenAI: 创建 API 请求失败: %w", err)
	}

	// 设置必要的 HTTP Headers (Authorization 使用 Bearer Token)
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	log.Printf("OpenAI: 发送请求到 %s (模型: %s)\n", c.apiEndpoint, c.model)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		// 处理网络层面的错误 (如超时、连接失败)
		return "", fmt.Errorf("OpenAI: API 请求执行失败: %w", err)
	}
	defer resp.Body.Close()

	// 步骤 4: 读取并解码 API 响应体
	respBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("OpenAI: 读取 API 响应体失败 (状态码 %d): %w", resp.StatusCode, err)
	}

	// 步骤 5: 处理响应状态码和内容
	var apiResponse openAIResponse
	// 尝试解码 JSON，即使状态码可能是错误的，因为错误信息也可能在 JSON 体中
	if err := json.Unmarshal(respBodyBytes, &apiResponse); err != nil {
		// 如果解码失败，返回原始状态码和部分响应体以供调试
		preview := string(respBodyBytes)
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		return "", fmt.Errorf("OpenAI: 解码 API 响应失败 (状态码 %d): %w. 响应体预览: %s", resp.StatusCode, err, preview)
	}

	// 检查响应体中是否包含 API 级别的错误信息
	if apiResponse.Error != nil {
		return "", fmt.Errorf("OpenAI: API 返回错误: %s (类型: %s, Code: %v)", apiResponse.Error.Message, apiResponse.Error.Type, apiResponse.Error.Code)
	}

	// 检查 HTTP 状态码是否表示成功 (2xx)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// 如果有错误结构体但之前未返回，这里可以再次尝试使用它提供信息
		errMsg := fmt.Sprintf("OpenAI: API 返回非成功状态码 %d", resp.StatusCode)
		if apiResponse.Error != nil { // 确认错误结构体已填充
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
	if len(apiResponse.Choices) == 0 || apiResponse.Choices[0].Message.Content == "" {
		// 可能是因为内容过滤或其他原因导致没有有效输出
		finishReason := "未知"
		if len(apiResponse.Choices) > 0 {
			finishReason = apiResponse.Choices[0].FinishReason
		}
		log.Printf("OpenAI: API 响应不包含有效内容。完成原因: %s\n", finishReason)
		return "", fmt.Errorf("OpenAI: API 响应未包含有效翻译内容 (完成原因: %s)", finishReason)
	}

	translatedText := apiResponse.Choices[0].Message.Content
	log.Printf("OpenAI: 成功接收并解析响应。\n")

	// 注意: 从这里返回的是 LLM 的原始输出。
	// <translate> 标签的提取将在调用此函数之后 (在 processor/worker.go 中) 进行。
	return translatedText, nil
}
