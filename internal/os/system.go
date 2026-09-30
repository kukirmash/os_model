package os

import (
	"math/rand/v2"
	"time"

	"os_model/internal/cpu"
	"os_model/internal/io"
	"os_model/internal/memory"
	"os_model/internal/process"
	"os_model/internal/scheduler"
)

// ----------------------------------------------------------------------------------------
// Параметры модели по умолчанию (если соответствующие поля Config не заполнены).
const (
	DefaultMemorySize     = 8192 // общий объём моделируемой памяти
	DefaultTickDurationMs = 1    // начальная длительность такта, мс
	DefaultIOMaxDuration  = 10   // максимальная длительность ввода-вывода, тактов
)

// ----------------------------------------------------------------------------------------
// Параметры генерации заданий (значения по умолчанию на этапе компиляции;
// на этапе 8 будут читаться из файла параметров).
const (
	TaskSizeMin = 100 // размер задания, единиц памяти
	TaskSizeMax = 500

	TaskCommandsMin = 10 // число команд задания
	TaskCommandsMax = 20

	TaskIOPercentMin = 0 // процент команд ввода-вывода
	TaskIOPercentMax = 50

	TaskBasePriorityMin = 1 // базовый приоритет
	TaskBasePriorityMax = 10
)

// ----------------------------------------------------------------------------------------
// MaxProcesses — число записей в таблице слов состояний процессов.
const MaxProcesses = 16

// ----------------------------------------------------------------------------------------
// Границы скорости моделирования: от 0,1 до 1000 тактов в секунду.
const (
	minSpeed         = 0.1
	maxSpeed         = 1000.0
	speedStepPercent = 10
)

// ----------------------------------------------------------------------------------------
// Directive — управляющая директива оператора.
type Directive int

const (
	DirectivePause     Directive = iota // приостановить моделирование
	DirectiveResume                     // продолжить моделирование
	DirectiveSpeedUp                    // увеличить скорость
	DirectiveSpeedDown                  // уменьшить скорость
	DirectiveQuit                       // завершить моделирование
)

// ----------------------------------------------------------------------------------------
// System — ядро модели: связывает все компоненты и содержит главный цикл.
type System struct {
	Memory  *memory.MemoryManager // менеджер памяти
	Planner *scheduler.Planner    // планировщик
	CPU     *cpu.CPU              // центральный процессор
	IO      *io.IOManager         // подсистема ввода-вывода

	Config Config // параметры конфигурации

	Time      int            // глобальное время модели в тактах
	Processes []*process.PSW // таблица PSW всех загруженных процессов
	Completed int            // число выполненных (завершённых) заданий

	// Управление работой модели.
	Directives chan Directive // директивы оператора
	Speed      float64        // текущая скорость модели, тактов в секунду
	Paused     bool           // моделирование приостановлено
	Quit       bool           // получена директива завершения

	// OnTick вызывается каждый такт для индикации состояния модели
	// (например, для отправки снимка в интерфейс). Может быть nil.
	OnTick func(s *System)

	nextTaskID int // счётчик идентификаторов заданий
}

// ----------------------------------------------------------------------------------------
// Init инициализирует структуры модели по значениям полей Config
// до запуска главного цикла.
func (sys *System) Init() {
	// Значения по умолчанию для незаполненной(или невалидной) конфигурации.
	if sys.Config.MemorySize <= 0 {
		sys.Config.MemorySize = DefaultMemorySize
	}
	if sys.Config.TickDurationMs <= 0 {
		sys.Config.TickDurationMs = DefaultTickDurationMs
	}
	if sys.Config.IOMaxDuration <= 0 {
		sys.Config.IOMaxDuration = DefaultIOMaxDuration
	}

	// Создание подсистем и их связывание.
	sys.Memory = &memory.MemoryManager{TotalSize: sys.Config.MemorySize}
	sys.Planner = &scheduler.Planner{}
	sys.IO = &io.IOManager{Planner: sys.Planner}
	sys.CPU = &cpu.CPU{
		IO:            sys.IO,
		Memory:        sys.Memory,
		IOMaxDuration: sys.Config.IOMaxDuration,
	}

	if sys.Directives == nil {
		sys.Directives = make(chan Directive, MaxProcesses)
	}

	sys.Time = 0
	sys.Processes = nil
	sys.Completed = 0
	sys.nextTaskID = 0
	sys.Paused = false
	sys.Quit = false
	sys.setSpeed(1000.0 / float64(sys.Config.TickDurationMs))
}

// ----------------------------------------------------------------------------------------
// GenerateAndLoadTask генерирует параметры нового задания и пытается
// загрузить его через MemoryManager.Allocate. При успехе создаётся PSW
// в состоянии Ready, который регистрируется в таблице процессов
// и передаётся планировщику. Возвращает true, если задание загружено.
func (sys *System) GenerateAndLoadTask() bool {
	// Проверка места в таблице слов состояний процессов.
	if len(sys.Processes) >= MaxProcesses {
		return false
	}

	// Генерируем задание
	task := sys.generateTask()

	// Выделяем память для задания
	if err := sys.Memory.Allocate(task); err != nil {
		return false
	}

	// Порождаем процесс
	psw := &process.PSW{
		ID:              task.ID,
		Task:            &task,
		PC:              0,
		State:           process.StateReady,
		DynamicPriority: task.BasePriority,
		IOTicksLeft:     0,
	}

	sys.Processes = append(sys.Processes, psw)
	sys.Planner.AddProcess(psw)
	return true
}

