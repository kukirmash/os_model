package os

import (
	"os_model/internal/interrupt"
	"os_model/internal/io"
	"os_model/internal/process"
)

// ----------------------------------------------------------------------------------------
// ProcessInfo — данные одного процесса для индикации.
type ProcessInfo struct {
	ID              int `json:"id"`
	Size            int `json:"size"`
	PC              int `json:"pc"`
	TotalCommands   int `json:"totalCommands"`
	IOPercent       int `json:"ioPercent"`
	State           int `json:"state"`
	DynamicPriority int `json:"dynamicPriority"`
	BasePriority    int `json:"basePriority"`
}

// ----------------------------------------------------------------------------------------
// OperationInfo — данные одной операции ввода-вывода (занятого процессора ВВ).
type OperationInfo struct {
	ProcessorID int `json:"processorId"` // номер процессора ВВ
	ProcessID   int `json:"processId"`   // обслуживаемый процесс
	State       int `json:"state"`       // «Инициализация IO» или «Блокирован»
	TicksLeft   int `json:"ticksLeft"`   // осталось тактов
	TotalTicks  int `json:"totalTicks"`  // полная длительность операции
}

// ----------------------------------------------------------------------------------------
// InterruptInfo — данные сигнала прерывания для индикации.
type InterruptInfo struct {
	Type      int `json:"type"`      // тип прерывания
	Time      int `json:"time"`      // такт модели
	ProcessID int `json:"processId"` // процесс, которого касается прерывание
	SourceID  int `json:"sourceId"`  // номер источника (процессора ВВ)
}

// ----------------------------------------------------------------------------------------
// TaskParams — диапазоны генерации заданий (входные параметры модели).
type TaskParams struct {
	SizeMin      int `json:"sizeMin"`
	SizeMax      int `json:"sizeMax"`
	CommandsMin  int `json:"commandsMin"`
	CommandsMax  int `json:"commandsMax"`
	IOPercentMin int `json:"ioPercentMin"`
	IOPercentMax int `json:"ioPercentMax"`
	PriorityMin  int `json:"priorityMin"`
	PriorityMax  int `json:"priorityMax"`
}

// ----------------------------------------------------------------------------------------
// Snapshot — снимок состояния модели для индикации в интерфейсе.
type Snapshot struct {
	Time      int     `json:"time"`
	Speed     float64 `json:"speed"`
	Paused    bool    `json:"paused"`
	Quit      bool    `json:"quit"`
	Generated int     `json:"generated"`
	Completed int     `json:"completed"`

	MemoryTotal int `json:"memoryTotal"`
	MemoryUsed  int `json:"memoryUsed"`

	CPUState        int `json:"cpuState"`        // 0 — ожидание, 1 — работа
	ActiveID        int `json:"activeId"`        // 0 — нет активного процесса
	CommandType     int `json:"commandType"`     // 0 — вычислительная, 1 — IO, 2 — завершение
	CommandCode     int `json:"commandCode"`     // код вычислительной операции
	CommandAddr1    int `json:"commandAddr1"`    // адрес первого операнда
	CommandAddr2    int `json:"commandAddr2"`    // адрес второго операнда
	CommandDuration int `json:"commandDuration"` // длительность операции IO, тактов

	// Прерывания: счётчик обработанных сигналов и журнал последних.
	InterruptsHandled int             `json:"interruptsHandled"`
	InterruptLog      []InterruptInfo `json:"interruptLog"`

	MaxProcesses int `json:"maxProcesses"`

	Ready      []ProcessInfo   `json:"ready"`      // очередь готовности
	Operations []OperationInfo `json:"operations"` // занятые процессоры ВВ
	Processes  []ProcessInfo   `json:"processes"`  // таблица PSW

	TaskParams TaskParams `json:"taskParams"`
}

// ----------------------------------------------------------------------------------------
// Snapshot формирует снимок текущего состояния модели.
func (sys *System) Snapshot() Snapshot {
	snap := Snapshot{
		Time:              sys.Time,
		Speed:             sys.Speed,
		Paused:            sys.Paused,
		Quit:              sys.Quit,
		Generated:         sys.nextTaskID,
		Completed:         sys.Regulator.Completed,
		MemoryTotal:       sys.Memory.TotalSize,
		MemoryUsed:        sys.Memory.UsedSize,
		CPUState:          int(sys.CPU.State),
		InterruptsHandled: sys.Interrupts.Handled,
		MaxProcesses:      MaxProcesses,
		TaskParams: TaskParams{
			SizeMin:      TaskSizeMin,
			SizeMax:      TaskSizeMax,
			CommandsMin:  TaskCommandsMin,
			CommandsMax:  TaskCommandsMax,
			IOPercentMin: TaskIOPercentMin,
			IOPercentMax: TaskIOPercentMax,
			PriorityMin:  TaskBasePriorityMin,
			PriorityMax:  TaskBasePriorityMax,
		},
	}

	// Команда актуальна только при активном процессе.
	if p := sys.CPU.ActiveProcess; p != nil {
		snap.ActiveID = p.ID
		snap.CommandType = int(sys.CPU.CurrentCommand.Type)
		snap.CommandCode = int(sys.CPU.CurrentCommand.Code)
		snap.CommandAddr1 = sys.CPU.CurrentCommand.Address1
		snap.CommandAddr2 = sys.CPU.CurrentCommand.Address2
		snap.CommandDuration = sys.CPU.CurrentCommand.Duration
	}

	snap.Ready = processInfos(sys.Regulator.Planner.ReadyQueue)
	snap.Operations = operationInfos(sys.IO.Operations)
	snap.Processes = processInfos(sys.Regulator.Table)
	snap.InterruptLog = interruptInfos(sys.Interrupts.Log)
	return snap
}

// ----------------------------------------------------------------------------------------
// processInfos преобразует список PSW в данные для индикации.
func processInfos(psws []*process.PSW) []ProcessInfo {
	infos := make([]ProcessInfo, 0, len(psws))
	for _, p := range psws {
		infos = append(infos, ProcessInfo{
			ID:              p.ID,
			Size:            p.Task.Size,
			PC:              p.PC,
			TotalCommands:   p.Task.TotalCommands,
			IOPercent:       p.Task.IOPercent,
			State:           int(p.State),
			DynamicPriority: p.DynamicPriority,
			BasePriority:    p.Task.BasePriority,
		})
	}
	return infos
}

// ----------------------------------------------------------------------------------------
// operationInfos преобразует операции ввода-вывода в данные для индикации.
func operationInfos(operations []*io.Operation) []OperationInfo {
	infos := make([]OperationInfo, 0, len(operations))
	for _, op := range operations {
		infos = append(infos, OperationInfo{
			ProcessorID: op.ProcessorID,
			ProcessID:   op.Process.ID,
			State:       int(op.Process.State),
			TicksLeft:   op.TicksLeft,
			TotalTicks:  op.TotalTicks,
		})
	}
	return infos
}

// ----------------------------------------------------------------------------------------
// interruptInfos преобразует журнал прерываний в данные для индикации.
func interruptInfos(signals []interrupt.Signal) []InterruptInfo {
	infos := make([]InterruptInfo, 0, len(signals))
	for _, s := range signals {
		info := InterruptInfo{
			Type:     int(s.Type),
			Time:     s.Time,
			SourceID: s.SourceID,
		}
		if s.Process != nil {
			info.ProcessID = s.Process.ID
		}
		infos = append(infos, info)
	}
	return infos
}
