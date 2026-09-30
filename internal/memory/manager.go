// Package memory — менеджер памяти модели.
//
// Отвечает исключительно за учёт свободной и занятой памяти. Внутреннее
// расположение образов заданий в модели не детализируется.
package memory

import (
	"fmt"

	"os_model/internal/process"
)

// ----------------------------------------------------------------------------------------
// MemoryManager отслеживает суммарный и занятый объём памяти.
type MemoryManager struct {
	TotalSize int // общий объём моделируемой памяти
	UsedSize  int // занятый объём памяти
}

// ----------------------------------------------------------------------------------------
// CheckAvailable проверяет, достаточно ли свободной памяти для загрузки.
func (m *MemoryManager) CheckAvailable(size int) bool {
	freeSize := m.TotalSize - m.UsedSize
	return freeSize >= size
}

// ----------------------------------------------------------------------------------------
// Allocate выделяет память под новое задание и регистрирует его.
// Возвращает ошибку, если свободной памяти недостаточно.
func (m *MemoryManager) Allocate(task process.Task) error {
	if !m.CheckAvailable(task.Size) {
		return fmt.Errorf("недостаточно памяти для задания %d: требуется %d, свободно %d",
			task.ID, task.Size, m.TotalSize-m.UsedSize)
	}

	m.UsedSize += task.Size
	return nil
}

// ----------------------------------------------------------------------------------------
// Free освобождает память при завершении задания.
func (m *MemoryManager) Free(task process.Task) {
	m.UsedSize -= task.Size
}

// ----------------------------------------------------------------------------------------
