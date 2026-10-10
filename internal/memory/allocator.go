// Package memory — супервизор памяти модели.
//
// Отвечает исключительно за учёт свободной и занятой памяти. Внутреннее
// расположение образов заданий в модели не детализируется.
package memory

import (
	"fmt"

	"os_model/internal/process"
)

// ----------------------------------------------------------------------------------------
// Allocator отслеживает суммарный и занятый объём памяти.
type Allocator struct {
	TotalSize int // общий объём моделируемой памяти
	UsedSize  int // занятый объём памяти
}

// ----------------------------------------------------------------------------------------
// CheckAvailable проверяет, достаточно ли свободной памяти для загрузки.
func (a *Allocator) CheckAvailable(size int) bool {
	freeSize := a.TotalSize - a.UsedSize
	return freeSize >= size
}

// ----------------------------------------------------------------------------------------
// Allocate выделяет память под новое задание и регистрирует его.
// Возвращает ошибку, если свободной памяти недостаточно.
func (a *Allocator) Allocate(task process.Task) error {
	if !a.CheckAvailable(task.Size) {
		return fmt.Errorf("недостаточно памяти для задания %d: требуется %d, свободно %d",
			task.ID, task.Size, a.TotalSize-a.UsedSize)
	}

	a.UsedSize += task.Size
	return nil
}

// ----------------------------------------------------------------------------------------
// Free освобождает память при завершении задания.
func (a *Allocator) Free(task process.Task) {
	a.UsedSize -= task.Size
}
