package os

import (
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
		sys.Snapshot() // индикация не должна паниковать

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

	// Снимок состояния содержит статистику и параметры нового задания.
	snap := sys.Snapshot()
	if snap.Stats.CompletedCount != sys.Regulator.Completed {
		t.Error("снимок: счётчик завершённых процессов статистики не совпадает")
	}
	if len(snap.Stats.ExecHistory) != len(sys.Stats.ExecHistory) {
		t.Error("снимок: последовательность выполнения процессов не совпадает")
	}
	if !snap.LastTask.Loaded {
		t.Error("снимок: последнее задание должно быть загружено")
	}
}
