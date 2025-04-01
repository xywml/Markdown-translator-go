package processor

import (
	"context"
	"log"
	"os" // 导入 os 包
	"path/filepath"
	"sync"
	"sync/atomic" // 使用原子操作保证计数器线程安全
	"time"        // 导入 time 包

	"Markdown-translator-go/config"
	"Markdown-translator-go/translator"
	"Markdown-translator-go/utils"
)

// TranslationTask 结构体包含处理单个文件所需的所有信息。
type TranslationTask struct {
	RelativePath string // 文件相对于源/目标基础目录的路径。
}

// Stats 结构体用于跟踪处理过程中的统计数据。
type Stats struct {
	TotalFiles int32        // 发现的总文件数。
	Processed  atomic.Int32 // 成功处理的文件数 (成功调用API并写入或跳过)。
	Skipped    atomic.Int32 // 因目标文件已存在且未设置覆盖而跳过的文件数。
	Failed     atomic.Int32 // 处理过程中遇到错误的文件数。
	DryRunHits atomic.Int32 // 在空跑模式下“模拟处理”的文件数。
}

// ProcessFiles 函数设置 Worker 池（一组 Goroutine），并将文件处理任务分发给它们。
func ProcessFiles(cfg *config.Config, files []string, trans translator.Translator) *Stats {
	numFiles := len(files)
	stats := &Stats{TotalFiles: int32(numFiles)} // 初始化统计对象
	log.Printf("开始处理 %d 个文件，使用 %d 个 Worker...\n", numFiles, cfg.Concurrency)

	// 创建一个带缓冲区的 channel 用于传递任务。缓冲区大小设为文件数，避免发送者阻塞。
	tasks := make(chan TranslationTask, numFiles)
	// 使用 sync.WaitGroup 等待所有 Worker Goroutine 完成任务。
	var wg sync.WaitGroup

	// 启动指定数量的 Worker Goroutine。
	for i := 0; i < cfg.Concurrency; i++ {
		wg.Add(1) // 每启动一个 Worker，计数器加 1。
		// 启动 Goroutine 执行 worker 函数，传入 Worker ID (用于日志区分) 和其他必要参数。
		go worker(i+1, cfg, tasks, trans, &wg, stats)
	}

	// 将所有待处理的文件路径封装成 TranslationTask，发送到 tasks channel。
	for _, relPath := range files {
		tasks <- TranslationTask{RelativePath: relPath}
	}
	// 所有任务都已发送完毕，关闭 tasks channel。
	// Worker 在读完 channel 中所有数据后会检测到 channel 关闭并退出循环。
	close(tasks)

	// 等待所有 Worker Goroutine 调用 wg.Done()，表示它们已完成工作。
	wg.Wait()

	log.Printf("所有 Worker 已完成工作。\n")
	return stats // 返回包含处理结果的统计对象。
}

