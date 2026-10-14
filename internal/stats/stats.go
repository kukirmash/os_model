// Package stats — модуль сбора статистики модели операционной системы.
//
// Статистика ведётся для анализа эффективности метода распределения времени
// центрального процессора (лабораторная работа 6): времена ожидания и
// выполнения процессов, длины очередей, полезная загрузка процессора,
// производительность системы и средние значения параметров процессов.
//
// Времена процесса (ожидание, выполнение, ввод-вывод) хранятся в слове
// состояния процесса — см. поля PSW в пакете process. Пакет stats хранит
// суммарные значения по системе и накопители для вычисления средних.
package stats

import "os_model/internal/process"

// ----------------------------------------------------------------------------------------
// HistoryLimit — сколько последних процессов, выполнявшихся на процессоре,
// хранится для индикации (лабораторная работа 6).
const HistoryLimit = 16

// ----------------------------------------------------------------------------------------
// Stats — накопленная статистика модели.
type Stats struct {
	Now int // текущий такт модели (обновляется подпрограммой Collect)

	// Центральный процессор.
	CPUBusy int // тактов процессор выполнял команды активного процесса
	CPUIdle int // тактов процессор простаивал

	// Очередь готовности.
	ReadySum int // сумма длин очереди готовности по тактам
	ReadyMax int // максимальная длина очереди за время моделирования

	// Процессоры ввода-вывода.
	IOBusySum int // сумма занятых процессоров ВВ по тактам
	IOBusyMax int // максимум одновременно занятых процессоров ВВ

	// Последовательность выполнения процессов на ЦПр (до HistoryLimit
	// последних номеров).
	ExecHistory []int

	// Входные параметры всех загруженных заданий (накопители средних).
	LoadedCount     int
	SumSize         int
	SumCommands     int
	SumIOPercent    int
	SumBasePriority int

	// Выходные параметры завершённых процессов (накопители средних).
	CompletedCount int
	SumWait        int
	SumExec        int
	SumIO          int
	SumTurnaround  int
}

// ----------------------------------------------------------------------------------------
// OnLoad регистрирует загруженное задание: запоминает такт загрузки
// и накапливает входные параметры для средних. Вызывается регулировщиком.
func (s *Stats) OnLoad(p *process.PSW) {
	p.LoadedAt = s.Now

	s.LoadedCount++
	s.SumSize += p.Task.Size
	s.SumCommands += p.Task.TotalCommands
	s.SumIOPercent += p.Task.IOPercent
	s.SumBasePriority += p.Task.BasePriority
}

// ----------------------------------------------------------------------------------------
// OnComplete накапливает выходные параметры завершённого процесса:
// времена ожидания, выполнения, ввода-вывода и оборотное время.
// Вызывается регулировщиком при выгрузке процесса.
func (s *Stats) OnComplete(p *process.PSW) {
	s.CompletedCount++
	s.SumWait += p.WaitTicks
	s.SumExec += p.ExecTicks
	s.SumIO += p.IOTicks
	s.SumTurnaround += s.Now - p.LoadedAt
}

// ----------------------------------------------------------------------------------------
// OnDispatch запоминает номер процесса, выбранного планировщиком для
// выполнения на центральном процессоре. Хранятся последние HistoryLimit
// номеров — «последовательность выполнения процессов на процессоре».
func (s *Stats) OnDispatch(id int) {
	s.ExecHistory = append(s.ExecHistory, id)
	if len(s.ExecHistory) > HistoryLimit {
		s.ExecHistory = s.ExecHistory[len(s.ExecHistory)-HistoryLimit:]
	}
}

