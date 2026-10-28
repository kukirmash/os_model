package os

import (
	"math"
	"testing"

	"os_model/internal/process"
	"os_model/internal/regulator"
)

// testCosts — фиксированные системные затраты для детерминированных тестов.
func testCosts() regulator.Costs {
	return regulator.Costs{
		Load:        2,
		Dispatch:    1,
		IOInit:      1,
		IOInterrupt: 1,
		Terminate:   1,
	}
}

// newLab7System создаёт модель с отключённой автозагрузкой и
// детерминированной длительностью операций ВВ (ровно 1 такт).
func newLab7System(t *testing.T, costs regulator.Costs) *System {
	t.Helper()

	sys := &System{Config: Config{
		MemorySize:     4096,
		TickDurationMs: 1,
		IOMaxDuration:  1,
		SystemCosts:    &costs,
	}}
	sys.Init()
	sys.AutoLoadEnabled = false
	return sys
}

// runUntilCompleted выполняет такты, пока не завершится want процессов.
func runUntilCompleted(t *testing.T, sys *System, want int) {
	t.Helper()

	const limit = 10000
	for tick := 0; tick < limit && sys.Regulator.Completed < want; tick++ {
		sys.DoOneTick()
	}
	if sys.Regulator.Completed != want {
		t.Fatalf("завершено %d процессов, ожидалось %d (такт %d)",
			sys.Regulator.Completed, want, sys.Time)
	}
}

// TestLab7SingleCPUBound проверяет простейший случай: одно задание без ВВ.
//
// Трассировка при затратах 2/1/1/1/1 и 3 командах:
//
//	загрузка: T_load = 2
//	такт 1: выбор на ЦПр T_next = 1, первая команда
//	такты 2–3: ещё две команды
//	такт 4: команда завершения (такт входит в ExecTicks)
//
//	T_mono  = 2 + 4 + 0 + 1 = 7
//	T_multi = 7 + 1 = 8
//	T_эфф   = 4 модельных такта + 4 системных = 8
func TestLab7SingleCPUBound(t *testing.T) {
	sys := newLab7System(t, testCosts())
	if !sys.LoadTask(process.Task{Size: 100, TotalCommands: 3, IOPercent: 0, BasePriority: 5}) {
		t.Fatal("задание не загружено")
	}

	runUntilCompleted(t, sys, 1)
	stats := sys.Stats

	if got, want := stats.SumMono, 7; got != want {
		t.Errorf("T_mono = %d, ожидалось %d", got, want)
	}
	if got, want := stats.SumMulti, 8; got != want {
		t.Errorf("T_multi = %d, ожидалось %d", got, want)
	}
	if got, want := stats.EffectiveTime(), 8; got != want {
		t.Errorf("T_эфф = %d, ожидалось %d", got, want)
	}
	if got, want := stats.SystemTicks, 4; got != want {
		t.Errorf("T_сист = %d, ожидалось %d", got, want)
	}
	if got, want := stats.AvgDExe(), 100.0*8/7; math.Abs(got-want) > 1e-9 {
		t.Errorf("D_exe = %.2f, ожидалось %.2f", got, want)
	}
	if got, want := stats.Performance(), 100.0*7/8; math.Abs(got-want) > 1e-9 {
		t.Errorf("D_multi = %.2f, ожидалось %.2f", got, want)
	}
}

