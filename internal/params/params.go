// Package params хранит входные параметры модели ОС и правила их генерации
// (лабораторная работа 8).
//
// Параметры читаются из JSON-файла один раз при запуске модели. Для каждого
// параметра задаются минимум, максимум, дискретность и комментарий.
package params

import "fmt"

// DefaultPath — имя файла параметров по умолчанию.
const DefaultPath = "params.json"

// DefaultSeed — фиксированное зерно генератора псевдослучайных чисел.
// Одинаковое зерно обеспечивает повторяемость результатов при каждом запуске.
const DefaultSeed uint64 = 2026

// ----------------------------------------------------------------------------------------
// Param — диапазон и дискретность одного входного параметра.
type Param struct {
	Min     int    `json:"min"`     // минимальное значение
	Max     int    `json:"max"`     // максимальное значение
	Step    int    `json:"step"`    // дискретность
	Comment string `json:"comment"` // комментарий для индикации
}

// ----------------------------------------------------------------------------------------
// Costs — диапазоны длительностей системных действий.
type Costs struct {
	Load        Param `json:"load"`
	Dispatch    Param `json:"dispatch"`
	IOInit      Param `json:"ioInit"`
	IOInterrupt Param `json:"ioInterrupt"`
	Terminate   Param `json:"terminate"`
}

// ----------------------------------------------------------------------------------------
// System — диапазоны системных параметров.
type System struct {
	MemorySize     Param `json:"memorySize"`
	TickDurationMs Param `json:"tickDurationMs"`
	IOMaxDuration  Param `json:"ioMaxDuration"`
	Costs          Costs `json:"costs"`
}

// ----------------------------------------------------------------------------------------
// Task — диапазоны параметров генерируемых заданий.
type Task struct {
	Size         Param `json:"size"`
	Commands     Param `json:"commands"`
	IOPercent    Param `json:"ioPercent"`
	BasePriority Param `json:"basePriority"`
}

// ----------------------------------------------------------------------------------------
// Set — полный набор параметров из файла.
type Set struct {
	Seed   uint64 `json:"seed"`   // зерно генератора (0 — значение по умолчанию)
	System System `json:"system"` // системные параметры
	Task   Task   `json:"task"`   // параметры заданий
}

// ----------------------------------------------------------------------------------------
// CostsValues — сгенерированные длительности системных действий.
type CostsValues struct {
	Load        int
	Dispatch    int
	IOInit      int
	IOInterrupt int
	Terminate   int
}

// ----------------------------------------------------------------------------------------
// SystemValues — сгенерированные значения системных параметров.
type SystemValues struct {
	MemorySize     int
	TickDurationMs int
	IOMaxDuration  int
	Costs          CostsValues
}

// ----------------------------------------------------------------------------------------
// TaskValues — сгенерированные значения параметров задания.
type TaskValues struct {
	Size         int
	Commands     int
	IOPercent    int
	BasePriority int
}

// ----------------------------------------------------------------------------------------
// Default возвращает параметры по умолчанию. Они используются, если файл
// параметров не найден или не задан.
func Default() Set {
	return Set{
		Seed: DefaultSeed,
		System: System{
			MemorySize:     Param{Min: 4096, Max: 8192, Step: 1024, Comment: "Объём памяти, единиц"},
			TickDurationMs: Param{Min: 10, Max: 100, Step: 10, Comment: "Длительность такта, мс"},
			IOMaxDuration:  Param{Min: 1, Max: 10, Step: 1, Comment: "Максимальная длительность ВВ, тактов"},
			Costs: Costs{
				Load:        Param{Min: 1, Max: 1, Step: 1, Comment: "Затраты на загрузку задания, тактов"},
				Dispatch:    Param{Min: 1, Max: 1, Step: 1, Comment: "Затраты на выбор процесса, тактов"},
				IOInit:      Param{Min: 1, Max: 1, Step: 1, Comment: "Затраты на инициализацию ВВ, тактов"},
				IOInterrupt: Param{Min: 1, Max: 1, Step: 1, Comment: "Затраты на обслуживание прерывания ВВ, тактов"},
				Terminate:   Param{Min: 1, Max: 1, Step: 1, Comment: "Затраты на завершение задания, тактов"},
			},
		},
		Task: Task{
			Size:         Param{Min: 100, Max: 500, Step: 50, Comment: "Размер задания, единиц памяти"},
			Commands:     Param{Min: 10, Max: 20, Step: 1, Comment: "Число команд задания"},
			IOPercent:    Param{Min: 0, Max: 50, Step: 5, Comment: "Процент команд ВВ, %"},
			BasePriority: Param{Min: 1, Max: 10, Step: 1, Comment: "Базовый приоритет задания"},
		},
	}
}

// ----------------------------------------------------------------------------------------
// IsZero сообщает, что набор параметров не заполнен.
func (s Set) IsZero() bool {
	return s.System.MemorySize.Step == 0 &&
		s.System.TickDurationMs.Step == 0 &&
		s.System.IOMaxDuration.Step == 0 &&
		s.Task.Size.Step == 0
}

// ----------------------------------------------------------------------------------------
// Validate проверяет корректность всех диапазонов.
func (s Set) Validate() error {
	params := []struct {
		name string
		p    Param
	}{
		{"system.memorySize", s.System.MemorySize},
		{"system.tickDurationMs", s.System.TickDurationMs},
		{"system.ioMaxDuration", s.System.IOMaxDuration},
		{"system.costs.load", s.System.Costs.Load},
		{"system.costs.dispatch", s.System.Costs.Dispatch},
		{"system.costs.ioInit", s.System.Costs.IOInit},
		{"system.costs.ioInterrupt", s.System.Costs.IOInterrupt},
		{"system.costs.terminate", s.System.Costs.Terminate},
		{"task.size", s.Task.Size},
		{"task.commands", s.Task.Commands},
		{"task.ioPercent", s.Task.IOPercent},
		{"task.basePriority", s.Task.BasePriority},
	}

	for _, item := range params {
		if err := item.p.validate(item.name); err != nil {
			return err
		}
	}

	if s.System.MemorySize.Min < 1 {
		return fmt.Errorf("system.memorySize: минимум должен быть не меньше 1")
	}
	if s.System.TickDurationMs.Min < 1 {
		return fmt.Errorf("system.tickDurationMs: минимум должен быть не меньше 1")
	}
	if s.System.IOMaxDuration.Min < 1 {
		return fmt.Errorf("system.ioMaxDuration: минимум должен быть не меньше 1")
	}
	if s.Task.Size.Min < 1 {
		return fmt.Errorf("task.size: минимум должен быть не меньше 1")
	}
	if s.Task.Commands.Min < 1 {
		return fmt.Errorf("task.commands: минимум должен быть не меньше 1")
	}
	if s.Task.IOPercent.Min < 0 || s.Task.IOPercent.Max > 100 {
		return fmt.Errorf("task.ioPercent: диапазон должен находиться в пределах 0..100")
	}
	if s.Task.BasePriority.Min < 1 {
		return fmt.Errorf("task.basePriority: минимум должен быть не меньше 1")
	}

	return nil
}

// ----------------------------------------------------------------------------------------
// validate проверяет один диапазон.
func (p Param) validate(name string) error {
	if p.Step <= 0 {
		return fmt.Errorf("%s: шаг должен быть больше 0", name)
	}
	if p.Min > p.Max {
		return fmt.Errorf("%s: минимум %d больше максимума %d", name, p.Min, p.Max)
	}
	return nil
}
