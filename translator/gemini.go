package translator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io" // 导入 io 包
	"log"
	"net/http"
	"strings"
	"text/template"
)

const (
	// Gemini API (v1beta) 的端点格式，需要填充模型名称
	defaultGeminiEndpointFormat = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent"
	// 默认使用的 Gemini 模型 (请根据可用性和需求选择)
	defaultGeminiModel = "gemini-1.5-flash-latest" // 或 gemini-pro
)

// GeminiClient 结构体实现了 Translator 接口，用于与 Google Gemini API 交互。
type GeminiClient struct {
	httpClient  *http.Client
	apiKey      string
	apiEndpoint string // 存储最终构建好的 API 端点 URL
	promptTmpl  *template.Template
}

// NewGeminiClient 创建一个新的 Gemini 客户端实例。
func NewGeminiClient(client *http.Client, apiKey, apiEndpoint, model string, promptTmpl *template.Template) (*GeminiClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Gemini API 密钥不能为空")
	}
	if model == "" {
		model = defaultGeminiModel
	}

	// 如果用户没有提供完整的 API Endpoint，则根据格式和模型构建默认的
	if apiEndpoint == "" {
		apiEndpoint = fmt.Sprintf(defaultGeminiEndpointFormat, model)
	} else {
		// 如果用户提供了完整的 URL (可能用于指向特定版本或区域)，则直接使用
		log.Printf("Gemini: 使用用户提供的完整 API 端点: %s\n", apiEndpoint)
	}

	log.Printf("初始化 Gemini 客户端: Endpoint=%s\n", apiEndpoint)
	return &GeminiClient{
		httpClient:  client,
		apiKey:      apiKey,
		apiEndpoint: apiEndpoint, // 保存最终使用的 URL
		promptTmpl:  promptTmpl,
	}, nil
}

