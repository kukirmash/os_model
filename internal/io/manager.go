// Package io моделирует подсистему ввода-вывода.
//
// Управляет процессами, заблокированными на операциях ввода-вывода,
// и отсчитывает такты их обслуживания.
package io

import (
	"os_model/internal/process"
	"os_model/internal/scheduler"
)

// ----------------------------------------------------------------------------------------
// IOManager управляет очередью процессов, заблокированных по вводу-выводу.
type IOManager struct {
	BlockedQueue []*process.PSW // процессы в состоянии Blocked_IO

	// Planner — планировщик, в очередь готовности которого возвращаются
	// процессы, завершившие операцию ввода-вывода.
	Planner *scheduler.Planner
}

// ----------------------------------------------------------------------------------------
// StartIO переводит процесс в состояние блокировки и устанавливает
// счётчик тактов до завершения текущей операции ввода-вывода.
func (m *IOManager) StartIO(p *process.PSW, duration int) {
	p.State = process.StateBlockedIO
	p.IOTicksLeft = duration
	m.BlockedQueue = append(m.BlockedQueue, p)
}

// ----------------------------------------------------------------------------------------
// TickIO уменьшает счётчики тактов ввода-вывода на каждом такте моделирования.
// При достижении нуля процесс переводится в состояние Ready и возвращается
// планировщику.
func (m *IOManager) TickIO() {
	// Фильтрация на месте: в очереди остаются незавершённые операции.
	waiting := m.BlockedQueue[:0]
	for _, p := range m.BlockedQueue {
		p.IOTicksLeft--
		if p.IOTicksLeft > 0 {
			waiting = append(waiting, p)
			continue
		}

		p.IOTicksLeft = 0
		m.Planner.AddProcess(p)
	}
	m.BlockedQueue = waiting
}

// ----------------------------------------------------------------------------------------
