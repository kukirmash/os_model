// Команда osmodel — точка входа модели операционной системы (вариант 11).
package main

import "os_model/internal/os"

func main() {
	system := &os.System{
		Config: os.Config{
			MemorySize:         8192,
			TickDurationMs:     100,
			IOMaxDuration:      10,
			PrioritySizeFactor: 1,
		},
	}

	// Запуск модели. Работает до получения директивы завершения
	// (или прерывается по Ctrl+C).
	system.Init()
	system.RunLoop()
}
