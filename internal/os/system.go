package os

import (
	"math/rand/v2"
	"time"

	"os_model/internal/cpu"
	"os_model/internal/interrupt"
	"os_model/internal/io"
	"os_model/internal/memory"
	"os_model/internal/process"
	"os_model/internal/regulator"
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
	Memory     *memory.Allocator     // супервизор памяти
	Regulator  *regulator.Regulator  // регулировщик (состояния процессов, планировщик)
	Interrupts *interrupt.Supervisor // супервизор прерываний
	CPU        *cpu.CPU              // центральный процессор
	IO         *io.Processors        // подсистема ввода-вывода (процессоры ВВ)

	Config Config // параметры конфигурации

	Time int // глобальное время модели в тактах

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
	// Значения по умолчанию для незаполненной (или невалидной) конфигурации.
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
	sys.Memory = &memory.Allocator{TotalSize: sys.Config.MemorySize}
	sys.Regulator = &regulator.Regulator{
		Planner:      &regulator.Planner{},
		Memory:       sys.Memory,
		MaxProcesses: MaxProcesses,
	}
	sys.Interrupts = &interrupt.Supervisor{}
	sys.IO = &io.Processors{
		Regulator:  sys.Regulator,
		Interrupts: sys.Interrupts,
	}
	sys.CPU = &cpu.CPU{
		IO:            sys.IO,
		Processes:     sys.Regulator,
		IOMaxDuration: sys.Config.IOMaxDuration,
	}

	if sys.Directives == nil {
		sys.Directives = make(chan Directive, MaxProcesses)
	}

	sys.Time = 0
	sys.nextTaskID = 0
	sys.Paused = false
	sys.Quit = false
	sys.setSpeed(1000.0 / float64(sys.Config.TickDurationMs))
}

