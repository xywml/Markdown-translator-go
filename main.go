package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"Markdown-translator-go/config"
	"Markdown-translator-go/discovery"
	"Markdown-translator-go/processor"
	"Markdown-translator-go/translator"
)

func main() {
	// 记录程序开始时间，用于计算总耗时
	startTime := time.Now()
	// 配置标准日志库，添加日期、时间和微秒输出，便于追踪和调试
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.Println("启动 Markdown-translator-go...")

	// 设置信号处理，以便程序可以优雅地退出
	setupSignalHandler()

	// --- 步骤 1: 加载和校验配置 ---
	log.Println("加载配置信息...")
	cfg, err := config.LoadConfig()
	if err != nil {
		// 配置加载失败是致命错误，记录并退出
		log.Fatalf("配置错误: %v", err) // Fatalf 会打印日志并调用 os.Exit(1)
	}
	// 打印加载的关键配置信息
	log.Printf("配置加载完成: 源='%s', 目标='%s', 并发=%d, 提供商='%s', 模型='%s', 覆盖=%t, 空跑=%t",
		cfg.SourceDir, cfg.TargetDir, cfg.Concurrency, cfg.LLMProvider, cfg.LLMModel, cfg.Overwrite, cfg.DryRun)
	if cfg.DryRun {
		log.Println("!!! 注意：已启用空跑(Dry Run)模式 !!! 不会实际调用 API 或写入文件。")
	}
	log.Printf("使用的 Prompt 文件: %s", cfg.PromptFile)

	// --- 步骤 2: 发现需要翻译的文件 ---
	log.Println("开始在源目录中查找 Markdown 文件...")
	filesToProcess, err := discovery.FindMarkdownFiles(cfg.SourceDir)
	if err != nil {
		log.Fatalf("查找 Markdown 文件失败: %v", err)
	}

	// 如果没有找到文件，则无需继续，正常退出
	if len(filesToProcess) == 0 {
		log.Println("在源目录中未找到任何 Markdown 文件。程序退出。")
		os.Exit(0)
	}
	log.Printf("发现 %d 个 Markdown 文件待处理。\n", len(filesToProcess))

	// --- 步骤 3: 初始化翻译器实例 (使用工厂模式) ---
	var llmTrans translator.Translator // 使用接口类型，与具体实现解耦
	// 仅在非空跑模式下才需要初始化实际的 Translator
	if !cfg.DryRun {
		log.Printf("初始化 LLM 翻译器 (提供商: %s)...", cfg.LLMProvider)
		// 调用工厂函数创建对应提供商的 Translator 实例
		llmTrans, err = translator.NewTranslator(cfg)
		if err != nil {
			// 初始化失败是致命错误
			log.Fatalf("初始化 LLM 翻译器失败: %v", err)
		}
		log.Println("LLM 翻译器初始化成功。")

		// 如果翻译器支持关闭，确保在程序结束时关闭
		if closer, ok := llmTrans.(translator.Closer); ok {
			defer func() {
				log.Println("关闭 LLM 翻译器连接...")
				if err := closer.Close(); err != nil {
					log.Printf("关闭 LLM 翻译器时出错: %v", err)
				}
			}()
		}
	} else {
		// 在空跑模式下，不需要实际的 Translator 实例
		log.Println("空跑(Dry Run)模式：跳过 LLM 翻译器初始化。")
		llmTrans = nil // worker 逻辑会处理 trans 为 nil 的情况 (在 dry run 分支跳过调用)
	}

	// --- 步骤 4: 并发处理所有文件 ---
	log.Println("开始并发处理文件...")
	// 调用处理函数，传入配置、文件列表和 (可能为 nil 的) Translator 实例
	stats := processor.ProcessFiles(cfg, filesToProcess, llmTrans)

	// --- 步骤 5: 报告处理结果总结 ---
	duration := time.Since(startTime) // 计算总耗时
	fmt.Println("\n--- 翻译任务总结 ---")
	fmt.Printf("发现文件总数:        %d\n", stats.TotalFiles)
	if cfg.DryRun {
		// 在空跑模式下，报告模拟处理的文件数
		fmt.Printf("处理文件数 (空跑):    %d\n", stats.DryRunHits.Load())
	} else {
		// 在正常模式下，报告实际处理、跳过和失败的文件数
		fmt.Printf("成功处理文件数:      %d\n", stats.Processed.Load())
		fmt.Printf("跳过文件数 (已存在): %d\n", stats.Skipped.Load())
	}
	fmt.Printf("失败文件数:          %d\n", stats.Failed.Load())
	fmt.Printf("总耗时:              %v\n", duration)
	fmt.Println("--------------------")

	// --- 步骤 6: 根据结果决定退出状态码 ---
	// 如果有任何文件处理失败，以非零状态码退出，表示程序执行中存在问题
	if stats.Failed.Load() > 0 {
		log.Printf("处理完成，但有 %d 个文件处理失败。请检查以上日志获取详细信息。", stats.Failed.Load())
		os.Exit(1) // 使用 1 作为通用的错误退出码
	}

	// 如果所有文件都处理成功 (或在空跑模式下完成)，则正常退出
	log.Println("翻译处理流程成功完成。")
	// 默认退出码为 0，表示成功
}

// setupSignalHandler 设置信号处理器，以便程序可以优雅地退出
func setupSignalHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-c
		log.Printf("接收到信号 %v，正在优雅退出...", sig)
		os.Exit(0)
	}()
}