// --- Gemini API 特有的请求和响应结构体 (基于 v1beta) ---
type geminiRequest struct {
	Contents         []geminiContent        `json:"contents"`                   // 主要内容
	GenerationConfig *geminiGenerationConfig `json:"generationConfig,omitempty"` // 生成参数配置
	SafetySettings   []geminiSafetySetting   `json:"safetySettings,omitempty"`   // 安全设置
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`            // 内容块列表 (通常只有一个 text part)
	Role  string       `json:"role,omitempty"` // 角色: "user" 或 "model"
}

type geminiPart struct {
	Text string `json:"text"` // 文本内容
}

type geminiGenerationConfig struct {
	Temperature     float32 `json:"temperature,omitempty"`
	TopK            int     `json:"topK,omitempty"`
	TopP            float32 `json:"topP,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"` // 限制输出长度
	// StopSequences   []string `json:"stopSequences,omitempty"`
}

type geminiSafetySetting struct {
	Category  string `json:"category"`  // 如 "HARM_CATEGORY_SEXUALLY_EXPLICIT"
	Threshold string `json:"threshold"` // 如 "BLOCK_MEDIUM_AND_ABOVE"
}

// Gemini API 的响应结构
type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
			Role  string       `json:"role"`
		} `json:"content"`
		FinishReason string `json:"finishReason"` // 完成原因: "STOP", "MAX_TOKENS", "SAFETY", etc.
		// SafetyRatings []...
	} `json:"candidates"`
	// PromptFeedback 可能包含因安全设置等原因被阻止的信息
	PromptFeedback *struct {
		BlockReason string `json:"blockReason"` // 如 "SAFETY"
		// SafetyRatings []...
	} `json:"promptFeedback,omitempty"`
	// Gemini 可能在顶层返回错误，例如认证失败
    Error *struct {
        Code    int    `json:"code"`    // HTTP status code mapped
        Message string `json:"message"` // Error message
        Status  string `json:"status"`  // e.g., "UNAUTHENTICATED"
	} `json:"error,omitempty"`
}

// Translate 方法实现了 Translator 接口，用于 Gemini。
// !!! 重要: 此实现基于 Gemini API v1beta 文档，务必进行实际测试和调整 !!!
func (c *GeminiClient) Translate(ctx context.Context, markdownContent string) (string, error) {
	// 步骤 1: 渲染 Prompt
	var promptBuf bytes.Buffer
	templateData := map[string]string{"Content": markdownContent}
	if err := c.promptTmpl.Execute(&promptBuf, templateData); err != nil {
		return "", fmt.Errorf("Gemini: 执行 Prompt 模板失败: %w", err)
	}
	finalPrompt := promptBuf.String()

	// 步骤 2: 构建 Gemini API 请求体
	apiRequest := geminiRequest{
		Contents: []geminiContent{
			{
				// 对于简单的单轮请求，可以省略 'user' 角色
				Parts: []geminiPart{
					{Text: finalPrompt},
				},
			},
		},
		// GenerationConfig: &geminiGenerationConfig{ // 按需配置生成参数
		// 	MaxOutputTokens: 8192,
		// 	Temperature: 0.7,
		// },
		// SafetySettings: []geminiSafetySetting{ // 按需配置安全设置
		// 	{Category: "HARM_CATEGORY_HARASSMENT", Threshold: "BLOCK_NONE"},
		// 	{Category: "HARM_CATEGORY_HATE_SPEECH", Threshold: "BLOCK_NONE"},
		// 	{Category: "HARM_CATEGORY_SEXUALLY_EXPLICIT", Threshold: "BLOCK_NONE"},
		// 	{Category: "HARM_CATEGORY_DANGEROUS_CONTENT", Threshold: "BLOCK_NONE"},
		// },
	}

	reqBodyBytes, err := json.Marshal(apiRequest)
	if err != nil {
		return "", fmt.Errorf("Gemini: 序列化 API 请求失败: %w", err)
	}

	// 步骤 3: 创建并发送 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiEndpoint, bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return "", fmt.Errorf("Gemini: 创建 API 请求失败: %w", err)
	}

	// 设置 Gemini 特有的请求参数 (API Key 通常作为 URL Query 参数)
	q := req.URL.Query()
	q.Add("key", c.apiKey)
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	log.Printf("Gemini: 发送请求到 %s\n", c.apiEndpoint) // API Key 在 URL 中，不直接打印
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Gemini: API 请求执行失败: %w", err)
	}
	defer resp.Body.Close()

	// 步骤 4: 读取并解码响应体
	respBodyBytes, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", fmt.Errorf("Gemini: 读取 API 响应体失败 (状态码 %d): %w", resp.StatusCode, err)
    }

	// 步骤 5: 处理响应状态码和内容
	var apiResponse geminiResponse
	if err := json.Unmarshal(respBodyBytes, &apiResponse); err != nil {
		preview := string(respBodyBytes)
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		return "", fmt.Errorf("Gemini: 解码 API 响应失败 (状态码 %d): %w. 响应体预览: %s", resp.StatusCode, err, preview)
	}

	// 检查顶层错误 (例如认证、权限问题)
    if apiResponse.Error != nil {
        return "", fmt.Errorf("Gemini: API 返回顶层错误: %s (Code: %d, Status: %s)", apiResponse.Error.Message, apiResponse.Error.Code, apiResponse.Error.Status)
    }

	// 检查 HTTP 状态码 (应该在检查顶层错误之后)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        errMsg := fmt.Sprintf("Gemini: API 返回非成功状态码 %d", resp.StatusCode)
		preview := string(respBodyBytes)
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		errMsg = fmt.Sprintf("%s. 响应体预览: %s", errMsg, preview)
		return "", fmt.Errorf(errMsg)
	}


	// 检查 Prompt 是否因安全等原因被阻止
	if apiResponse.PromptFeedback != nil && apiResponse.PromptFeedback.BlockReason != "" {
		return "", fmt.Errorf("Gemini: 请求被阻止，原因: %s", apiResponse.PromptFeedback.BlockReason)
	}

	// 检查是否有候选结果以及完成原因是否正常
	if len(apiResponse.Candidates) == 0 {
		// 即使没有错误，也可能没有候选结果 (例如，prompt 被过滤但未报告 blockReason)
		log.Printf("Gemini: API 响应不包含候选结果。PromptFeedback: %+v\n", apiResponse.PromptFeedback)
		return "", fmt.Errorf("Gemini: API 响应未包含候选结果")
	}

	// 检查第一个候选者的完成原因
	finishReason := apiResponse.Candidates[0].FinishReason
	if finishReason != "STOP" && finishReason != "MAX_TOKENS" {
		// 其他原因如 "SAFETY", "RECITATION", "OTHER" 都表示有问题
		return "", fmt.Errorf("Gemini: 生成因 '%s' 原因停止", finishReason)
	}

	// 步骤 6: 提取翻译结果 (通常在第一个候选者的第一个 Part 中)
	if len(apiResponse.Candidates[0].Content.Parts) == 0 || apiResponse.Candidates[0].Content.Parts[0].Text == "" {
		log.Printf("Gemini: API 响应的候选结果中不包含有效文本内容。FinishReason: %s\n", finishReason)
		return "", fmt.Errorf("Gemini: API 响应未包含有效翻译内容 (FinishReason: %s)", finishReason)
	}

	// 拼接所有 Parts (虽然通常只有一个)
	var builder strings.Builder
	for _, part := range apiResponse.Candidates[0].Content.Parts {
		builder.WriteString(part.Text)
	}
	translatedText := builder.String()

	log.Printf("Gemini: 成功接收并解析响应。\n")

	return translatedText, nil
}
