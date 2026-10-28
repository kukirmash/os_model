package os

import (
	"fmt"

	"os_model/internal/process"
	"os_model/internal/regulator"
)

// ----------------------------------------------------------------------------------------
// DirectiveKind — вид управляющей директивы оператора (лабораторная работа 9).
type DirectiveKind int

const (
	DirectivePause     DirectiveKind = iota // пауза моделирования
	DirectiveResume                         // продолжить моделирование
	DirectiveSpeedUp                        // увеличить скорость
	DirectiveSpeedDown                      // уменьшить скорость
	DirectiveQuit                           // выключить питание

	DirectiveAutoLoadPause  // приостановить автоматическую загрузку
	DirectiveAutoLoadResume // продолжить автоматическую загрузку
	DirectiveLoadTask       // загрузить новое задание
	DirectiveSetNextTask    // задать параметры следующего задания
	DirectiveClearNextTask  // сбросить переопределение параметров
	DirectiveSuspendProcess // приостановить выбранный процесс
	DirectiveResumeProcess  // активизировать выбранный процесс
	DirectiveDestroyProcess // уничтожить выбранный процесс
	DirectiveFinishAndQuit  // выполнить загруженные задания и выключить
	DirectiveSetMemory      // изменить размер памяти
)

// ----------------------------------------------------------------------------------------
// String возвращает имя директивы для индикации.
func (k DirectiveKind) String() string {
	switch k {
	case DirectivePause:
		return "пауза модели"
	case DirectiveResume:
		return "продолжить модель"
	case DirectiveSpeedUp:
		return "увеличить скорость"
	case DirectiveSpeedDown:
		return "уменьшить скорость"
	case DirectiveQuit:
		return "выключить питание"
	case DirectiveAutoLoadPause:
		return "пауза автозагрузки"
	case DirectiveAutoLoadResume:
		return "возобновить автозагрузку"
	case DirectiveLoadTask:
		return "загрузить задание"
	case DirectiveSetNextTask:
		return "параметры следующего задания"
	case DirectiveClearNextTask:
		return "сбросить параметры задания"
	case DirectiveSuspendProcess:
		return "приостановить процесс"
	case DirectiveResumeProcess:
		return "активизировать процесс"
	case DirectiveDestroyProcess:
		return "уничтожить процесс"
	case DirectiveFinishAndQuit:
		return "выполнить и выключить"
	case DirectiveSetMemory:
		return "изменить память"
	default:
		return "неизвестная директива"
	}
}

// ----------------------------------------------------------------------------------------
// Directive — директива оператора вместе с параметрами.
type Directive struct {
	Kind       DirectiveKind
	ProcessID  int
	Task       *process.Task
	MemorySize int
}

