package os

import "os_model/internal/process"

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
	IOTicksLeft     int `json:"ioTicksLeft"`
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

	MaxProcesses int `json:"maxProcesses"`

	Ready     []ProcessInfo `json:"ready"`
	Blocked   []ProcessInfo `json:"blocked"`
	Processes []ProcessInfo `json:"processes"`

	TaskParams TaskParams `json:"taskParams"`
}

// ----------------------------------------------------------------------------------------
// Snapshot формирует снимок текущего состояния модели.
func (sys *System) Snapshot() Snapshot {
	snap := Snapshot{
		Time:         sys.Time,
		Speed:        sys.Speed,
		Paused:       sys.Paused,
		Quit:         sys.Quit,
		Generated:    sys.nextTaskID,
		Completed:    sys.Completed,
		MemoryTotal:  sys.Memory.TotalSize,
		MemoryUsed:   sys.Memory.UsedSize,
		CPUState:     int(sys.CPU.State),
		MaxProcesses: MaxProcesses,
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

	snap.Ready = processInfos(sys.Planner.ReadyQueue)
	snap.Blocked = processInfos(sys.IO.BlockedQueue)
	snap.Processes = processInfos(sys.Processes)
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
			IOTicksLeft:     p.IOTicksLeft,
		})
	}
	return infos
}
