package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/BurntSushi/toml" // 导入 TOML 解析库
)

// SupportedProviders 列出了当前支持的 LLM 提供商标识符。
var SupportedProviders = []string{"openai", "claude", "gemini"}

// TomlConfig 结构体对应 TOML 配置文件结构
type TomlConfig struct {
	API struct {
		Provider string `toml:"provider"`
		Endpoint string `toml:"endpoint"`
		Key      string `toml:"key"`
		Model    string `toml:"model"`
	} `toml:"api"`
	General struct {
		SourceDir   string `toml:"source_dir"`
		TargetDir   string `toml:"target_dir"`
		Concurrency int    `toml:"concurrency"`
		PromptFile  string `toml:"prompt_file"`
		Overwrite   bool   `toml:"overwrite"`
	} `toml:"general"`
}

// Config 结构体保存所有应用程序的配置项。
type Config struct {
	SourceDir      string             // 源目录: 包含待翻译的英文 Markdown 文件。
	TargetDir      string             // 目标目录: 用于存放翻译后的 Markdown 文件。
	Concurrency    int                // 并发数: 同时运行的翻译 Worker (Goroutine) 数量。
	LLMProvider    string             // LLM提供商: 指定使用哪个 LLM 服务 (例如 "openai", "claude", "gemini")。
	LLMAPIEndpoint string             // LLM API 端点: 对应提供商的 API URL (对于某些提供商可能是基础URL)。
	LLMAPIKey      string             // LLM API 密钥: 通过环境变量 MK_TRANSLATOR_API_KEY 获取。
	LLMModel       string             // LLM 模型: 指定使用的具体模型名称 (可选, 取决于提供商默认值)。
	PromptFile     string             // Prompt 文件路径: 自定义 Prompt 模板文件的路径。
	PromptTemplate *template.Template // Prompt 模板: 已解析的 Prompt 模板对象。
	Overwrite      bool               // 覆盖模式: 是否覆盖目标目录中已存在的同名文件。
	DryRun         bool               // 空跑模式: 若为 true, 则不实际调用 API 或写入文件, 仅日志记录。
	ConfigFile     string             // TOML 配置文件路径
}

// LoadConfig 函数解析命令行标志和环境变量来填充 Config 结构体, 并进行校验。
func LoadConfig() (*Config, error) {
	cfg := &Config{}

	// 定义命令行参数及其描述 (中文)
	flag.StringVar(&cfg.SourceDir, "source", "pages", "源目录 (包含英文 md 文件)")
	flag.StringVar(&cfg.TargetDir, "target", "pages.zh", "目标目录 (用于输出翻译文件)")
	flag.IntVar(&cfg.Concurrency, "concurrency", 5, "并发 Worker 数量")
	flag.StringVar(&cfg.LLMProvider, "provider", "openai", fmt.Sprintf("使用的 LLM 提供商 (%s)", strings.Join(SupportedProviders, ", ")))
	flag.StringVar(&cfg.LLMAPIEndpoint, "api-url", "", "LLM API 端点 URL (对于某些提供商可能是基础 URL)")
	flag.StringVar(&cfg.LLMModel, "model", "", "使用的 LLM 模型名称 (可选, 取决于提供商默认值)")
	flag.StringVar(&cfg.PromptFile, "prompt-file", "prompt.template", "LLM Prompt 模板文件路径")
	flag.BoolVar(&cfg.Overwrite, "overwrite", false, "覆盖已存在的目标文件")
	flag.BoolVar(&cfg.DryRun, "dry-run", false, "空跑模式 (不调用 API, 不写入文件)")
	flag.StringVar(&cfg.ConfigFile, "config", "", "TOML 配置文件路径 (优先级高于环境变量)")

	// 从环境变量读取 API Key (更安全)
	apiKeyEnv := "MK_TRANSLATOR_API_KEY"
	cfg.LLMAPIKey = os.Getenv(apiKeyEnv)

	flag.Parse() // 解析注册的命令行参数

	// 如果指定了配置文件，从配置文件加载设置
	if cfg.ConfigFile != "" {
		if err := loadTomlConfig(cfg); err != nil {
			return nil, fmt.Errorf("加载配置文件失败: %w", err)
		}
	}

	cfg.PromptFile = filepath.Clean(cfg.PromptFile)

	// --- 配置项校验 ---
	cfg.LLMProvider = strings.ToLower(cfg.LLMProvider) // 统一转为小写
	isValidProvider := false
	for _, p := range SupportedProviders {
		if cfg.LLMProvider == p {
			isValidProvider = true
			break
		}
	}
	if !isValidProvider {
		return nil, fmt.Errorf("不支持的 LLM 提供商 '%s'. 支持的提供商: %s", cfg.LLMProvider, strings.Join(SupportedProviders, ", "))
	}

	// 在非空跑模式下, API Key 是必需的
	if cfg.LLMAPIKey == "" && !cfg.DryRun {
		return nil, fmt.Errorf("必须设置 API Key (通过环境变量 %s 或配置文件) (除非使用 --dry-run)", apiKeyEnv)
	}
	if cfg.Concurrency <= 0 {
		return nil, fmt.Errorf("并发数 (--concurrency) 必须大于 0")
	}
	// 检查源目录是否存在
	if _, err := os.Stat(cfg.SourceDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("源目录 '%s' 不存在", cfg.SourceDir)
	}

	// 加载并解析 Prompt 模板文件
	promptTemplateContent := getDefaultPromptTemplate() // 获取默认模板内容
	promptBytes, err := os.ReadFile(cfg.PromptFile)
	if err == nil {
		// 如果成功读取文件, 使用文件内容
		promptTemplateContent = string(promptBytes)
		fmt.Printf("成功加载 Prompt 文件: %s\n", cfg.PromptFile)
	} else if !os.IsNotExist(err) {
		// 如果发生错误但不是 "文件未找到", 则报告警告
		fmt.Printf("警告: 读取 Prompt 文件 '%s' 时出错: %v. 将使用默认 Prompt。\n", cfg.PromptFile, err)
	} else {
		// 如果文件不存在 (os.IsNotExist), 使用默认模板是预期行为
		fmt.Printf("警告: Prompt 文件 '%s' 未找到。将使用默认 Prompt。\n", cfg.PromptFile)
	}

	// 仅解析一次 Prompt 模板
	tmpl, err := template.New("prompt").Parse(promptTemplateContent)
	if err != nil {
		return nil, fmt.Errorf("解析 Prompt 模板失败: %w", err)
	}
	cfg.PromptTemplate = tmpl // 保存已解析的模板对象

	// 在非空跑模式下, 确保目标目录存在
	if !cfg.DryRun {
		if err := os.MkdirAll(cfg.TargetDir, 0755); err != nil {
			return nil, fmt.Errorf("创建目标目录 '%s' 失败: %w", cfg.TargetDir, err)
		}
		fmt.Printf("已确保目标目录 '%s' 存在。\n", cfg.TargetDir)
	}

	return cfg, nil
}

