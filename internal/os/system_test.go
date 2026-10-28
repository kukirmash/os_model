package os

import (
	"math"
	"testing"

	"os_model/internal/process"
	"os_model/internal/stats"
)

// TestTickInvariants прогоняет модель несколько тысяч тактов и проверяет
// согласованность состояний процессов, очередей, памяти и прерываний.
func TestTickInvariants(t *testing.T) {
	sys := &System{Config: Config{MemorySize: 4096, TickDurationMs: 1, IOMaxDuration: 10}}
	sys.Init()
	for sys.GenerateAndLoadTask() {
	}

	if len(sys.Regulator.Table) == 0 {
		t.Fatal("не загружено ни одного задания")
	}

	sawInitIO := false

	for tick := 0; tick < 3000; tick++ {
		sys.DoOneTick()
		sys.ViewSnapshot() // индикация не должна паниковать

		// Процессы, обслуживаемые процессорами ВВ.
		opProcs := make(map[int]bool, len(sys.IO.Operations))
		for _, op := range sys.IO.Operations {
			opProcs[op.Process.ID] = true
			if op.TicksLeft <= 0 {
				t.Fatalf("такт %d: операция ВВ процесса #%d с нулевым счётчиком не завершена", tick, op.Process.ID)
			}
		}

		used := 0
		active, ready, ioBusy := 0, 0, 0
		for _, p := range sys.Regulator.Table {
			used += p.Task.Size
			switch p.State {
			case process.StateActive:
				active++
			case process.StateReady:
				ready++
			case process.StateInitIO:
				ioBusy++
				sawInitIO = true
				if !opProcs[p.ID] {
					t.Fatalf("такт %d: процесс #%d в «Инициализация IO» без операции ВВ", tick, p.ID)
				}
			case process.StateBlockedIO:
				ioBusy++
				if !opProcs[p.ID] {
					t.Fatalf("такт %d: процесс #%d в «Блокирован» без операции ВВ", tick, p.ID)
				}
			default:
				t.Fatalf("такт %d: недопустимое состояние %d у процесса #%d", tick, p.State, p.ID)
			}
		}

		if used != sys.Memory.UsedSize {
			t.Fatalf("такт %d: занято памяти %d, по таблице PSW %d", tick, sys.Memory.UsedSize, used)
		}
		if ready != len(sys.Regulator.Planner.ReadyQueue) {
			t.Fatalf("такт %d: готовых %d, в очереди готовности %d", tick, ready, len(sys.Regulator.Planner.ReadyQueue))
		}
		if ioBusy != len(sys.IO.Operations) {
			t.Fatalf("такт %d: процессов на ВВ %d, операций %d", tick, ioBusy, len(sys.IO.Operations))
		}
		if active > 1 {
			t.Fatalf("такт %d: активных процессов %d", tick, active)
		}
		if active == 1 && sys.CPU.ActiveProcess == nil {
			t.Fatalf("такт %d: активный процесс не загружен на ЦПр", tick)
		}
		if active == 0 && sys.CPU.ActiveProcess != nil {
			t.Fatalf("такт %d: ЦПр занят процессом в состоянии %d", tick, sys.CPU.ActiveProcess.State)
		}
		if active == 0 && ready > 0 {
			t.Fatalf("такт %d: ЦПр простаивает при %d готовых процессах", tick, ready)
		}
		if len(sys.Interrupts.Pending) != 0 {
			t.Fatalf("такт %d: %d необработанных сигналов прерывания", tick, len(sys.Interrupts.Pending))
		}
		if got := sys.Regulator.Completed + len(sys.Regulator.Table); got != sys.nextTaskID {
			t.Fatalf("такт %d: выполнено %d + резидентных %d != загружено %d",
				tick, sys.Regulator.Completed, len(sys.Regulator.Table), sys.nextTaskID)
		}

		// Инварианты статистики (лабораторная работа 6).
		if sys.Stats.CPUBusy+sys.Stats.CPUIdle != sys.Time {
			t.Fatalf("такт %d: занятость ЦПр %d + простой %d != время %d",
				tick, sys.Stats.CPUBusy, sys.Stats.CPUIdle, sys.Time)
		}
		if len(sys.Stats.ExecHistory) > stats.HistoryLimit {
			t.Fatalf("такт %d: в последовательности выполнения %d процессов", tick, len(sys.Stats.ExecHistory))
		}
		if sys.Stats.CompletedCount != sys.Regulator.Completed {
			t.Fatalf("такт %d: статистика завершённых %d != счётчик %d",
				tick, sys.Stats.CompletedCount, sys.Regulator.Completed)
		}
		if sys.Stats.LoadedCount != sys.nextTaskID {
			t.Fatalf("такт %d: статистика загруженных %d != счётчик %d",
				tick, sys.Stats.LoadedCount, sys.nextTaskID)
		}

		// Каждый такт процесс проводит ровно в одном состоянии, поэтому
		// сумма его времён равна времени пребывания в системе; суммарное
		// время на ЦПр совпадает с занятостью процессора.
		execSum := sys.Stats.SumExec
		for _, p := range sys.Regulator.Table {
			if got := p.WaitTicks + p.ExecTicks + p.IOTicks; got != sys.Time-p.LoadedAt {
				t.Fatalf("такт %d: времена процесса #%d: ожидание %d + ЦПр %d + ВВ %d != %d",
					tick, p.ID, p.WaitTicks, p.ExecTicks, p.IOTicks, sys.Time-p.LoadedAt)
			}
			execSum += p.ExecTicks
		}
		if execSum != sys.Stats.CPUBusy {
			t.Fatalf("такт %d: время на ЦПр по процессам %d != занятость ЦПр %d",
				tick, execSum, sys.Stats.CPUBusy)
		}

		// Инварианты лабораторной работы 7: разложение времён и
		// системные затраты.
		systemSum := sys.Stats.SystemLoadTicks + sys.Stats.SystemDispatchTicks +
			sys.Stats.SystemIOInitTicks + sys.Stats.SystemIOCompleteTicks +
			sys.Stats.SystemTerminateTicks
		if systemSum != sys.Stats.SystemTicks {
			t.Fatalf("такт %d: разбивка системных затрат %d != сумма %d",
				tick, systemSum, sys.Stats.SystemTicks)
		}
		effective := sys.Stats.EffectiveTime()
		for _, p := range sys.Regulator.Table {
			mono := stats.MonoTicks(p)
			multi := stats.MultiTicks(p)
			if mono > multi {
				t.Fatalf("такт %d: T_mono %d > T_multi %d у процесса #%d",
					tick, mono, multi, p.ID)
			}
			if multi > effective {
				t.Fatalf("такт %d: T_multi %d > T_эфф %d у процесса #%d",
					tick, multi, effective, p.ID)
			}
			if got := stats.ExecutionIncrease(p); got < 100 {
				t.Fatalf("такт %d: D_exe %.2f%% < 100%% у процесса #%d", tick, got, p.ID)
			}
		}
	}

	if sys.Regulator.Completed == 0 {
		t.Error("за 3000 тактов не завершено ни одного задания")
	}
	if sys.Interrupts.Handled == 0 {
		t.Error("за 3000 тактов не обработано ни одного прерывания")
	}
	if !sawInitIO {
		t.Error("состояние «Инициализация IO» ни разу не наблюдалось")
	}

	// Статистика за прогон: последовательность выполнения заполнена,
	// показатели средних значений определены.
	if len(sys.Stats.ExecHistory) == 0 {
		t.Error("последовательность выполнения процессов на ЦПр пуста")
	}
	if sys.Stats.CompletedCount == 0 {
		t.Error("нет завершённых процессов для средних значений")
	}
	if util := sys.Stats.CPUUtilization(); util <= 0 || util > 100 {
		t.Errorf("полезная загрузка ЦПр %.2f вне диапазона (0, 100]", util)
	}
	if sys.Stats.AvgReadyQueue() > float64(sys.Stats.ReadyMax) {
		t.Error("средняя длина очереди больше максимальной")
	}
	if sys.Stats.AvgTurnaround() <= 0 {
		t.Error("среднее оборотное время должно быть положительным")
	}

	// Показатели сравнения режимов (лабораторная работа 7).
	if sys.Stats.SystemTicks <= 0 {
		t.Error("системные затраты должны быть положительными")
	}
	if sys.Stats.SumMono <= 0 || sys.Stats.SumMulti <= 0 {
		t.Error("суммы T_mono и T_multi должны быть положительными")
	}
	if sys.Stats.AvgDExe() < 100 {
		t.Errorf("среднее D_exe %.2f%% меньше 100%%", sys.Stats.AvgDExe())
	}
	if got, want := sys.Stats.Performance(), 100*float64(sys.Stats.SumMono)/float64(sys.Stats.EffectiveTime()); math.Abs(got-want) > 1e-9 {
		t.Errorf("D_multi = %.4f, по формуле ΣT_mono/T_эфф = %.4f", got, want)
	}
	if sys.Stats.MonoPossible() <= 0 {
		t.Error("M_mono должно быть положительным")
	}

	// Снимок состояния содержит статистику и параметры нового задания.
	view := sys.ViewSnapshot()
	if view.Stats.CompletedCount != sys.Regulator.Completed {
		t.Error("снимок: счётчик завершённых процессов статистики не совпадает")
	}
	if len(view.Stats.ExecHistory) != len(sys.Stats.ExecHistory) {
		t.Error("снимок: последовательность выполнения процессов не совпадает")
	}
	if !view.LastTask.Loaded {
		t.Error("снимок: последнее задание должно быть загружено")
	}
	if view.Stats.TEffective != sys.Stats.EffectiveTime() {
		t.Error("снимок: T_эфф не совпадает")
	}
	if view.Stats.Performance != sys.Stats.Performance() {
		t.Error("снимок: D_multi не совпадает")
	}
	if len(view.Processes) > 0 {
		p := view.Processes[0]
		if p.TMono <= 0 || p.TMulti < p.TMono {
			t.Errorf("снимок: неверные T_mono/T_multi процесса #%d: %d/%d", p.ID, p.TMono, p.TMulti)
		}
	}
}