// ----------------------------------------------------------------------------------------
// GenerateAndLoadTask — системный процесс загрузки задания: генерирует
// параметры нового задания и пытается загрузить его, если есть ресурсы
// (свободная запись в таблице слов состояний процессов и достаточная
// память). При успехе регулировщик создаёт процесс в состоянии «Готов».
// Возвращает true, если задание загружено.
func (sys *System) GenerateAndLoadTask() bool {
	// Проверка места в таблице слов состояний процессов.
	if !sys.Regulator.FreeSlot() {
		return false
	}

	// Генерируем параметры задания и проверяем наличие доступной памяти
	// (новое задание генерируется только при наличии свободной памяти).
	task := sys.generateTask()
	if !sys.Memory.CheckAvailable(task.Size) {
		return false
	}

	// Присваиваем идентификатор и выделяем память для задания.
	sys.nextTaskID++
	task.ID = sys.nextTaskID
	if err := sys.Memory.Allocate(task); err != nil {
		return false
	}

	// Порождаем процесс; состояние «Готов» устанавливает регулировщик.
	psw := &process.PSW{
		ID:              task.ID,
		Task:            &task,
		PC:              0,
		DynamicPriority: task.BasePriority,
	}

	return sys.Regulator.Process(psw, regulator.EventLoad)
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
// RunLoop — главный цикл модели. В каждом такте выполняется подпрограмма
// DoOneTick (алгоритм одного такта моделирования), затем обрабатываются
// директивы оператора и обновляется индикация.
func (sys *System) RunLoop() {
	// Цикл загрузки заданий для начала моделирования: загружается столько
	// заданий, сколько помещается в память и в таблицу процессов.
	for sys.GenerateAndLoadTask() {
	}

	for !sys.Quit {
		if !sys.Paused {
			sys.DoOneTick()
		} else {
			// На паузе изменения (директивы, скорость) тоже нужно показывать.
			sys.notify()
		}

		sys.HandleDirectives()

		if sys.Quit {
			sys.notify() // завершающий снимок состояния
			break
		}

		sys.sleep()
	}
}

// ----------------------------------------------------------------------------------------
// DoOneTick — выполнение одного такта моделирования в соответствии
// с алгоритмом одного такта модели ОС:
//
//	А  — инициализация цикла: выбрать процесс, сделать его активным
//	     и восстановить слово состояния процесса;
//	Б1 — моделирование ЦПр: выполнить очередную команду активного процесса;
//	Б2 — моделирование процессоров ВВ: один такт операций ввода-вывода
//	     и обработка прерываний от процессоров ВВ;
//	Б3 — моделирование прерываний по времени (только для алгоритма RR;
//	     вариант 11 использует относительные приоритеты без вытеснения,
//	     поэтому шаг отсутствует);
//	Б4 — проверка достигнутого состояния системы и вызов планировщика;
//	Б5 — обновление баз данных модели ОС;
//	Б6 — отображение изменений в состоянии системы.
func (sys *System) DoOneTick() {
	sys.Time++

	// А. Инициализация цикла.
	if sys.CPU.State == cpu.StateIdle {
		sys.dispatchProcess()
	}

	// Б1. Моделирование центрального процессора.
	sys.CPU.Tick()

	// Б2. Моделирование процессоров ввода-вывода и обработка прерываний.
	sys.IO.TickIO(sys.Time)
	sys.HandleInterrupts()

	// Б3. Прерывания по времени не моделируются (см. комментарий выше).

	// Б4. Проверка достигнутого состояния системы.
	sys.CheckSystemState()

	// Б5. Обновление баз данных модели ОС.
	sys.Regulator.Planner.UpdatePriorities()

	// Б6. Отображение изменений в состоянии системы.
	sys.notify()
}

// ----------------------------------------------------------------------------------------
// dispatchProcess — выбор планировщиком следующего процесса и передача его
// на выполнение: регулировщик переводит процесс в состояние «Активен»,
// процессор восстанавливает слово состояния процесса.
func (sys *System) dispatchProcess() bool {
	p := sys.Regulator.Planner.SelectNext()
	if p == nil {
		return false
	}

	if !sys.Regulator.Process(p, regulator.EventActivate) {
		return false
	}

	sys.CPU.ContextSwitch(p)
	return true
}

// ----------------------------------------------------------------------------------------
// CheckSystemState — проверка достигнутого состояния системы (шаг Б4).
//
// Для всех процессоров: если резидентный процесс перестал быть активным
// (заблокирован или завершён), процессор освобождается; если процессор
// находится в состоянии «Ожидание», а список готовности не пуст, вызывается
// планировщик для выбора следующего процесса. Если готовых процессов нет,
// процессор переходит в состояние «Ожидание», а ОС порождает системный
// процесс загрузки нового задания, если есть ресурсы.
func (sys *System) CheckSystemState() {
	// Резидентный процесс мог перестать быть активным: его состояние
	// изменил регулировщик по событию ВВ или завершения.
	if p := sys.CPU.ActiveProcess; p != nil && p.State != process.StateActive {
		sys.CPU.Release()
	}

	if sys.CPU.State != cpu.StateIdle {
		return
	}

	// Список готовности не пуст — планировщик выбирает следующий процесс.
	if len(sys.Regulator.Planner.ReadyQueue) > 0 {
		sys.dispatchProcess()
		return
	}

	// Готовых процессов нет: ЦПр «Ожидает», ОС может загрузить одно новое
	// задание (место в таблице PSW и достаточная память проверяются
	// в GenerateAndLoadTask).
	if sys.GenerateAndLoadTask() {
		sys.dispatchProcess()
	}
}

// ----------------------------------------------------------------------------------------
// HandleInterrupts — супервизор прерываний: обрабатывает накопленные сигналы.
// Обработка сводится к вызову регулировщика, который изменяет состояния
// процессов (прерывание от процессора ВВ переводит процесс в «Готов»).
func (sys *System) HandleInterrupts() {
	for _, signal := range sys.Interrupts.ExtractAll() {
		switch signal.Type {
		case interrupt.IOComplete:
			sys.Regulator.Process(signal.Process, regulator.EventIOComplete)
		}
	}
}

// ----------------------------------------------------------------------------------------
// generateTask генерирует параметры нового задания в заданных диапазонах
// (без идентификатора: идентификатор присваивается загруженному заданию).
func (sys *System) generateTask() process.Task {
	return process.Task{
		Size:          randomInRange(TaskSizeMin, TaskSizeMax),
		TotalCommands: randomInRange(TaskCommandsMin, TaskCommandsMax),
		IOPercent:     randomInRange(TaskIOPercentMin, TaskIOPercentMax),
		BasePriority:  randomInRange(TaskBasePriorityMin, TaskBasePriorityMax),
	}
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
