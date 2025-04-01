package translator

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"Markdown-translator-go/config" // 根据你的实际项目路径调整
)

// Translator 接口定义了所有 LLM 翻译提供商必须实现的方法。
// 这是策略模式 (Strategy Pattern) 的核心。
type Translator interface {
	// Translate 方法接收原始 Markdown 内容，并返回翻译后的 Markdown 字符串。
	// 每个实现负责处理各自的 Prompt 格式化、API 调用、错误处理以及从响应中提取最终结果。
	Translate(ctx context.Context, markdownContent string) (string, error)
}

// Closer 接口定义了一个可关闭的资源
type Closer interface {
    Close() error
}


// --- 工厂函数 (Factory Function) ---

// NewTranslator 函数充当一个工厂，根据配置信息创建并返回合适的 Translator 实例。
// 这是工厂模式 (Factory Pattern) 的应用。
func NewTranslator(cfg *config.Config) (Translator, error) {
	// 创建一个共享的 HTTP 客户端实例，可以根据需要进行更复杂的配置 (例如重试逻辑)
	httpClient := &http.Client{
		Timeout: 120 * time.Second, // 为 LLM API 调用设置较长的超时时间 (例如 120 秒)
	}

	// 根据配置中的 LLMProvider 决定创建哪个具体的 Translator 实现
	switch cfg.LLMProvider {
	case "openai":
		// 创建 OpenAI 客户端实例
		// 需要 API Key, Endpoint (可选), Model (可选), HTTP Client, Prompt 模板
		return NewOpenAIClient(httpClient, cfg.LLMAPIKey, cfg.LLMAPIEndpoint, cfg.LLMModel, cfg.PromptTemplate)
	case "claude":
		// 创建 Claude 客户端实例
		// 需要 API Key, Endpoint (可选), Model (可选), HTTP Client, Prompt 模板
		// 注意: Claude 可能需要特定的 HTTP Header (如 'anthropic-version')
		return NewClaudeClient(httpClient, cfg.LLMAPIKey, cfg.LLMAPIEndpoint, cfg.LLMModel, cfg.PromptTemplate)
	case "gemini":
		// 创建 Gemini 客户端实例
		// 需要 API Key, Endpoint (可能包含模型名称), Model (用于构建 URL), HTTP Client, Prompt 模板
		return NewGeminiClient(httpClient, cfg.LLMAPIKey, cfg.LLMAPIEndpoint, cfg.LLMModel, cfg.PromptTemplate)
	default:
		// 这个分支理论上不应该被触及，因为配置加载时已经校验过 Provider
		// 但作为代码健壮性的保证，还是加上错误处理
		return nil, fmt.Errorf("内部错误: 不支持的 LLM 提供商 '%s' 传入工厂函数", cfg.LLMProvider)
	}
}
