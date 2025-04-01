package utils

import (
	"fmt"
	"log" // 导入 log 包用于记录详细错误
	"regexp"
	"strings"
)

// `(?s)`: 单行模式，让 `.` 可以匹配换行符。
// `.*?`: 非贪婪匹配，匹配 <translate> 和 </translate> 之间的任意字符。
var translateTagRegex = regexp.MustCompile(`(?s)<translate>(.*?)</translate>`)

// ExtractTranslation 函数从 LLM 返回的原始字符串中提取被 `<translate>` 和 `</translate>` 包裹的内容。
func ExtractTranslation(rawLLMOutput string) (string, error) {
	// 尝试直接匹配
	matches := translateTagRegex.FindStringSubmatch(rawLLMOutput)

	// 如果直接匹配失败，可能是标签前后有额外的空白字符
	if len(matches) < 2 { // 索引 0 是整个匹配，索引 1 是第一个捕获组
		trimmedOutput := strings.TrimSpace(rawLLMOutput)
		matches = translateTagRegex.FindStringSubmatch(trimmedOutput)
		// 如果去除空白后仍然匹配失败
		if len(matches) < 2 {
			// 记录详细错误信息，包括部分原始输出，便于调试
			preview := rawLLMOutput
			if len(preview) > 300 { // 限制日志中预览的长度
				preview = preview[:300] + "..."
			}
			errMsg := fmt.Sprintf("无法在 LLM 输出中找到 <translate>...</translate> 标签。输出预览 (最多300字符): %s", preview)
			log.Println("错误: " + errMsg) // 使用 log 记录更详细的信息
			return "", fmt.Errorf(errMsg) // 返回错误
		}
	}

	// 返回第一个捕获组的内容 (索引为 1)，并去除其两端的空白字符
	return strings.TrimSpace(matches[1]), nil
}
