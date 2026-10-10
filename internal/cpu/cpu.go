// Package cpu моделирует центральный процессор и АЛУ.
//
// Архитектура фон-неймановская: команды и данные хранятся в общей памяти,
// команда читается по аппаратному счётчику команд (PC). Процессор работает
// только с одним текущим активным процессом.
//
// Процессор не изменяет состояния процессов: при блокировке или завершении
// процесса он лишь сообщает о событии соответствующим подпрограммам
// (инициализация ВВ, завершение процесса) — состояния меняет регулировщик.
package cpu

import (
	"math/rand/v2"

	"os_model/internal/process"
)

// ----------------------------------------------------------------------------------------
// State — состояние процессора.
type State int

const (
	StateIdle    State = iota // ожидание: активного процесса нет
	StateWorking              // работа: выполняется команда активного процесса
)

// ----------------------------------------------------------------------------------------
// CommandType — тип команды после декодирования.
type CommandType int

const (
	CommandCompute   CommandType = iota // вычислительная команда (АЛУ)
	CommandIO                           // команда ввода-вывода
	CommandTerminate                    // команда завершения процесса
)

// ----------------------------------------------------------------------------------------
// Operation — код операции команды.
type Operation int

const (
	// вычислительные операции (выполняются в АЛУ)
	OpAdd Operation = iota
	OpSub
	OpMul
	OpDiv

	// системные
	OpIO        // обращение к устройству ввода-вывода
	OpTerminate // завершение задания
)

// ----------------------------------------------------------------------------------------
// Command — команда, обрабатываемая процессором.
type Command struct {
	Type     CommandType // тип команды, распознанный при декодировании
	Code     Operation   // код операции
	Address1 int         // математический адрес первого операнда
	Address2 int         // математический адрес второго операнда (приёмник результата)
	Duration int         // длительность операции ввода-вывода, тактов
}

// ----------------------------------------------------------------------------------------
// IOInitiator — часть подсистемы ввода-вывода, используемая процессором
// для инициализации операции ввода-вывода.
type IOInitiator interface {
	// InitIO выделяет процессор ВВ и сохраняет слово состояния процесса;
	// процессор лишь сообщает о событии, состояние меняет регулировщик.
	InitIO(p *process.PSW, duration int)
}

// ----------------------------------------------------------------------------------------
// ProcessFinisher — часть ОС, используемая процессором при выполнении команды
// завершения задания (подпрограмма завершения процесса регулировщика).
type ProcessFinisher interface {
	Terminate(p *process.PSW)
}

// ----------------------------------------------------------------------------------------
// OperandMemorySize — размер моделируемой аппаратной памяти под операнды.
const OperandMemorySize = 256

// CPU — центральный процессор модели.
type CPU struct {
	ActiveProcess  *process.PSW // указатель на PSW текущего активного процесса
	State          State        // состояние процессора
	CurrentCommand Command      // команда, обрабатываемая в текущем такте

	// Смежные подсистемы; проставляются ядром при инициализации.
	IO        IOInitiator     // подсистема ввода-вывода
	Processes ProcessFinisher // завершение процесса (регулировщик)

	// IOMaxDuration — максимальная длительность операции ввода-вывода, тактов.
	IOMaxDuration int

	// OperandMemory — аппаратная память, в которой хранятся операнды
	// (инициализируется лениво).
	OperandMemory []int
}

// ----------------------------------------------------------------------------------------
// ContextSwitch загружает PSW нового процесса и переводит процессор
// в состояние «Работа», восстанавливая слово состояния процесса.
//
// В модели всё состояние процесса хранится в его PSW (включая PC),
// поэтому отдельное сохранение не требуется: при следующей загрузке
// поля восстанавливаются из PSW. Состояние «Активен» процессу
// устанавливает регулировщик до вызова.
func (cpu *CPU) ContextSwitch(p *process.PSW) {
	cpu.ActiveProcess = p
	if p == nil {
		cpu.State = StateIdle
		return
	}

	cpu.State = StateWorking
}

// ----------------------------------------------------------------------------------------
// Release освобождает процессор: активный процесс блокирован или завершён,
// процессор переходит в состояние «Ожидание».
func (cpu *CPU) Release() {
	cpu.ActiveProcess = nil
	cpu.State = StateIdle
}

// ----------------------------------------------------------------------------------------
// Tick выполняет один такт процессора: выборку, декодирование и выполнение
// одной команды активного процесса. Длина команды в модели равна 1,
// поэтому PC увеличивается сразу после выборки.
func (cpu *CPU) Tick() {
	if cpu.State != StateWorking || cpu.ActiveProcess == nil {
		return
	}

	// 1. Чтение команды из памяти по адресу на аппаратном счётчике команд.
	cpu.CurrentCommand = cpu.Fetch()

	// 2. Увеличение аппаратного счётчика команд на длину команды (равна 1).
	cpu.ActiveProcess.PC++

	// 3. Распознавание команды.
	cpu.CurrentCommand.Type = cpu.Decode(cpu.CurrentCommand)

	// 4-6. Чтение операндов, выполнение операции, запись результата.
	cpu.Execute(cpu.CurrentCommand)
}