// ----------------------------------------------------------------------------------------
// HandleDirectives обрабатывает директивы оператора:
// пауза, изменение скорости моделирования, завершение работы.
func (sys *System) HandleDirectives() {
	for {
		select {
		case d := <-sys.Directives:
			sys.applyDirective(d)
		default:
			return
		}
	}
}

// ----------------------------------------------------------------------------------------
// RunLoop — главный цикл модели. В каждом такте:
//
//  1. IOManager.TickIO()         — обслуживание операций ввода-вывода;
//  2. Planner.UpdatePriorities() — пересчёт динамических приоритетов;
//  3. если готовых процессов нет, ОС загружает новое задание; свободному
//     ЦПр передаётся процесс, выбранный Planner.GetNextProcess();
//  4. CPU.Tick()                 — выполнение одного такта активного процесса;
//  5. опрос интерфейса/индикации и обработка директив оператора.
func (sys *System) RunLoop() {
	// Цикл загрузки заданий для начала моделирования: загружается столько
	// заданий, сколько помещается в память и в таблицу процессов.
	for sys.GenerateAndLoadTask() {
	}

	for !sys.Quit {
		if !sys.Paused {
			sys.IO.TickIO()
			sys.Planner.UpdatePriorities()

			// Очередь готовности пуста — ОС загружает новое задание.
			if sys.CPU.State == cpu.StateIdle && len(sys.Planner.ReadyQueue) == 0 {
				sys.GenerateAndLoadTask()
			}

			// Диспетчеризация: свободному ЦПр передаётся лучший процесс.
			if sys.CPU.State == cpu.StateIdle {
				if p := sys.Planner.GetNextProcess(); p != nil {
					sys.CPU.ContextSwitch(p)
				}
			}

			sys.CPU.Tick()
			sys.removeFinished()
			sys.Time++
		}

		// Обработка директив оператора и индикация состояния модели.
		sys.HandleDirectives()
		sys.notify()

		if sys.Quit {
			break
		}

		sys.sleep()
	}
}

// ----------------------------------------------------------------------------------------
// generateTask генерирует параметры нового задания в заданных диапазонах.
func (sys *System) generateTask() process.Task {
	sys.nextTaskID++
	return process.Task{
		ID:            sys.nextTaskID,
		Size:          randomInRange(TaskSizeMin, TaskSizeMax),
		TotalCommands: randomInRange(TaskCommandsMin, TaskCommandsMax),
		IOPercent:     randomInRange(TaskIOPercentMin, TaskIOPercentMax),
		BasePriority:  randomInRange(TaskBasePriorityMin, TaskBasePriorityMax),
	}
}

// ----------------------------------------------------------------------------------------
// removeFinished удаляет из таблицы завершённые процессы (StateNone),
// освобождая записи для новых заданий.
func (sys *System) removeFinished() {
	alive := sys.Processes[:0]
	for _, p := range sys.Processes {
		if p.State != process.StateNone {
			alive = append(alive, p)
			continue
		}
		sys.Completed++
	}
	sys.Processes = alive
}

// ----------------------------------------------------------------------------------------
// applyDirective выполняет одну директиву оператора.
func (sys *System) applyDirective(d Directive) {
	switch d {
	case DirectivePause:
		sys.Paused = true
	case DirectiveResume:
		sys.Paused = false
	case DirectiveSpeedUp:
		sys.changeSpeed(speedStepPercent)
	case DirectiveSpeedDown:
		sys.changeSpeed(-speedStepPercent)
	case DirectiveQuit:
		sys.Quit = true
	}
}

// ----------------------------------------------------------------------------------------
// changeSpeed изменяет скорость модели на заданное число процентов.
func (sys *System) changeSpeed(percent int) {
	sys.setSpeed(sys.Speed * float64(100+percent) / 100)
}

// ----------------------------------------------------------------------------------------
// setSpeed устанавливает скорость модели (тактов в секунду)
// в допустимых пределах.
func (sys *System) setSpeed(speed float64) {
	if speed < minSpeed {
		speed = minSpeed
	}
	if speed > maxSpeed {
		speed = maxSpeed
	}
	sys.Speed = speed
}

// ----------------------------------------------------------------------------------------
// notify вызывает обработчик индикации, если он установлен.
func (sys *System) notify() {
	if sys.OnTick != nil {
		sys.OnTick(sys)
	}
}

// ----------------------------------------------------------------------------------------
// sleep выполняет программную задержку в соответствии со скоростью модели.
func (sys *System) sleep() {
	if sys.Speed <= 0 {
		return
	}
	time.Sleep(time.Duration(float64(time.Second) / sys.Speed))
}

// ----------------------------------------------------------------------------------------
// randomInRange возвращает случайное число из диапазона [min, max].
func randomInRange(min, max int) int {
	if max <= min {
		return min
	}
	return min + rand.IntN(max-min+1)
}

// ----------------------------------------------------------------------------------------