// ----------------------------------------------------------------------------------------
// DirectiveResult — результат выполнения последней директивы.
type DirectiveResult struct {
	Time    int    `json:"time"`
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// ----------------------------------------------------------------------------------------
// applyDirective выполняет одну директиву и запоминает результат.
func (sys *System) applyDirective(d Directive) {
	result := DirectiveResult{
		Time: sys.Time,
		Name: d.Kind.String(),
		OK:   true,
	}

	switch d.Kind {
	case DirectivePause:
		sys.Paused = true
		result.Message = "моделирование приостановлено"

	case DirectiveResume:
		sys.Paused = false
		result.Message = "моделирование продолжено"

	case DirectiveSpeedUp:
		sys.changeSpeed(speedStepPercent)
		result.Message = fmt.Sprintf("скорость %.1f такт/с", sys.Speed)

	case DirectiveSpeedDown:
		sys.changeSpeed(-speedStepPercent)
		result.Message = fmt.Sprintf("скорость %.1f такт/с", sys.Speed)

	case DirectiveQuit:
		sys.Quit = true
		result.Message = "питание выключено"

	case DirectiveAutoLoadPause:
		sys.AutoLoadEnabled = false
		result.Message = "автоматическая загрузка приостановлена"

	case DirectiveAutoLoadResume:
		sys.AutoLoadEnabled = true
		result.Message = "автоматическая загрузка возобновлена"

	case DirectiveLoadTask:
		if sys.GenerateAndLoadTask() {
			result.Message = fmt.Sprintf("задание #%d загружено", sys.LastTask.Task.ID)
		} else {
			result.OK = false
			result.Message = "задание не загружено: " + sys.LastTask.Reason
		}

	case DirectiveSetNextTask:
		if d.Task == nil {
			result.OK = false
			result.Message = "не заданы параметры задания"
			break
		}
		if err := validateTask(*d.Task); err != nil {
			result.OK = false
			result.Message = err.Error()
			break
		}
		task := *d.Task
		sys.NextTask = &task
		result.Message = fmt.Sprintf("следующее задание: размер %d, команд %d, ВВ %d %%, приоритет %d",
			task.Size, task.TotalCommands, task.IOPercent, task.BasePriority)

	case DirectiveClearNextTask:
		sys.NextTask = nil
		result.Message = "параметры следующего задания сброшены"

	case DirectiveSuspendProcess:
		if err := sys.suspendProcess(d.ProcessID); err != nil {
			result.OK = false
			result.Message = err.Error()
		} else {
			result.Message = fmt.Sprintf("процесс #%d приостановлен", d.ProcessID)
		}

	case DirectiveResumeProcess:
		if err := sys.resumeProcess(d.ProcessID); err != nil {
			result.OK = false
			result.Message = err.Error()
		} else {
			result.Message = fmt.Sprintf("процесс #%d возобновлён", d.ProcessID)
		}

	case DirectiveDestroyProcess:
		if err := sys.destroyProcess(d.ProcessID); err != nil {
			result.OK = false
			result.Message = err.Error()
		} else {
			result.Message = fmt.Sprintf("процесс #%d уничтожен", d.ProcessID)
		}

	case DirectiveFinishAndQuit:
		sys.FinishAndQuit = true
		sys.AutoLoadEnabled = false
		sys.Paused = false
		sys.Regulator.ResumeAll()
		result.Message = "загруженные задания выполняются, новые не загружаются"

	case DirectiveSetMemory:
		if err := sys.Memory.SetTotalSize(d.MemorySize); err != nil {
			result.OK = false
			result.Message = err.Error()
		} else {
			sys.Config.MemorySize = d.MemorySize
			result.Message = fmt.Sprintf("размер памяти: %d единиц", d.MemorySize)
		}

	default:
		result.OK = false
		result.Message = "неизвестная директива"
	}

	sys.LastDirective = result
	sys.notify()
}

// ----------------------------------------------------------------------------------------
// validateTask проверяет параметры задания, заданные оператором.
func validateTask(task process.Task) error {
	if task.Size <= 0 {
		return fmt.Errorf("размер задания должен быть больше 0")
	}
	if task.TotalCommands <= 0 {
		return fmt.Errorf("число команд должно быть больше 0")
	}
	if task.IOPercent < 0 || task.IOPercent > 100 {
		return fmt.Errorf("процент команд ВВ должен быть в пределах 0..100")
	}
	if task.BasePriority <= 0 {
		return fmt.Errorf("базовый приоритет должен быть больше 0")
	}
	return nil
}

// ----------------------------------------------------------------------------------------
// findProcess ищет резидентный процесс по идентификатору.
func (sys *System) findProcess(id int) *process.PSW {
	for _, p := range sys.Regulator.Table {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// ----------------------------------------------------------------------------------------
// suspendProcess приостанавливает готовый или активный процесс.
func (sys *System) suspendProcess(id int) error {
	p := sys.findProcess(id)
	if p == nil {
		return fmt.Errorf("процесс #%d не найден", id)
	}
	if p.State != process.StateReady && p.State != process.StateActive {
		return fmt.Errorf("процесс #%d нельзя приостановить в состоянии %s",
			id, process.StateName(p.State))
	}
	if !sys.Regulator.Process(p, regulator.EventSuspend) {
		return fmt.Errorf("не удалось приостановить процесс #%d", id)
	}
	if sys.CPU.ActiveProcess == p {
		sys.CPU.Release()
	}
	return nil
}

// ----------------------------------------------------------------------------------------
// resumeProcess возвращает приостановленный процесс в очередь готовности.
func (sys *System) resumeProcess(id int) error {
	p := sys.findProcess(id)
	if p == nil {
		return fmt.Errorf("процесс #%d не найден", id)
	}
	if p.State != process.StateSuspended {
		return fmt.Errorf("процесс #%d не приостановлен", id)
	}
	if !sys.Regulator.Process(p, regulator.EventResume) {
		return fmt.Errorf("не удалось возобновить процесс #%d", id)
	}
	return nil
}

// ----------------------------------------------------------------------------------------
// destroyProcess уничтожает процесс, освобождая ЦПр, ВВ и память.
func (sys *System) destroyProcess(id int) error {
	p := sys.findProcess(id)
	if p == nil {
		return fmt.Errorf("процесс #%d не найден", id)
	}

	if sys.CPU.ActiveProcess == p {
		sys.CPU.Release()
	}
	sys.IO.Cancel(p)
	sys.Interrupts.Cancel(p)
	if !sys.Regulator.Destroy(p) {
		return fmt.Errorf("не удалось уничтожить процесс #%d", id)
	}
	return nil
}

// ----------------------------------------------------------------------------------------
// checkFinish завершает моделирование, когда все загруженные задания
// выполнены или уничтожены.
func (sys *System) checkFinish() {
	if sys.FinishAndQuit && len(sys.Regulator.Table) == 0 {
		sys.Quit = true
	}
}