// ----------------------------------------------------------------------------------------
// Collect фиксирует состояние модели за один такт: занятость процессора,
// длины очередей и времена процессов по их текущим состояниям.
// Вызывается ядром каждый такт после диспетчеризации и до выполнения
// команды. Состояния процессов не изменяются.
func (s *Stats) Collect(now int, cpuBusy bool, table []*process.PSW, readyLen, ioBusy int) {
	s.Now = now

	// Занятость центрального процессора.
	if cpuBusy {
		s.CPUBusy++
	} else {
		s.CPUIdle++
	}

	// Длина очереди готовности: сумма для средней и максимум.
	s.ReadySum += readyLen
	if readyLen > s.ReadyMax {
		s.ReadyMax = readyLen
	}

	// Занятые процессоры ввода-вывода: сумма и максимум.
	s.IOBusySum += ioBusy
	if ioBusy > s.IOBusyMax {
		s.IOBusyMax = ioBusy
	}

	// Время процессов: каждый такт процесс проводит ровно в одном
	// состоянии — ожидания, выполнения или ввода-вывода.
	for _, p := range table {
		switch p.State {
		case process.StateReady:
			p.WaitTicks++
		case process.StateActive:
			p.ExecTicks++
		case process.StateInitIO, process.StateBlockedIO:
			p.IOTicks++
		}
	}
}

// ----------------------------------------------------------------------------------------
// History возвращает копию последовательности выполнения процессов на ЦПр.
func (s *Stats) History() []int {
	history := make([]int, len(s.ExecHistory))
	copy(history, s.ExecHistory)
	return history
}

// ----------------------------------------------------------------------------------------
// Средние значения и производные показатели.

// AvgReadyQueue — средняя длина очереди готовности, процессов.
func (s *Stats) AvgReadyQueue() float64 { return average(s.ReadySum, s.Now) }

// AvgIOBusy — среднее число одновременно занятых процессоров ВВ.
func (s *Stats) AvgIOBusy() float64 { return average(s.IOBusySum, s.Now) }

// CPUUtilization — полезная загрузка процессора, %.
func (s *Stats) CPUUtilization() float64 { return percent(s.CPUBusy, s.Now) }

// Throughput — производительность системы: выполненных заданий на 1000 тактов.
func (s *Stats) Throughput() float64 { return average(s.CompletedCount*1000, s.Now) }

// AvgSize — средний размер загруженных заданий, единиц памяти.
func (s *Stats) AvgSize() float64 { return average(s.SumSize, s.LoadedCount) }

// AvgCommands — среднее число команд загруженных заданий.
func (s *Stats) AvgCommands() float64 { return average(s.SumCommands, s.LoadedCount) }

// AvgIOPercent — средний процент команд ввода-вывода загруженных заданий.
func (s *Stats) AvgIOPercent() float64 { return average(s.SumIOPercent, s.LoadedCount) }

// AvgPriority — средний базовый приоритет загруженных заданий.
func (s *Stats) AvgPriority() float64 { return average(s.SumBasePriority, s.LoadedCount) }

// AvgWait — среднее время ожидания завершённых процессов, тактов.
func (s *Stats) AvgWait() float64 { return average(s.SumWait, s.CompletedCount) }

// AvgExec — среднее время выполнения завершённых процессов на ЦПр, тактов.
func (s *Stats) AvgExec() float64 { return average(s.SumExec, s.CompletedCount) }

// AvgIO — среднее время ввода-вывода завершённых процессов, тактов.
func (s *Stats) AvgIO() float64 { return average(s.SumIO, s.CompletedCount) }

// AvgTurnaround — среднее оборотное время завершённых процессов, тактов.
func (s *Stats) AvgTurnaround() float64 { return average(s.SumTurnaround, s.CompletedCount) }

// ----------------------------------------------------------------------------------------
// average возвращает sum/count (0 при отсутствии данных).
func average(sum, count int) float64 {
	if count == 0 {
		return 0
	}
	return float64(sum) / float64(count)
}

// ----------------------------------------------------------------------------------------
// percent возвращает part/total в процентах (0 при отсутствии данных).
func percent(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) * 100 / float64(total)
}