// TestLab7SingleIOBound проверяет одно задание, состоящее только из команд ВВ.
//
// При IOMaxDuration = 1 каждая операция ВВ занимает один такт обслуживания
// плюс такт инициализации. Для двух команд ВВ:
//
//	T_mono  = 2 (загрузка) + 3 (команды, включая завершение)
//	          + 2 (такты ВВ) + 1 (завершение) = 8
//	T_multi = 8 + 3 (три выбора на ЦПр) + 2 (инициализация ВВ)
//	          + 2 (прерывания ВВ) = 15
//	T_эфф   = 5 модельных тактов + 10 системных = 15
func TestLab7SingleIOBound(t *testing.T) {
	sys := newLab7System(t, testCosts())
	if !sys.LoadTask(process.Task{Size: 100, TotalCommands: 2, IOPercent: 100, BasePriority: 5}) {
		t.Fatal("задание не загружено")
	}

	runUntilCompleted(t, sys, 1)
	stats := sys.Stats

	if got, want := stats.SumMono, 8; got != want {
		t.Errorf("T_mono = %d, ожидалось %d", got, want)
	}
	if got, want := stats.SumMulti, 15; got != want {
		t.Errorf("T_multi = %d, ожидалось %d", got, want)
	}
	if got, want := stats.EffectiveTime(), 15; got != want {
		t.Errorf("T_эфф = %d, ожидалось %d", got, want)
	}
	if got, want := stats.SystemTicks, 10; got != want {
		t.Errorf("T_сист = %d, ожидалось %d", got, want)
	}
}

// TestLab7TwoTasks проверяет перекрытие вычислений и ввода-вывода:
// два задания завершаются за время, меньшее суммы их T_mono.
func TestLab7TwoTasks(t *testing.T) {
	// Без системных затрат выигрыш от перекрытия ВВ и вычислений
	// проявляется в чистом виде.
	sys := newLab7System(t, regulator.Costs{})
	// Длительный ВВ, чтобы перекрытие с вычислениями было заметным.
	sys.CPU.FixedIODuration = 10

	// Первым выбирается задание с большим базовым приоритетом.
	if !sys.LoadTask(process.Task{Size: 100, TotalCommands: 20, IOPercent: 0, BasePriority: 1}) {
		t.Fatal("CPU-задание не загружено")
	}
	if !sys.LoadTask(process.Task{Size: 100, TotalCommands: 4, IOPercent: 100, BasePriority: 10}) {
		t.Fatal("IO-задание не загружено")
	}

	runUntilCompleted(t, sys, 2)
	stats := sys.Stats

	t.Logf("T_mono=%d T_multi=%d T_мод=%d T_сист=%d T_эфф=%d D_multi=%.2f%% D_exe_avg=%.2f%% D_ready_avg=%.2f%%",
		stats.SumMono, stats.SumMulti, sys.Time, stats.SystemTicks,
		stats.EffectiveTime(), stats.Performance(), stats.AvgDExe(), stats.AvgDReady())

	if stats.SumMono <= stats.EffectiveTime() {
		t.Errorf("нет выигрыша от мультипрограммирования: ΣT_mono=%d, T_эфф=%d",
			stats.SumMono, stats.EffectiveTime())
	}
	if got := stats.Performance(); got <= 100 {
		t.Errorf("D_multi = %.2f%%, ожидалось больше 100%%", got)
	}
	if stats.SumMulti <= stats.SumMono {
		t.Errorf("ΣT_multi=%d должно быть больше ΣT_mono=%d", stats.SumMulti, stats.SumMono)
	}
}

// TestLab7ZeroCosts проверяет, что при нулевых системных затратах
// T_mono и T_multi не содержат системных составляющих.
func TestLab7ZeroCosts(t *testing.T) {
	sys := newLab7System(t, regulator.Costs{})
	if !sys.LoadTask(process.Task{Size: 100, TotalCommands: 3, IOPercent: 0, BasePriority: 5}) {
		t.Fatal("задание не загружено")
	}

	runUntilCompleted(t, sys, 1)
	stats := sys.Stats

	if got, want := stats.SystemTicks, 0; got != want {
		t.Errorf("T_сист = %d, ожидалось %d", got, want)
	}
	if got, want := stats.SumMono, 4; got != want {
		t.Errorf("T_mono = %d, ожидалось %d", got, want)
	}
	if got, want := stats.SumMulti, 4; got != want {
		t.Errorf("T_multi = %d, ожидалось %d", got, want)
	}
	if got, want := stats.EffectiveTime(), sys.Time; got != want {
		t.Errorf("T_эфф = %d, ожидалось %d", got, want)
	}
	if got, want := stats.Performance(), 100.0; got != want {
		t.Errorf("D_multi = %.2f, ожидалось %.2f", got, want)
	}
}
