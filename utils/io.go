package utils

import (
	"fmt"
	"os"
	"path/filepath"
	// "log" // 如果需要记录跳过信息，取消此行注释
)

// ReadFile 函数读取指定路径文件的全部内容，并以字符串形式返回。
func ReadFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		// 使用 %w 包装原始错误，保留错误上下文
		return "", fmt.Errorf("读取文件 %s 失败: %w", path, err)
	}
	return string(content), nil
}

// WriteFile 函数将字符串内容写入指定路径的文件。
// 它会确保目标文件的父目录存在。
// overwrite 参数控制是否覆盖已存在的文件。
func WriteFile(path string, content string, overwrite bool) error {
	// --- 文件存在性检查 (仅当不允许覆盖时) ---
	if !overwrite {
		// os.Stat 获取文件信息。如果 err 为 nil，表示文件已存在。
		if _, err := os.Stat(path); err == nil {
			// 文件存在，且不允许覆盖，则直接返回 nil (表示成功跳过，不是错误)
			// log.Printf("信息: 跳过已存在的文件 (未设置覆盖): %s\n", path) // 可选日志
			return nil
		} else if !os.IsNotExist(err) {
			// 如果 Stat 返回错误，但不是 "文件不存在" (例如权限问题)，则这是一个需要报告的错误。
			return fmt.Errorf("检查目标文件 %s 状态失败: %w", path, err)
		}
		// 如果 err 是 os.IsNotExist(err)，则表示文件不存在，可以继续写入。
	}

	// --- 确保目录存在 ---
	// 获取目标文件所在的目录路径。
	dir := filepath.Dir(path)
	// os.MkdirAll 会递归创建所有必需的父目录，如果目录已存在则什么也不做。
	// 0755 是常用的目录权限 (owner: rwx, group: rx, others: rx)。
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录 %s 失败: %w", dir, err)
	}

	// --- 写入文件 ---
	// os.WriteFile 将内容写入文件。如果文件已存在，它会被覆盖 (这是 WriteFile 的默认行为)。
	// 0644 是常用的文件权限 (owner: rw, group: r, others: r)。
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("写入文件 %s 失败: %w", path, err)
	}

	// 写入成功
	return nil
}
