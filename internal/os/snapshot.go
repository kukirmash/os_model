package os

import (
	"os_model/internal/interrupt"
	"os_model/internal/io"
	"os_model/internal/params"
	"os_model/internal/process"
	"os_model/internal/stats"
)

// ----------------------------------------------------------------------------------------
// Типы этого файла относятся только к индикации (UI). Префикс UI отделяет
// данные для интерфейса от структур ядра: process.PSW, stats.Stats,
// regulator.Regulator и т. д. Ядро не зависит от этих типов.

// UIProcessInfo — данные одного процесса для индикации.
type UIProcessInfo struct {
	ID              int `json:"id"`
	Size            int `json:"size"`
	PC              int `json:"pc"`
	TotalCommands   int `json:"totalCommands"`
	IOPercent       int `json:"ioPercent"`
	State           int `json:"state"`
	DynamicPriority int `json:"dynamicPriority"`
	BasePriority    int `json:"basePriority"`

	// Времена процесса, тактов (лабораторная работа 6).
	WaitTicks      int `json:"waitTicks"`      // ожидание в очереди готовности
	ExecTicks      int `json:"execTicks"`      // выполнение на ЦПр
	IOTicks        int `json:"ioTicks"`        // «Инициализация IO» и «Блокирован»
	SuspendedTicks int `json:"suspendedTicks"` // «Приостановлен»
	Turnaround     int `json:"turnaround"`     // оборотное время от загрузки

	// Составляющие системных затрат (лабораторная работа 7).
	LoadTicks       int `json:"loadTicks"`
	TermTicks       int `json:"termTicks"`
	SwitchTicks     int `json:"switchTicks"`
	IOInitTicks     int `json:"ioInitTicks"`
	IOIntrTicks     int `json:"ioIntrTicks"`
	SystemWaitTicks int `json:"systemWaitTicks"`

	// Расчётные времена процесса (лабораторная работа 7).
	TMono  int     `json:"tMono"`  // время в однопрограммной ОС
	TMulti int     `json:"tMulti"` // время в мультипрограммной системе
	DExe   float64 `json:"dExe"`   // увеличение времени выполнения, %
	TReady int     `json:"tReady"` // полное время ожидания
	DReady float64 `json:"dReady"` // доля ожидания в T_multi, %
}

// ----------------------------------------------------------------------------------------
// UIOperationInfo — данные одной операции ввода-вывода (занятого процессора ВВ).
type UIOperationInfo struct {
	ProcessorID int `json:"processorId"` // номер процессора ВВ
	ProcessID   int `json:"processId"`   // обслуживаемый процесс
	State       int `json:"state"`       // «Инициализация IO» или «Блокирован»
	TicksLeft   int `json:"ticksLeft"`   // осталось тактов
	TotalTicks  int `json:"totalTicks"`  // полная длительность операции
}

// ----------------------------------------------------------------------------------------
// UIInterruptInfo — данные сигнала прерывания для индикации.
type UIInterruptInfo struct {
	Type      int `json:"type"`      // тип прерывания
	Time      int `json:"time"`      // такт модели
	ProcessID int `json:"processId"` // процесс, которого касается прерывание
	SourceID  int `json:"sourceId"`  // номер источника (процессора ВВ)
}

// ----------------------------------------------------------------------------------------
// UITaskParams — диапазоны генерации заданий (входные параметры модели).
type UITaskParams struct {
	SizeMin       int `json:"sizeMin"`
	SizeMax       int `json:"sizeMax"`
	SizeStep      int `json:"sizeStep"`
	CommandsMin   int `json:"commandsMin"`
	CommandsMax   int `json:"commandsMax"`
	CommandsStep  int `json:"commandsStep"`
	IOPercentMin  int `json:"ioPercentMin"`
	IOPercentMax  int `json:"ioPercentMax"`
	IOPercentStep int `json:"ioPercentStep"`
	PriorityMin   int `json:"priorityMin"`
	PriorityMax   int `json:"priorityMax"`
	PriorityStep  int `json:"priorityStep"`
}

