// Package scheduler реализует планировщик (вариант 11).
//
// Используются относительные динамические приоритеты: текущий активный
// процесс не вытесняется новым поступлением — выбор следующего процесса
// выполняется только при освобождении процессора.
package scheduler

import "os_model/internal/process"

// ----------------------------------------------------------------------------------------
// Planner инкапсулирует очередь готовности и алгоритм диспетчеризации.
type Planner struct {
	ReadyQueue []*process.PSW // процессы в состоянии Ready
}

// ----------------------------------------------------------------------------------------
// AddProcess добавляет процесс в очередь готовности.
func (p *Planner) AddProcess(psw *process.PSW) {
	psw.State = process.StateReady
	p.ReadyQueue = append(p.ReadyQueue, psw)
}

// ----------------------------------------------------------------------------------------
// GetNextProcess выбирает процесс с наивысшим DynamicPriority.
// Вызывается только при освобождении процессора.
func (p *Planner) GetNextProcess() *process.PSW {
	if len(p.ReadyQueue) == 0 {
		return nil
	}

	// Формула выбора: процесс с максимальным DynamicPriority;
	// при равенстве приоритетов выбирается стоящий раньше в очереди.
	best := 0
	for i, psw := range p.ReadyQueue {
		if psw.DynamicPriority > p.ReadyQueue[best].DynamicPriority {
			best = i
		}
	}

	chosen := p.ReadyQueue[best]
	p.ReadyQueue = append(p.ReadyQueue[:best], p.ReadyQueue[best+1:]...) // иключение из очереди готовнсти
	return chosen
}

// ----------------------------------------------------------------------------------------
// UpdatePriorities пересчитывает приоритеты процессов в очереди готовности.
// Вызывается каждый такт или при каждом обращении к планировщику.
//
// Вариант 11: прирост DynamicPriority тем больше, чем больше размер задания
// (PSW.Task.Size) — приоритет больших заданий увеличивается сильнее.
func (p *Planner) UpdatePriorities() {
	for _, psw := range p.ReadyQueue {
		// Прирост за такт ожидания пропорционален размеру задания.
		psw.DynamicPriority += psw.Task.Size
	}
}

// ----------------------------------------------------------------------------------------
