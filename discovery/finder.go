package discovery

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// FindMarkdownFiles 递归地查找指定源目录下的所有 `.md` 文件。
// 它返回一个包含相对于源目录的文件路径的字符串切片。
func FindMarkdownFiles(sourceDir string) ([]string, error) {
	var files []string
	log.Printf("开始在目录中查找文件: %s\n", sourceDir)

	err := filepath.WalkDir(sourceDir, func(path string, d fs.DirEntry, walkErr error) error {
		// 首先处理 WalkDir 本身可能遇到的错误
		if walkErr != nil {
			log.Printf("警告: 访问路径 %q 时出错: %v\n", path, walkErr)
			// 如果是源目录本身的权限问题，则应停止遍历
			if path == sourceDir && os.IsPermission(walkErr) {
				return fmt.Errorf("源目录 %s 权限不足: %w", sourceDir, walkErr)
			}
			// 如果发生错误时无法获取目录项信息
			if d == nil {
				log.Printf("警告: 无法获取 %q 的目录项信息，跳过。\n", path)
				return nil // 跳过这个无法处理的条目
			}
			// 如果是目录访问错误，可以选择跳过该目录及其所有子项
			if d.IsDir() {
				log.Printf("警告: 跳过目录 %q 因为错误: %v\n", path, walkErr)
				return fs.SkipDir // 跳过此目录
			}
			// 对于文件错误，只跳过当前文件
			return nil // 跳过出错的单个文件条目
		}

		// 检查是否是文件并且后缀是 .md (不区分大小写)
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			// 计算相对于 sourceDir 的路径
			relPath, err := filepath.Rel(sourceDir, path)
			if err != nil {
				// 理论上，如果 path 是 walkDir 找到的，它应该总在 sourceDir 之下
				// 但以防万一，记录错误并跳过
				log.Printf("警告: 无法获取 %q 相对于 %q 的路径: %v\n", path, sourceDir, err)
				return nil // 跳过这个文件
			}
			files = append(files, relPath)
			// log.Printf("发现文件: %s\n", relPath) // 如果需要详细日志，取消此行注释
		}
		return nil // 继续遍历
	})

	// 处理 WalkDir 返回的最终错误（如果中途没有被 return 掉）
	if err != nil {
		log.Printf("文件遍历过程中出错: %v\n", err)
		return nil, fmt.Errorf("文件发现失败: %w", err)
	}

	log.Printf("文件查找完成。共发现 %d 个 markdown 文件。\n", len(files))
	return files, nil
}