// loadTomlConfig 从 TOML 文件加载配置
func loadTomlConfig(cfg *Config) error {
	var tomlCfg TomlConfig

	// 解析 TOML 文件
	if _, err := toml.DecodeFile(cfg.ConfigFile, &tomlCfg); err != nil {
		return fmt.Errorf("解析 TOML 文件错误: %w", err)
	}

	fmt.Printf("已从配置文件 '%s' 加载设置\n", cfg.ConfigFile)

	// 应用 TOML 配置 (命令行参数会覆盖这些设置)
	// API 设置
	if tomlCfg.API.Provider != "" {
		cfg.LLMProvider = tomlCfg.API.Provider
		fmt.Printf("从配置文件设置提供商: %s\n", cfg.LLMProvider)
	}
	if tomlCfg.API.Endpoint != "" {
		cfg.LLMAPIEndpoint = tomlCfg.API.Endpoint
		fmt.Printf("从配置文件设置 API 端点\n")
	}
	if tomlCfg.API.Key != "" {
		cfg.LLMAPIKey = tomlCfg.API.Key
		fmt.Println("从配置文件加载 API 密钥")
	}
	if tomlCfg.API.Model != "" {
		cfg.LLMModel = tomlCfg.API.Model
		fmt.Printf("从配置文件设置模型: %s\n", cfg.LLMModel)
	}

	// 常规设置
	if tomlCfg.General.SourceDir != "" {
		cfg.SourceDir = tomlCfg.General.SourceDir
		fmt.Printf("从配置文件设置源目录: %s\n", cfg.SourceDir)
	}
	if tomlCfg.General.TargetDir != "" {
		cfg.TargetDir = tomlCfg.General.TargetDir
		fmt.Printf("从配置文件设置目标目录: %s\n", cfg.TargetDir)
	}
	if tomlCfg.General.Concurrency > 0 {
		cfg.Concurrency = tomlCfg.General.Concurrency
		fmt.Printf("从配置文件设置并发数: %d\n", cfg.Concurrency)
	}
	if tomlCfg.General.PromptFile != "" {
		cfg.PromptFile = tomlCfg.General.PromptFile
		fmt.Printf("从配置文件设置 Prompt 文件: %s\n", cfg.PromptFile)
	}

	// 覆盖模式需要特殊处理，因为它是布尔值
	// 只有当配置文件中明确指定时才应用
	cfg.Overwrite = tomlCfg.General.Overwrite
	if tomlCfg.General.Overwrite {
		fmt.Println("从配置文件启用覆盖模式")
	}

	return nil
}

// getDefaultPromptTemplate 返回默认的 Prompt 模板字符串。
// 这个模板是给 LLM 的指令，保持英文可能更通用。
func getDefaultPromptTemplate() string {
	return `You are a translation assistant specialized in command-line tool documentation (like tldr pages).
Translate the following Markdown content from English to Simplified Chinese.

**Crucial Instructions:**
1.  Preserve the original Markdown formatting EXACTLY (code blocks with backticks ` + "``" + `, {{placeholders}}, links, headers, lists, etc.).
2.  Ensure technical terms are translated accurately and consistently in the context of command-line usage.
3.  ONLY output the translated Markdown content. Do NOT include any other explanatory text before or after.
4.  Wrap your ENTIRE translated Markdown output within <translate> tags. Example: <translate># translated content...</translate>

Original English Markdown:
---
{{.Content}}
---

Translated Chinese Markdown (within <translate> tags):`
}