// ----------------------------------------------------------------------------------------
// UIParamRow — строка таблицы входных параметров из файла.
type UIParamRow struct {
	Name    string `json:"name"`
	Min     int    `json:"min"`
	Max     int    `json:"max"`
	Step    int    `json:"step"`
	Comment string `json:"comment"`
}

// ----------------------------------------------------------------------------------------
// UIParamsInfo — сведения о JSON-файле параметров.
type UIParamsInfo struct {
	File   string       `json:"file"`
	Loaded bool         `json:"loaded"`
	Error  string       `json:"error"`
	Seed   uint64       `json:"seed"`
	Rows   []UIParamRow `json:"rows"`
}

// ----------------------------------------------------------------------------------------
// UIDirectiveInfo — результат последней директивы оператора.
type UIDirectiveInfo struct {
	Time    int    `json:"time"`
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// ----------------------------------------------------------------------------------------
// UITaskInfo — параметры нового (последнего сгенерированного) задания.
type UITaskInfo struct {
	ID            int    `json:"id"`
	Size          int    `json:"size"`
	TotalCommands int    `json:"totalCommands"`
	IOPercent     int    `json:"ioPercent"`
	BasePriority  int    `json:"basePriority"`
	Loaded        bool   `json:"loaded"` // задание загружено в память
	Reason        string `json:"reason"` // причина, если загрузить не удалось
}

// ----------------------------------------------------------------------------------------
// UICostsInfo — длительности системных действий (лабораторная работа 7).
type UICostsInfo struct {
	Load        int `json:"load"`
	Dispatch    int `json:"dispatch"`
	IOInit      int `json:"ioInit"`
	IOInterrupt int `json:"ioInterrupt"`
	Terminate   int `json:"terminate"`
}

// ----------------------------------------------------------------------------------------
// UIStatsInfo — статистика модели для индикации (лабораторные работы 6–7).
type UIStatsInfo struct {
	// Центральный процессор.
	CPUBusy        int     `json:"cpuBusy"`        // тактов процессор выполнял команды
	CPUIdle        int     `json:"cpuIdle"`        // тактов процессор простаивал
	CPUUtilization float64 `json:"cpuUtilization"` // полезная загрузка ЦПр, %

	// Длины очередей.
	ReadyLen  int     `json:"readyLen"`  // текущая длина очереди готовности
	ReadyAvg  float64 `json:"readyAvg"`  // средняя длина очереди готовности
	ReadyMax  int     `json:"readyMax"`  // максимальная длина очереди
	IOBusy    int     `json:"ioBusy"`    // занятых процессоров ВВ сейчас
	IOBusyAvg float64 `json:"ioBusyAvg"` // среднее число занятых процессоров ВВ
	IOBusyMax int     `json:"ioBusyMax"` // максимум занятых процессоров ВВ

	// Производительность системы: выполненных заданий на 1000 тактов.
	Throughput float64 `json:"throughput"`

	// Последовательность выполнения процессов на ЦПр (до 16 номеров).
	ExecHistory []int `json:"execHistory"`

	// Средние входные параметры загруженных заданий.
	LoadedCount  int     `json:"loadedCount"`
	AvgSize      float64 `json:"avgSize"`
	AvgCommands  float64 `json:"avgCommands"`
	AvgIOPercent float64 `json:"avgIoPercent"`
	AvgPriority  float64 `json:"avgPriority"`

	// Средние выходные параметры завершённых процессов.
	CompletedCount int     `json:"completedCount"`
	DestroyedCount int     `json:"destroyedCount"`
	AvgWait        float64 `json:"avgWait"`       // ожидание, тактов
	AvgExec        float64 `json:"avgExec"`       // выполнение на ЦПр, тактов
	AvgIO          float64 `json:"avgIo"`         // ввод-вывод, тактов
	AvgTurnaround  float64 `json:"avgTurnaround"` // оборотное время, тактов

	// Сравнение однопрограммного и мультипрограммного режимов
	// (лабораторная работа 7).
	TModel           int     `json:"tModel"`           // модельные такты
	TSystem          int     `json:"tSystem"`          // системные затраты, тактов
	TEffective       int     `json:"tEffective"`       // T_мод + T_сист
	SumMono          int     `json:"sumMono"`          // Σ T_mono завершённых
	AvgMono          float64 `json:"avgMono"`          // среднее T_mono
	AvgMulti         float64 `json:"avgMulti"`         // среднее T_multi
	MMono            float64 `json:"mMono"`            // возможное число заданий
	MMulti           int     `json:"mMulti"`           // выполнено заданий
	Performance      float64 `json:"performance"`      // D_multi, %
	SystemShare      float64 `json:"systemShare"`      // D_sys, %
	AvgDExe          float64 `json:"avgDExe"`          // среднее D_exe, %
	AvgDReady        float64 `json:"avgDReady"`        // среднее D_ready, %
	SystemLoadTicks  int     `json:"systemLoadTicks"`  // затраты на загрузку
	SystemDispatch   int     `json:"systemDispatch"`   // затраты на переключения
	SystemIOInit     int     `json:"systemIoInit"`     // затраты на инициализацию ВВ
	SystemIOComplete int     `json:"systemIoComplete"` // затраты на прерывания ВВ
	SystemTerminate  int     `json:"systemTerminate"`  // затраты на завершение
}

// ----------------------------------------------------------------------------------------
// UISnapshot — снимок состояния модели для индикации в интерфейсе.
type UISnapshot struct {
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
	InterruptsHandled int               `json:"interruptsHandled"`
	InterruptLog      []UIInterruptInfo `json:"interruptLog"`

	MaxProcesses int `json:"maxProcesses"`

	Ready      []UIProcessInfo   `json:"ready"`      // очередь готовности
	Operations []UIOperationInfo `json:"operations"` // занятые процессоры ВВ
	Processes  []UIProcessInfo   `json:"processes"`  // таблица PSW

	TaskParams UITaskParams `json:"taskParams"`
	Params     UIParamsInfo `json:"params"`   // входные параметры из файла (лаб. 8)
	LastTask   UITaskInfo   `json:"lastTask"` // параметры нового задания
	NextTask   *UITaskInfo  `json:"nextTask"` // переопределение следующего задания (лаб. 9)
	Costs      UICostsInfo  `json:"costs"`    // системные затраты (лаб. 7)
	Stats      UIStatsInfo  `json:"stats"`    // статистика модели (лаб. 6–7)

	// Управление оператором (лабораторная работа 9).
	AutoLoadEnabled bool            `json:"autoLoadEnabled"`
	FinishAndQuit   bool            `json:"finishAndQuit"`
	LastDirective   UIDirectiveInfo `json:"lastDirective"`
}

// ----------------------------------------------------------------------------------------
// ViewSnapshot формирует снимок текущего состояния модели для интерфейса.
func (sys *System) ViewSnapshot() UISnapshot {
	view := UISnapshot{
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
		AutoLoadEnabled:   sys.AutoLoadEnabled,
		FinishAndQuit:     sys.FinishAndQuit,
		LastDirective: UIDirectiveInfo{
			Time:    sys.LastDirective.Time,
			Name:    sys.LastDirective.Name,
			OK:      sys.LastDirective.OK,
			Message: sys.LastDirective.Message,
		},
		TaskParams: UITaskParams{
			SizeMin:       sys.Params.Task.Size.Min,
			SizeMax:       sys.Params.Task.Size.Max,
			SizeStep:      sys.Params.Task.Size.Step,
			CommandsMin:   sys.Params.Task.Commands.Min,
			CommandsMax:   sys.Params.Task.Commands.Max,
			CommandsStep:  sys.Params.Task.Commands.Step,
			IOPercentMin:  sys.Params.Task.IOPercent.Min,
			IOPercentMax:  sys.Params.Task.IOPercent.Max,
			IOPercentStep: sys.Params.Task.IOPercent.Step,
			PriorityMin:   sys.Params.Task.BasePriority.Min,
			PriorityMax:   sys.Params.Task.BasePriority.Max,
			PriorityStep:  sys.Params.Task.BasePriority.Step,
		},
		Params: uiParamsInfo(sys.ParamsPath, sys.ParamsLoaded, sys.ParamsError, sys.Params),
		Costs: UICostsInfo{
			Load:        sys.Regulator.Costs.Load,
			Dispatch:    sys.Regulator.Costs.Dispatch,
			IOInit:      sys.Regulator.Costs.IOInit,
			IOInterrupt: sys.Regulator.Costs.IOInterrupt,
			Terminate:   sys.Regulator.Costs.Terminate,
		},
	}

	// Команда актуальна только при активном процессе.
	if p := sys.CPU.ActiveProcess; p != nil {
		view.ActiveID = p.ID
		view.CommandType = int(sys.CPU.CurrentCommand.Type)
		view.CommandCode = int(sys.CPU.CurrentCommand.Code)
		view.CommandAddr1 = sys.CPU.CurrentCommand.Address1
		view.CommandAddr2 = sys.CPU.CurrentCommand.Address2
		view.CommandDuration = sys.CPU.CurrentCommand.Duration
	}

	view.LastTask = UITaskInfo{
		ID:            sys.LastTask.Task.ID,
		Size:          sys.LastTask.Task.Size,
		TotalCommands: sys.LastTask.Task.TotalCommands,
		IOPercent:     sys.LastTask.Task.IOPercent,
		BasePriority:  sys.LastTask.Task.BasePriority,
		Loaded:        sys.LastTask.Loaded,
		Reason:        sys.LastTask.Reason,
	}
	if sys.NextTask != nil {
		view.NextTask = &UITaskInfo{
			Size:          sys.NextTask.Size,
			TotalCommands: sys.NextTask.TotalCommands,
			IOPercent:     sys.NextTask.IOPercent,
			BasePriority:  sys.NextTask.BasePriority,
		}
	}
	view.Stats = sys.uiStatsInfo()

	view.Ready = uiProcessInfos(sys.Regulator.Planner.ReadyQueue, sys.Time)
	view.Operations = uiOperationInfos(sys.IO.Operations)
	view.Processes = uiProcessInfos(sys.Regulator.Table, sys.Time)
	view.InterruptLog = uiInterruptInfos(sys.Interrupts.Log)
	return view
}

// ----------------------------------------------------------------------------------------
// uiParamsInfo формирует данные о входных параметрах для индикации.
func uiParamsInfo(path string, loaded bool, errText string, set params.Set) UIParamsInfo {
	row := func(name string, p params.Param) UIParamRow {
		return UIParamRow{
			Name:    name,
			Min:     p.Min,
			Max:     p.Max,
			Step:    p.Step,
			Comment: p.Comment,
		}
	}

	return UIParamsInfo{
		File:   path,
		Loaded: loaded,
		Error:  errText,
		Seed:   set.Seed,
		Rows: []UIParamRow{
			row("Объём памяти", set.System.MemorySize),
			row("Длительность такта", set.System.TickDurationMs),
			row("Максимальная длительность ВВ", set.System.IOMaxDuration),
			row("Затраты на загрузку", set.System.Costs.Load),
			row("Затраты на переключение", set.System.Costs.Dispatch),
			row("Затраты на инициализацию ВВ", set.System.Costs.IOInit),
			row("Затраты на прерывание ВВ", set.System.Costs.IOInterrupt),
			row("Затраты на завершение", set.System.Costs.Terminate),
			row("Размер задания", set.Task.Size),
			row("Число команд", set.Task.Commands),
			row("Процент команд ВВ", set.Task.IOPercent),
			row("Базовый приоритет", set.Task.BasePriority),
		},
	}
}

// ----------------------------------------------------------------------------------------
// uiStatsInfo собирает статистику модели в данные для индикации.
func (sys *System) uiStatsInfo() UIStatsInfo {
	return UIStatsInfo{
		CPUBusy:          sys.Stats.CPUBusy,
		CPUIdle:          sys.Stats.CPUIdle,
		CPUUtilization:   sys.Stats.CPUUtilization(),
		ReadyLen:         len(sys.Regulator.Planner.ReadyQueue),
		ReadyAvg:         sys.Stats.AvgReadyQueue(),
		ReadyMax:         sys.Stats.ReadyMax,
		IOBusy:           len(sys.IO.Operations),
		IOBusyAvg:        sys.Stats.AvgIOBusy(),
		IOBusyMax:        sys.Stats.IOBusyMax,
		Throughput:       sys.Stats.Throughput(),
		ExecHistory:      sys.Stats.History(),
		LoadedCount:      sys.Stats.LoadedCount,
		AvgSize:          sys.Stats.AvgSize(),
		AvgCommands:      sys.Stats.AvgCommands(),
		AvgIOPercent:     sys.Stats.AvgIOPercent(),
		AvgPriority:      sys.Stats.AvgPriority(),
		CompletedCount:   sys.Stats.CompletedCount,
		DestroyedCount:   sys.Stats.DestroyedCount,
		AvgWait:          sys.Stats.AvgWait(),
		AvgExec:          sys.Stats.AvgExec(),
		AvgIO:            sys.Stats.AvgIO(),
		AvgTurnaround:    sys.Stats.AvgTurnaround(),
		TModel:           sys.Time,
		TSystem:          sys.Stats.SystemTicks,
		TEffective:       sys.Stats.EffectiveTime(),
		SumMono:          sys.Stats.SumMono,
		AvgMono:          sys.Stats.MonoAvg(),
		AvgMulti:         sys.Stats.MultiAvg(),
		MMono:            sys.Stats.MonoPossible(),
		MMulti:           sys.Stats.CompletedCount,
		Performance:      sys.Stats.Performance(),
		SystemShare:      sys.Stats.SystemShare(),
		AvgDExe:          sys.Stats.AvgDExe(),
		AvgDReady:        sys.Stats.AvgDReady(),
		SystemLoadTicks:  sys.Stats.SystemLoadTicks,
		SystemDispatch:   sys.Stats.SystemDispatchTicks,
		SystemIOInit:     sys.Stats.SystemIOInitTicks,
		SystemIOComplete: sys.Stats.SystemIOCompleteTicks,
		SystemTerminate:  sys.Stats.SystemTerminateTicks,
	}
}

// ----------------------------------------------------------------------------------------
// uiProcessInfos преобразует список PSW в данные для индикации.
// now — текущий такт модели: по нему вычисляется оборотное время.
func uiProcessInfos(psws []*process.PSW, now int) []UIProcessInfo {
	infos := make([]UIProcessInfo, 0, len(psws))
	for _, p := range psws {
		infos = append(infos, UIProcessInfo{
			ID:              p.ID,
			Size:            p.Task.Size,
			PC:              p.PC,
			TotalCommands:   p.Task.TotalCommands,
			IOPercent:       p.Task.IOPercent,
			State:           int(p.State),
			DynamicPriority: p.DynamicPriority,
			BasePriority:    p.Task.BasePriority,
			WaitTicks:       p.WaitTicks,
			ExecTicks:       p.ExecTicks,
			IOTicks:         p.IOTicks,
			SuspendedTicks:  p.SuspendedTicks,
			Turnaround:      now - p.LoadedAt,
			LoadTicks:       p.LoadTicks,
			TermTicks:       p.TermTicks,
			SwitchTicks:     p.SwitchTicks,
			IOInitTicks:     p.IOInitTicks,
			IOIntrTicks:     p.IOIntrTicks,
			SystemWaitTicks: p.SystemWaitTicks,
			TMono:           stats.MonoTicks(p),
			TMulti:          stats.MultiTicks(p),
			DExe:            stats.ExecutionIncrease(p),
			TReady:          stats.ReadyTicks(p),
			DReady:          stats.ReadyShare(p),
		})
	}
	return infos
}

// ----------------------------------------------------------------------------------------
// uiOperationInfos преобразует операции ввода-вывода в данные для индикации.
func uiOperationInfos(operations []*io.Operation) []UIOperationInfo {
	infos := make([]UIOperationInfo, 0, len(operations))
	for _, op := range operations {
		infos = append(infos, UIOperationInfo{
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
// uiInterruptInfos преобразует журнал прерываний в данные для индикации.
func uiInterruptInfos(signals []interrupt.Signal) []UIInterruptInfo {
	infos := make([]UIInterruptInfo, 0, len(signals))
	for _, s := range signals {
		info := UIInterruptInfo{
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
