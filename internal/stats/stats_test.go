package stats

import (
	"math"
	"testing"

	"os_model/internal/process"
)

// TestProcessTimes проверяет разложение T_mono и T_multi по составляющим.
func TestProcessTimes(t *testing.T) {
	p := &process.PSW{
		LoadTicks:       2,
		ExecTicks:       5,
		IOTicks:         3,
		TermTicks:       1,
		SwitchTicks:     4,
		IOInitTicks:     2,
		IOIntrTicks:     1,
		WaitTicks:       10,
		SystemWaitTicks: 6,
	}

	if got, want := MonoTicks(p), 11; got != want {
		t.Errorf("T_mono = %d, ожидалось %d", got, want)
	}
	if got, want := ReadyTicks(p), 16; got != want {
		t.Errorf("T_ожид = %d, ожидалось %d", got, want)
	}
	if got, want := MultiTicks(p), 34; got != want {
		t.Errorf("T_multi = %d, ожидалось %d", got, want)
	}
	if got, want := ExecutionIncrease(p), 100.0*34/11; math.Abs(got-want) > 1e-9 {
		t.Errorf("D_exe = %.2f, ожидалось %.2f", got, want)
	}
	if got, want := ReadyShare(p), 100.0*16/34; math.Abs(got-want) > 1e-9 {
		t.Errorf("D_ready = %.2f, ожидалось %.2f", got, want)
	}
}

// TestSystemMetrics проверяет производительность и долю системных затрат.
func TestSystemMetrics(t *testing.T) {
	s := &Stats{
		Now:            100,
		SystemTicks:    20,
		CompletedCount: 2,
		SumMono:        40,
		SumMulti:       80,
		SumDExe:        300,
		SumDReady:      50,
	}

	if got, want := s.EffectiveTime(), 120; got != want {
		t.Errorf("T_эфф = %d, ожидалось %d", got, want)
	}
	if got, want := s.MonoAvg(), 20.0; got != want {
		t.Errorf("T_mono_avg = %.2f, ожидалось %.2f", got, want)
	}
	if got, want := s.MonoPossible(), 6.0; got != want {
		t.Errorf("M_mono = %.2f, ожидалось %.2f", got, want)
	}
	if got, want := s.Performance(), 100.0*2/6; math.Abs(got-want) > 1e-9 {
		t.Errorf("D_multi = %.2f, ожидалось %.2f", got, want)
	}
	if got, want := s.SystemShare(), 100.0*20/120; math.Abs(got-want) > 1e-9 {
		t.Errorf("D_sys = %.2f, ожидалось %.2f", got, want)
	}
	if got, want := s.AvgDExe(), 150.0; got != want {
		t.Errorf("AvgDExe = %.2f, ожидалось %.2f", got, want)
	}
	if got, want := s.AvgDReady(), 25.0; got != want {
		t.Errorf("AvgDReady = %.2f, ожидалось %.2f", got, want)
	}
}

// TestSystemMetricsZero проверяет отсутствие деления на ноль.
func TestSystemMetricsZero(t *testing.T) {
	s := &Stats{}
	if s.MonoPossible() != 0 || s.Performance() != 0 || s.SystemShare() != 0 {
		t.Error("при нулевых данных производные показатели должны быть равны 0")
	}
	if s.AvgDExe() != 0 || s.AvgDReady() != 0 {
		t.Error("при отсутствии завершённых процессов средние должны быть равны 0")
	}
}

// TestOnSystemAction проверяет накопление системных затрат по видам.
func TestOnSystemAction(t *testing.T) {
	s := &Stats{}
	s.OnSystemAction(SystemLoad, 2)
	s.OnSystemAction(SystemDispatch, 1)
	s.OnSystemAction(SystemIOInit, 3)
	s.OnSystemAction(SystemIOComplete, 4)
	s.OnSystemAction(SystemTerminate, 5)
	s.OnSystemAction(SystemLoad, 0)
	s.OnSystemAction(SystemLoad, -1)

	if got, want := s.SystemTicks, 15; got != want {
		t.Errorf("SystemTicks = %d, ожидалось %d", got, want)
	}
	if s.SystemLoadTicks != 2 || s.SystemDispatchTicks != 1 ||
		s.SystemIOInitTicks != 3 || s.SystemIOCompleteTicks != 4 ||
		s.SystemTerminateTicks != 5 {
		t.Error("разбивка системных затрат не совпадает с начислениями")
	}
}
