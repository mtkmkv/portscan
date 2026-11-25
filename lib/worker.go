package lib

import (
	"context"
	"sync"
)

// Задача проверки порта на разрешённой цели
type scanTask struct {
	target resolvedTarget
	port   uint16
}

// Параметры конкурентного выполнения проверок
type workerConfig struct {
	Concurrency int
	Dial        dialConfig
}

// Конкурентный запуск проверок портов с возвратом канала результатов
func runWorkers(
	ctx context.Context,
	targets []resolvedTarget,
	ports []uint16,
	cfg workerConfig,
) <-chan Result {
	concurrency := cfg.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}

	tasks := make(chan scanTask, concurrency)
	results := make(chan Result, concurrency)

	if len(targets) == 0 || len(ports) == 0 {
		close(tasks)
		close(results)
		return results
	}

	go enqueueTasks(ctx, targets, ports, tasks)

	var wg sync.WaitGroup
	wg.Add(concurrency)
	for range concurrency {
		go func() {
			defer wg.Done()
			for task := range tasks {
				if err := ctx.Err(); err != nil {
					return
				}
				res := probeTCP(ctx, task.target, task.port, cfg.Dial)
				select {
				case results <- res:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	return results
}

// Постановка задач в очередь до завершения или отмены контекста
func enqueueTasks(
	ctx context.Context,
	targets []resolvedTarget,
	ports []uint16,
	tasks chan<- scanTask,
) {
	defer close(tasks)

	for _, target := range targets {
		for _, port := range ports {
			if ctx.Err() != nil {
				return
			}
			select {
			case tasks <- scanTask{target: target, port: port}:
			case <-ctx.Done():
				return
			}
		}
	}
}
