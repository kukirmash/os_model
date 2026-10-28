package regulator

import (
	"testing"

	"os_model/internal/memory"
	"os_model/internal/process"
	"os_model/internal/stats"
)

// TestIncreaseReadyTime проверяет подпрограмму «Увеличить время готовых
// процессов»: время получают только процессы в состоянии «Готов».
func TestIncreaseReadyTime(t *testing.T) {
	ready := &process.PSW{State: process.StateReady}
	active := &process.PSW{State: process.StateActive}
	initIO := &process.PSW{State: process.StateInitIO}
	blocked := &process.PSW{State: process.StateBlockedIO}

	r := &Regulator{Table: []*process.PSW{ready, active, initIO, blocked}}
	r.IncreaseReadyTime(5)

	if ready.SystemWaitTicks != 5 {
		t.Errorf("готовый процесс получил %d тактов, ожидалось 5", ready.SystemWaitTicks)
	}
	for name, p := range map[string]*process.PSW{
		"активный": active, "инициализация ВВ": initIO, "блокированный": blocked,
	} {
		if p.SystemWaitTicks != 0 {
			t.Errorf("%s процесс получил %d тактов, ожидалось 0", name, p.SystemWaitTicks)
		}
	}

	r.IncreaseReadyTime(0)
	r.IncreaseReadyTime(-3)
	if ready.SystemWaitTicks != 5 {
		t.Errorf("нулевые и отрицательные такты изменили время: %d", ready.SystemWaitTicks)
	}
}

// TestSystemActionCosts проверяет начисление затрат по всем системным
// событиям и увеличение ожидания готового процесса.
func TestSystemActionCosts(t *testing.T) {
	costs := Costs{Load: 2, Dispatch: 1, IOInit: 1, IOInterrupt: 1, Terminate: 1}
	r := &Regulator{
		Planner:      &Planner{},
		Memory:       &memory.Allocator{TotalSize: 1000, UsedSize: 100},
		MaxProcesses: 16,
		Stats:        &stats.Stats{},
		Costs:        costs,
	}

	// Постоянно готовый процесс, который должен получать системное время.
	ready := &process.PSW{State: process.StateReady, Task: &process.Task{Size: 10}}
	r.Table = append(r.Table, ready)

	p := &process.PSW{Task: &process.Task{ID: 1, Size: 100}}
	if !r.Process(p, EventLoad) {
		t.Fatal("загрузка отклонена")
	}
	if p.LoadTicks != costs.Load {
		t.Errorf("LoadTicks = %d, ожидалось %d", p.LoadTicks, costs.Load)
	}

	if !r.Process(p, EventActivate) {
		t.Fatal("активация отклонена")
	}
	if p.SwitchTicks != costs.Dispatch || p.DispatchCount != 1 {
		t.Errorf("переключение: SwitchTicks = %d, DispatchCount = %d", p.SwitchTicks, p.DispatchCount)
	}

	if !r.Process(p, EventIORequest) {
		t.Fatal("запрос ВВ отклонён")
	}
	if p.IOInitTicks != costs.IOInit || p.IORequestCount != 1 {
		t.Errorf("инициализация ВВ: IOInitTicks = %d, IORequestCount = %d", p.IOInitTicks, p.IORequestCount)
	}

	if !r.Process(p, EventIOStart) {
		t.Fatal("старт ВВ отклонён")
	}
	if !r.Process(p, EventIOComplete) {
		t.Fatal("завершение ВВ отклонено")
	}
	if p.IOIntrTicks != costs.IOInterrupt || p.IOCompleteCount != 1 {
		t.Errorf("прерывание ВВ: IOIntrTicks = %d, IOCompleteCount = %d", p.IOIntrTicks, p.IOCompleteCount)
	}

	if !r.Process(p, EventActivate) {
		t.Fatal("повторная активация отклонена")
	}
	if !r.Process(p, EventTerminate) {
		t.Fatal("завершение отклонено")
	}
	if p.TermTicks != costs.Terminate {
		t.Errorf("TermTicks = %d, ожидалось %d", p.TermTicks, costs.Terminate)
	}

	wantSystem := costs.Load + costs.Dispatch + costs.IOInit + costs.IOInterrupt +
		costs.Dispatch + costs.Terminate
	if r.Stats.SystemTicks != wantSystem {
		t.Errorf("SystemTicks = %d, ожидалось %d", r.Stats.SystemTicks, wantSystem)
	}
	if ready.SystemWaitTicks != wantSystem {
		t.Errorf("SystemWaitTicks готового процесса = %d, ожидалось %d",
			ready.SystemWaitTicks, wantSystem)
	}
}
