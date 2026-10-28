package stats

import "os_model/internal/process"

// ----------------------------------------------------------------------------------------
// Расчёт времён процесса для сравнения однопрограммного и
// мультипрограммного режимов (лабораторная работа 7).
//
// T_mono — время выполнения задания в однопрограммной ОС: загрузка,
//           выполнение команд, ввод-вывод и завершение.
// T_multi — время нахождения процесса в мультипрограммной системе:
//           к T_mono добавляются переключения, логическая инициализация
//           ВВ, обслуживание прерываний и ожидание в очереди готовности.

// MonoTicks — время выполнения процесса в однопрограммной ОС, тактов.
func MonoTicks(p *process.PSW) int {
	if p == nil {
		return 0
	}
	return p.LoadTicks + p.ExecTicks + p.IOTicks + p.TermTicks
}

// ReadyTicks — полное время ожидания процесса в очереди готовности,
// включая дополнительные такты системных действий.
func ReadyTicks(p *process.PSW) int {
	if p == nil {
		return 0
	}
	return p.WaitTicks + p.SystemWaitTicks
}

// MultiTicks — время нахождения процесса в мультипрограммной системе, тактов.
func MultiTicks(p *process.PSW) int {
	if p == nil {
		return 0
	}
	return MonoTicks(p) +
		p.SwitchTicks +
		p.IOInitTicks +
		p.IOIntrTicks +
		p.SuspendedTicks +
		ReadyTicks(p)
}

// ExecutionIncrease — увеличение времени выполнения задания по сравнению
// с однопрограммной ОС, %.
func ExecutionIncrease(p *process.PSW) float64 {
	mono := MonoTicks(p)
	if mono <= 0 {
		return 0
	}
	return float64(MultiTicks(p)) / float64(mono) * 100
}

// ReadyShare — доля времени ожидания в T_multi, %.
func ReadyShare(p *process.PSW) float64 {
	multi := MultiTicks(p)
	if multi <= 0 {
		return 0
	}
	return float64(ReadyTicks(p)) / float64(multi) * 100
}