// worker 函数是每个并发 Goroutine 执行的核心逻辑。
// 它从 tasks channel 接收任务，处理单个文件的翻译，直到 channel 关闭。
func worker(id int, cfg *config.Config, tasks <-chan TranslationTask, trans translator.Translator, wg *sync.WaitGroup, stats *Stats) {
	// defer 语句确保在 worker 函数退出前（无论是正常结束还是 panic），都会调用 wg.Done()。
	defer wg.Done()
	log.Printf("[Worker %d] 启动。\n", id)

	// 使用 for range 循环从 tasks channel 接收任务。
	// 当 channel 关闭且所有数据都被读取后，循环会自动结束。
	for task := range tasks {
		// 构建源文件和目标文件的完整路径。
		sourcePath := filepath.Join(cfg.SourceDir, task.RelativePath)
		targetPath := filepath.Join(cfg.TargetDir, task.RelativePath)

		log.Printf("[Worker %d] 正在处理: %s -> %s\n", id, task.RelativePath, targetPath)

		// --- 检查目标文件是否存在以及是否需要跳过 ---
		// 仅在非空跑模式且未设置覆盖模式时执行此检查。
		if !cfg.Overwrite && !cfg.DryRun {
			// os.Stat 返回文件信息。如果 error 为 nil，表示文件存在。
			if _, err := os.Stat(targetPath); err == nil {
				log.Printf("[Worker %d] 跳过已存在的文件: %s\n", id, targetPath)
				stats.Skipped.Add(1) // 原子地增加跳过计数。
				continue             // 跳过当前任务，处理下一个。
			} else if !os.IsNotExist(err) {
				// 如果 Stat 返回错误，但不是 "文件不存在" 错误 (例如权限问题)，则记录错误并跳过。
				log.Printf("[Worker %d] 检查目标文件 %s 状态时出错: %v\n", id, targetPath, err)
				stats.Failed.Add(1) // 原子地增加失败计数。
				continue            // 处理下一个任务。
			}
			// 如果文件不存在 (os.IsNotExist(err) is true)，则继续后续处理。
		}

		// --- 读取源文件内容 ---
		content, err := utils.ReadFile(sourcePath)
		if err != nil {
			log.Printf("[Worker %d] 读取源文件 %s 时出错: %v\n", id, sourcePath, err)
			stats.Failed.Add(1)
			continue // 跳过当前任务。
		}

		// --- 处理空跑 (Dry Run) 模式 ---
		if cfg.DryRun {
			log.Printf("[Worker %d] [空跑模式] 将翻译并写入 (模拟): %s\n", id, targetPath)
			stats.DryRunHits.Add(1) // 增加空跑命中计数。
			// 在空跑模式下，我们认为这个文件被“处理”了，即使没有实际操作。
			// stats.Processed.Add(1) // 可以选择也增加 Processed 计数，或仅用 DryRunHits。
			continue // 跳过后续的 API 调用和文件写入。
		}

		// --- 检查 Translator 实例是否有效 ---
		// 在非空跑模式下，trans 不应为 nil。这是个健壮性检查。
		if trans == nil {
			log.Printf("[Worker %d] 错误: Translator 实例未初始化 (可能处于空跑模式但逻辑出错)。跳过 %s\n", id, task.RelativePath)
			stats.Failed.Add(1)
			continue
		}

		// --- 调用 LLM API 进行翻译 ---
		// 创建一个带有超时的 Context，用于控制 API 调用时间。
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second) // 例如，设置 120 秒超时。
		translatedRaw, err := trans.Translate(ctx, content)                       // 调用所选 Provider 的 Translate 方法。
		cancel()                                                                  // 及时调用 cancel 释放 Context 相关资源。

		if err != nil {
			// 如果翻译过程中出错 (网络问题、API 错误等)，记录错误并跳过。
			log.Printf("[Worker %d] 翻译文件 %s 时出错: %v\n", id, task.RelativePath, err)
			stats.Failed.Add(1)
			continue
		}

		// --- 从 LLM 的原始响应中提取 <translate> 标签内的内容 ---
		translatedContent, err := utils.ExtractTranslation(translatedRaw)
		if err != nil {
			// 如果提取失败 (例如 LLM 未按要求添加标签)，记录错误。
			// ExtractTranslation 内部已经记录了详细的错误信息和预览。
			log.Printf("[Worker %d] 提取文件 %s 的翻译内容失败: %v\n", id, task.RelativePath, err)
			stats.Failed.Add(1)
			continue
		}

		// --- 将提取到的翻译内容写入目标文件 ---
		// 使用配置中的 Overwrite 标志。
		err = utils.WriteFile(targetPath, translatedContent, cfg.Overwrite)
		if err != nil {
			// 如果写入失败 (例如磁盘空间不足、权限问题)，记录错误。
			log.Printf("[Worker %d] 写入目标文件 %s 时出错: %v\n", id, targetPath, err)
			stats.Failed.Add(1)
			continue // 处理下一个任务。
		}
		// 如果 WriteFile 没有返回错误，表示写入成功或因未设置覆盖而已存在被跳过 (返回 nil)。
		// 两种情况都表示这个文件处理成功。
		log.Printf("[Worker %d] 成功处理并写入 (或已跳过): %s\n", id, targetPath)
		stats.Processed.Add(1) // 原子地增加成功处理计数。

	} // 结束 for range 循环，当前 Worker 完成所有分配的任务。
	log.Printf("[Worker %d] 结束。\n", id)
} // Worker 函数返回，wg.Done() 被调用。
