# Markdown Translator (Go)

[English](#english) / [简体中文](#简体中文)

## English

**Acknowledgement**

A special thanks to @TQmyLady for providing API access that greatly facilitated the development and testing of this tool.

---

This is a command-line tool written in Go designed to efficiently translate Markdown documents using Large Language Model (LLM) APIs.

---

### Features

*   **Multi LLM Support**: Easily switch between different LLM services like OpenAI, Claude, Gemini via configuration.
*   **Concurrent Processing**: Leverages Go's concurrency for fast and efficient processing of numerous Markdown files.
*   **Highly Configurable**: Flexibly configure source/target directories, concurrency level, LLM provider, model, API endpoint, prompt template, etc., via command-line arguments or a config file.
*   **Intelligent Extraction**: Automatically extracts translation content wrapped in `<translate>` tags from the LLM response.
*   **Robustness & Fault Tolerance**: Includes detailed error handling, file existence checks (with optional overwrite), and a Dry Run mode for testing.
*   **Docker Support**: Provides a Dockerfile for simplified deployment and cross-environment execution.
*   **Clear Structure**: The code is organized for readability, maintainability, and ease of extending support for new LLM providers.

---

### Configuration

Configure the tool via command-line arguments:

*   `-source <path>`: Source directory containing Markdown files (Default: `pages`).
*   `-target <path>`: Target directory for translated files (Default: `pages.zh`).
*   `-concurrency <number>`: Number of concurrent translation workers (Default: `5`).
*   `-provider <name>`: **[Important]** Specify the LLM provider (e.g., `openai`, `claude`, `gemini`, Default: `openai`).
*   `-api-url <URL>`: LLM API endpoint URL. Optional for some providers (like OpenAI, uses default), potentially required in specific formats for others (like Gemini). Refer to provider docs and code.
*   `-model <name>`: Specify the LLM model name (e.g., `gpt-4o-mini`, `claude-3-opus-20240229`, `gemini-1.5-pro-latest`). Uses provider's default if omitted.
*   `-prompt-file <path>`: Path to a custom prompt template file (Default: `prompt.template`).
*   `-overwrite`: If set, overwrites existing files in the target directory.
*   `-dry-run`: If set, performs file discovery but does **not** call LLM APIs or write files. Ideal for testing configuration.
*   `-config <path>`: Path to a TOML configuration file (e.g., `config.toml`). Arguments override file settings.

**Environment Variable:**

*   `MK_TRANSLATOR_API_KEY`: **[Required]** Your Large Language Model API key. Ensure it's the correct key for the selected `-provider`.

---

### Local Run

#### Prerequisites

*   Go installed (Version >= 1.18 recommended).
*   `MK_TRANSLATOR_API_KEY` environment variable set.

#### Setup & Build

1.  Clone or download the repository.
2.  Navigate to the project root (`Markdown-translator-go`).
3.  (If no `go.mod` exists) Run: `go mod init Markdown-translator-go`
4.  Download dependencies: `go mod tidy`
5.  Build the executable: `go build -o Markdown-translator-go-app .`

#### Usage Examples

```bash
# --- Using OpenAI (assuming key is set) ---
export MK_TRANSLATOR_API_KEY='sk-YourOpenAIKey...'
./Markdown-translator-go-app \
    --source ./path/to/source/md \
    --target ./output-zh \
    --provider openai \
    --model "gpt-4o-mini" \
    --concurrency 10 \
    --overwrite

# --- Using Claude (assuming key is set) ---
export MK_TRANSLATOR_API_KEY='YourClaudeKey...'
./Markdown-translator-go-app \
    --source ./path/to/source/md \
    --target ./output-zh \
    --provider claude \
    --model "claude-3-haiku-20240307" \
    --concurrency 8

# --- Using Gemini (assuming key is set) ---
export MK_TRANSLATOR_API_KEY='YourGeminiKey...'
./Markdown-translator-go-app \
    --source ./path/to/source/md \
    --target ./output-zh \
    --provider gemini \
    --model "gemini-1.5-flash-latest" \
    --concurrency 12 \
    --dry-run # Example: using dry run mode

# --- Using a config file ---
./Markdown-translator-go-app --config config.toml
```

---

### Docker Run

Use Docker to run the tool without a local Go environment.

#### Prerequisites

*   Docker installed.

#### Build Docker Image

From the project root directory (containing the `Dockerfile`):

```bash
docker build -t markdown-translator-go:latest .
```

#### Run Docker Container

Mount your source directory (read-only) and target directory (writable), passing the API key via environment variable.

```bash
# --- Example: Running with OpenAI via Docker ---
docker run --rm \
    -e MK_TRANSLATOR_API_KEY='sk-YourOpenAIKey...' \
    -v "/path/on/host/to/source/md:/app/source:ro" \
    -v "/path/on/host/to/output/zh:/app/target:rw" \
    markdown-translator-go:latest \
    --provider openai \
    --source /app/source \
    --target /app/target \
    --model "gpt-4o-mini" \
    --concurrency 10

# --- Example: Running with Claude via Docker ---
docker run --rm \
    -e MK_TRANSLATOR_API_KEY='YourClaudeKey...' \
    -v "/path/on/host/to/source/md:/app/source:ro" \
    -v "/path/on/host/to/output/zh:/app/target:rw" \
    markdown-translator-go:latest \
    --provider claude \
    --source /app/source \
    --target /app/target \
    --model "claude-3-sonnet-20240229" \
    --concurrency 8 \
    --overwrite # Example: adding overwrite flag
```

**Note:**
*   Replace `/path/on/host/to/source/md` and `/path/on/host/to/output/zh` with actual paths on your host machine.
*   `:ro` mounts read-only, `:rw` mounts read-write.
*   `--rm` automatically removes the container instance on exit.

---

### Important: Adapting and Testing LLM API Implementations

This project supports multiple LLM providers, with implementations located in the `translator/` directory (e.g., `openai.go`, `claude.go`, `gemini.go`).

**While basic implementations are provided, careful attention and potential adjustments are needed:**

1.  **API Key**: Ensure the correct format and valid API Key is provided for the selected `-provider`.
2.  **Model Name**: Verify the `-model` name is supported by the provider and accessible to you.
3.  **API Endpoint**: Check if `--api-url` is appropriate for your provider and network environment (e.g., proxies).
4.  **Request/Response Structure**: **Implementations like `claude.go` and `gemini.go` are based on public documentation. LLM APIs evolve. Actual request parameters (e.g., `max_tokens` for Claude), response formats, and error structures might differ. Always cross-reference with the latest official API documentation, test thoroughly, and debug, especially the error handling logic.**
5.  **Specific Headers**: Some APIs (like Claude) require specific HTTP headers (e.g., `anthropic-version`). Ensure these are correctly set in the respective implementation.

**Incorrect configuration or mismatch with specific provider API details is the most common cause of translation failures. Test extensively!**

---

### Prompt Template (`prompt.template`)

The program loads `prompt.template` from the same directory as the executable by default. Use `--prompt-file` to specify a different template.

The default prompt aims to instruct the LLM to:
*   Perform English-to-Chinese translation (or adapt as needed).
*   Focus on accuracy for technical documentation, preserving terminology.
*   **Strictly preserve** the original Markdown formatting (code blocks, `{{placeholders}}`, links, etc.).
*   Output **only** the translated Markdown content without extra explanations.
*   **Wrap the entire translation** within `<translate>` and `</translate>` tags for extraction.

Feel free to adjust the prompt template based on the chosen LLM's behavior and desired translation quality.

---

### Contributing

Contributions via Pull Requests or Issues to improve this tool are welcome.

---

### License

MIT License

---

## 简体中文

**鸣谢**

特别感谢 @TQmyLady 提供 API 用于本工具的开发测试工作。

---

这是一个使用 Go 语言编写的命令行工具，旨在利用大语言模型（LLM）API 高效地翻译 Markdown 文档。

---

### 功能特性

*   **多 LLM 支持**: 通过配置轻松切换使用 OpenAI, Claude, Gemini 等不同的 LLM 服务。
*   **并发处理**: 利用 Go 的并发能力，快速、高效地处理大量 Markdown 文件。
*   **高度可配置**: 通过命令行参数或配置文件灵活配置源目录、目标目录、并发数、LLM 提供商、模型、API 端点、Prompt 模板等。
*   **智能提取**: 自动从 LLM 的响应中提取由 `<translate>` 标签包裹的翻译内容。
*   **健壮性与容错**: 包含详细的错误处理、文件存在检查（可配置覆盖）、空跑模式（Dry Run）用于测试。
*   **Docker 支持**: 提供 Dockerfile，简化部署和跨环境运行。
*   **清晰结构**: 代码结构清晰，易于理解、维护和扩展新的 LLM 提供商。

---

### 配置

通过命令行参数配置工具：

*   `-source <路径>`: 包含 Markdown 文件的源目录 (默认为: `pages`)。
*   `-target <路径>`: 输出翻译后文件的目标目录 (默认为: `pages.zh`)。
*   `-concurrency <数量>`: 并发执行翻译任务的 Worker 数量 (默认为: `5`)。
*   `-provider <名称>`: **[重要]** 指定使用的 LLM 提供商 (例如: `openai`, `claude`, `gemini`, 默认为 `openai`)。
*   `-api-url <URL>`: LLM API 端点 URL。对于某些提供商 (如 OpenAI) 是可选的（使用默认值），对于其他提供商 (如 Gemini) 可能需要特定格式。请参考提供商文档和代码实现。
*   `-model <名称>`: 指定要使用的具体 LLM 模型名称 (例如: `gpt-4o-mini`, `claude-3-opus-20240229`, `gemini-1.5-pro-latest`)。如果省略，会使用提供商的默认模型。
*   `-prompt-file <路径>`: 指定自定义 Prompt 模板文件的路径 (默认为: `prompt.template`)。
*   `-overwrite`: 如果设置此标志，将会覆盖目标目录中已存在的同名文件。
*   `-dry-run`: 如果设置此标志，将执行查找文件等操作，但**不会**实际调用 LLM API，也**不会**写入任何文件。非常适合用于测试配置。
*   `-config <路径>`: 指定 TOML 配置文件的路径（例如 `config.toml`）。命令行参数会覆盖文件中的设置。

**环境变量:**

*   `MK_TRANSLATOR_API_KEY`: **[必需]** 你的大语言模型 API 密钥。请确保提供适用于所选 `-provider` 的密钥。

---

### 本地运行

#### 前提条件

*   安装 Go (版本 >= 1.18 推荐)。
*   设置 `MK_TRANSLATOR_API_KEY` 环境变量。

#### 设置与构建

1.  克隆或下载本仓库代码。
2.  进入项目根目录 (`Markdown-translator-go`)。
3.  (如果项目没有 `go.mod` 文件) 运行: `go mod init Markdown-translator-go`
4.  下载依赖: `go mod tidy`
5.  构建可执行文件: `go build -o Markdown-translator-go-app .`

#### 使用示例

```bash
# --- 使用 OpenAI (假设 Key 已设置) ---
export MK_TRANSLATOR_API_KEY='sk-YourOpenAIKey...'
./Markdown-translator-go-app \
    --source ./源Markdown目录路径 \
    --target ./输出中文目录路径 \
    --provider openai \
    --model "gpt-4o-mini" \
    --concurrency 10 \
    --overwrite

# --- 使用 Claude (假设 Key 已设置) ---
export MK_TRANSLATOR_API_KEY='YourClaudeKey...'
./Markdown-translator-go-app \
    --source ./源Markdown目录路径 \
    --target ./输出中文目录路径 \
    --provider claude \
    --model "claude-3-haiku-20240307" \
    --concurrency 8

# --- 使用 Gemini (假设 Key 已设置) ---
export MK_TRANSLATOR_API_KEY='YourGeminiKey...'
./Markdown-translator-go-app \
    --source ./源Markdown目录路径 \
    --target ./输出中文目录路径 \
    --provider gemini \
    --model "gemini-1.5-flash-latest" \
    --concurrency 12 \
    --dry-run # 示例：使用空跑模式

# --- 使用配置文件 ---
./Markdown-translator-go-app --config config.toml
```

---

### Docker 运行

使用 Docker 可以在任何支持 Docker 的环境中运行此工具，无需在本地安装 Go 环境。

#### 前提条件

*   安装 Docker

#### 构建 Docker 镜像

在项目根目录（包含 `Dockerfile` 的目录）运行：

```bash
docker build -t markdown-translator-go:latest .
```

#### 运行 Docker 容器

你需要将包含源文件的目录挂载到容器内作为源目录（只读），并将用于存放结果的目录挂载为目标目录（可写）。同时，通过环境变量传递 API Key。

```bash
# --- 示例：通过 Docker 使用 OpenAI 运行 ---
docker run --rm \
    -e MK_TRANSLATOR_API_KEY='sk-YourOpenAIKey...' \
    -v "/宿主机/源Markdown目录路径:/app/source:ro" \
    -v "/宿主机/输出中文目录路径:/app/target:rw" \
    markdown-translator-go:latest \
    --provider openai \
    --source /app/source \
    --target /app/target \
    --model "gpt-4o-mini" \
    --concurrency 10

# --- 示例：通过 Docker 使用 Claude 运行 ---
docker run --rm \
    -e MK_TRANSLATOR_API_KEY='YourClaudeKey...' \
    -v "/宿主机/源Markdown目录路径:/app/source:ro" \
    -v "/宿主机/输出中文目录路径:/app/target:rw" \
    markdown-translator-go:latest \
    --provider claude \
    --source /app/source \
    --target /app/target \
    --model "claude-3-sonnet-20240229" \
    --concurrency 8 \
    --overwrite # 示例：添加覆盖选项
```

**注意:**
*   请将 `/宿主机/源Markdown目录路径` 和 `/宿主机/输出中文目录路径` 替换为你宿主机上的实际路径。
*   `:ro` 表示以只读方式挂载，`:rw` 表示以可写方式挂载。
*   `--rm` 标志表示容器退出后自动删除该容器实例。

---

### 重要：适配与测试 LLM API 实现

本项目支持多种 LLM 提供商，各自的实现位于 `translator/` 目录下 (例如 `openai.go`, `claude.go`, `gemini.go`)。

**虽然提供了基本实现，但你仍需特别注意并可能需要调整：**

1.  **API Key**: 确保为所选的 `-provider` 提供了正确格式且有效的 API Key。
2.  **模型名称**: 确认使用的 `-model` 名称是所选提供商支持的，并且你有权限访问。
3.  **API 端点**: 检查 `--api-url` 是否适用于你的提供商和网络环境（例如，是否需要代理）。
4.  **请求/响应结构**: **像 `claude.go` 和 `gemini.go` 中的实现是基于其公开文档的。LLM API 可能会更新，实际的请求参数要求 (例如 Claude 的 `max_tokens`)、响应格式、错误结构可能与代码中的示例有所不同。请务必对照最新的官方 API 文档进行检查，并通过实际调用进行测试和调试，特别是错误处理逻辑。**
5.  **特定 Headers**: 某些 API (如 Claude) 需要特定的 HTTP Headers (例如 `anthropic-version`)。请确保这些 Header 在对应的实现中被正确设置。

**未能正确配置或适配特定提供商的 API 细节是导致翻译失败的最常见原因。请务必进行充分的测试！**

---

### Prompt 模板 (`prompt.template`)

程序默认会加载与可执行文件同目录下的 `prompt.template` 文件。你可以通过 `--prompt-file` 参数指定不同的模板文件。

默认的 Prompt 设计用于指示 LLM：
*   执行英译中翻译（或根据需要调整）。
*   专注于技术文档的准确性，保留技术术语。
*   **严格保留**原始 Markdown 格式（代码块、`{{占位符}}`、链接等）。
*   **只输出**翻译后的 Markdown 内容，不含任何额外解释。
*   **将完整的翻译结果用 `<translate>` 和 `</translate>` 标签包裹起来**，以便程序提取。

你可以根据所选 LLM 的特性和翻译效果，自由调整 Prompt 模板以获得最佳结果。

---

### 贡献

欢迎通过提交 Pull Requests 或 Issues 来改进此工具。

---

### 许可证

MIT License