// ----------------------------------------------------------------------------------------
// Fetch читает команду из памяти по аппаратному счётчику команд.
//
// В модели команды не хранятся, а генерируются случайным образом:
// пока PC меньше общего числа команд задания, с вероятностью IOPercent
// генерируется команда ввода-вывода, иначе — вычислительная; когда PC
// достигает TotalCommands, генерируется команда завершения.
func (cpu *CPU) Fetch() Command {
	ap := cpu.ActiveProcess

	if ap == nil || ap.Task == nil {
		return Command{Code: OpTerminate}
	}

	// Завершающая команда, процесс полностью выполнился
	if ap.PC >= ap.Task.TotalCommands {
		return Command{Code: OpTerminate}
	}

	// Команда ввода-вывода
	if rand.IntN(100) < ap.Task.IOPercent {
		return Command{Code: OpIO, Duration: cpu.newIODuration()}
	}

	// Вычислительная команда: код операции и математические адреса операндов.
	computeOps := [...]Operation{OpAdd, OpSub, OpMul, OpDiv}
	return Command{
		Code:     computeOps[rand.IntN(len(computeOps))],
		Address1: rand.IntN(OperandMemorySize),
		Address2: rand.IntN(OperandMemorySize),
	}
}

// ----------------------------------------------------------------------------------------
// Decode распознаёт команду: вычислительная, ввод-вывод или завершение.
func (cpu *CPU) Decode(cmd Command) CommandType {
	switch cmd.Code {
	case OpIO:
		return CommandIO
	case OpTerminate:
		return CommandTerminate
	default:
		return CommandCompute
	}
}

// ----------------------------------------------------------------------------------------
// Execute выполняет операцию в АЛУ.
//
// Вычислительная команда выполняется за один такт: операнды читаются
// из аппаратной памяти, над ними выполняется двухместная операция,
// результат записывается обратно (во второй операнд). Команда ввода-вывода
// инициирует операцию ВВ (процесс переводится в состояние «Инициализация IO»),
// после чего процессор освобождается. Команда завершения инициирует
// выгрузку процесса.
func (cpu *CPU) Execute(cmd Command) {
	ap := cpu.ActiveProcess
	if ap == nil {
		return
	}

	switch cmd.Type {
	case CommandCompute:
		operand1 := cpu.ReadOperand(cmd.Address1)
		operand2 := cpu.ReadOperand(cmd.Address2)
		cpu.WriteOperand(cmd.Address2, applyOperation(cmd.Code, operand1, operand2))

	case CommandIO:
		// Инициализация операции ввода-вывода: выделяется процессор ВВ,
		// слово состояния процесса сохраняется, процесс блокируется.
		cpu.IO.InitIO(ap, cmd.Duration)
		cpu.Release()

	case CommandTerminate:
		// Завершение задания: процесс выгружается регулировщиком
		// (освобождение памяти, удаление записи из таблицы PSW).
		cpu.Processes.Terminate(ap)
		cpu.Release()
	}
}

// ----------------------------------------------------------------------------------------
// ReadOperand читает операнд из аппаратной памяти по математическому адресу.
func (cpu *CPU) ReadOperand(addr int) int {
	cpu.ensureOperandMemory()
	if addr < 0 || addr >= len(cpu.OperandMemory) {
		return 0
	}
	return cpu.OperandMemory[addr]
}

// ----------------------------------------------------------------------------------------
// WriteOperand записывает результат операции в аппаратную память.
func (cpu *CPU) WriteOperand(addr int, value int) {
	cpu.ensureOperandMemory()
	if addr < 0 || addr >= len(cpu.OperandMemory) {
		return
	}
	cpu.OperandMemory[addr] = value
}

// ----------------------------------------------------------------------------------------
// ensureOperandMemory лениво (нулями) инициализирует аппаратную память под операнды.
func (cpu *CPU) ensureOperandMemory() {
	if cpu.OperandMemory == nil {
		cpu.OperandMemory = make([]int, OperandMemorySize)
	}
}

// ----------------------------------------------------------------------------------------
// newIODuration генерирует длительность операции ввода-вывода
// в диапазоне 1..IOMaxDuration тактов.
func (cpu *CPU) newIODuration() int {
	maxDuration := cpu.IOMaxDuration
	if maxDuration < 1 {
		maxDuration = 1
	}
	return 1 + rand.IntN(maxDuration)
}

// ----------------------------------------------------------------------------------------
// applyOperation моделирует работу АЛУ, выполняя двухместную операцию.
func applyOperation(code Operation, operand1, operand2 int) int {
	switch code {
	case OpAdd:
		return operand1 + operand2
	case OpSub:
		return operand1 - operand2
	case OpMul:
		return operand1 * operand2
	case OpDiv:
		if operand2 == 0 {
			return 0
		}
		return operand1 / operand2
	default:
		return operand2
	}
}

// ----------------------------------------------------------------------------------------
