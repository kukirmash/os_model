// Package regulator реализует регулировщик — часть ОС, отвечающую за
// состояния процессов, и включает планировщик (Planner).
//
// Только регулировщик имеет право изменять состояния процессов: остальные
// модули модели (центральный процессор, подсистема ввода-вывода) лишь
// обращаются к подпрограмме Process, сообщая о произошедшем событии.
// Планировщик решает одну задачу — выбор следующего процесса из списка
// готовности; состояния он не изменяет.
// Регулировщик переводит процесс в новое состояние и выполняет связанные
// с этим изменения общих данных модели: очереди готовности, таблицы слов
// состояний процессов, памяти и счётчика завершённых заданий.
package regulator

import (
	"os_model/internal/memory"
	"os_model/internal/process"
	"os_model/internal/stats"
)

// ----------------------------------------------------------------------------------------
// Event — событие модели, по которому регулировщик изменяет состояние процесса.
type Event int

// ----------------------------------------------------------------------------------------
const (
	EventLoad       Event = iota // загрузка задания: создан процесс в состоянии «Готов»
	EventActivate                // выбор планировщика: «Готов» → «Активен»
	EventIORequest               // инициализация ВВ: «Активен» → «Инициализация IO»
	EventIOStart                 // процессор ВВ начал работу: «Инициализация IO» → «Блокирован»
	EventIOComplete              // прерывание от ВВ: «Блокирован» → «Готов»
	EventTerminate               // завершение задания: выгрузка процесса
	EventSuspend                 // оператор приостановил процесс
	EventResume                  // оператор возобновил процесс
)

// ----------------------------------------------------------------------------------------
// Regulator — регулировщик модели. Включает планировщик (Planner),
// выбирающий следующий процесс из очереди готовности.
type Regulator struct {
	Planner *Planner // планировщик: очередь готовности и диспетчеризация

	Memory       *memory.Allocator // супервизор памяти: освобождается при завершении
	MaxProcesses int               // число записей в таблице слов состояний процессов

	// Stats — модуль статистики: накапливает входные и выходные параметры
	// процессов (лабораторная работа 6). Может быть nil.
	Stats *stats.Stats

	// Costs — длительности системных действий (лабораторная работа 7).
	Costs Costs

	Table     []*process.PSW // таблица слов состояний процессов (резидентные процессы)
	Completed int            // число выполненных (завершённых) заданий
	Destroyed int            // число процессов, уничтоженных оператором
}

// ----------------------------------------------------------------------------------------
// Process — основная подпрограмма регулировщика: обрабатывает событие event
// для процесса p, изменяя его состояние и общие данные модели.
// Возвращает false, если событие недопустимо для текущего состояния процесса.
func (r *Regulator) Process(p *process.PSW, event Event) bool {
	switch event {
	case EventLoad:
		// Загрузка задания: нужна свободная запись в таблице PSW.
		if !r.FreeSlot() {
			return false
		}
		// Системные затраты на загрузку начисляются уже готовым процессам;
		// новый процесс в таблицу ещё не добавлен и это время не получает.
		r.charge(stats.SystemLoad, r.Costs.Load)
		p.LoadTicks += r.Costs.Load
		p.State = process.StateReady
		r.Table = append(r.Table, p)
		r.Planner.Add(p)
		if r.Stats != nil {
			r.Stats.OnLoad(p)
		}

	case EventActivate:
		// Планировщик выбрал процесс для выполнения на процессоре.
		if p.State != process.StateReady {
			return false
		}
		p.State = process.StateActive
		r.Planner.Remove(p)
		// Время выбора на ЦПр получают остальные готовые процессы;
		// выбранный процесс уже переведён в «Активен».
		r.charge(stats.SystemDispatch, r.Costs.Dispatch)
		p.SwitchTicks += r.Costs.Dispatch
		p.DispatchCount++

	case EventIORequest:
		// Задание инициализирует операцию ввода-вывода: слово состояния
		// процесса сохраняется, процесс переводится в «Инициализация IO».
		if p.State != process.StateActive {
			return false
		}
		p.State = process.StateInitIO
		r.charge(stats.SystemIOInit, r.Costs.IOInit)
		p.IOInitTicks += r.Costs.IOInit
		p.IORequestCount++

	case EventIOStart:
		// Процессор ВВ приступил к выполнению операции.
		if p.State != process.StateInitIO {
			return false
		}
		p.State = process.StateBlockedIO

	case EventIOComplete:
		// Сигнал прерывания от процессора ВВ: процесс снова готов
		// к выполнению и помещается в очередь готовности.
		if p.State != process.StateBlockedIO {
			return false
		}
		// Прерывание обслуживается, пока процесс ещё «Блокирован»,
		// поэтому собственное время ожидания он не получает.
		r.charge(stats.SystemIOComplete, r.Costs.IOInterrupt)
		p.IOIntrTicks += r.Costs.IOInterrupt
		p.IOCompleteCount++
		p.State = process.StateReady
		r.Planner.Add(p)

	case EventTerminate:
		// Завершение задания: выгрузка процесса.
		if p.State != process.StateActive {
			return false
		}
		r.charge(stats.SystemTerminate, r.Costs.Terminate)
		p.TermTicks += r.Costs.Terminate
		r.Planner.Remove(p)
		r.Memory.Free(*p.Task)
		r.removeFromTable(p)
		p.State = process.StateNone
		r.Completed++
		if r.Stats != nil {
			r.Stats.OnComplete(p)
		}

	case EventSuspend:
		// Приостановить можно готовый или активный процесс. Если процесс
		// активен, освобождение ЦПр выполняет ядро модели.
		if p.State != process.StateReady && p.State != process.StateActive {
			return false
		}
		if p.State == process.StateReady {
			r.Planner.Remove(p)
		}
		p.State = process.StateSuspended

	case EventResume:
		// Возобновлённый процесс снова становится готовым к выполнению.
		if p.State != process.StateSuspended {
			return false
		}
		p.State = process.StateReady
		r.Planner.Add(p)

	default:
		return false
	}

	return true
}

// ----------------------------------------------------------------------------------------
// IncreaseReadyTime — подпрограмма «Увеличить время готовых процессов»
// (лабораторная работа 7). Увеличивает на ticks время пребывания в системе
// всех процессов, находящихся в состоянии «Готов». Именно они ожидают,
// пока ОС выполняет системное действие.
//
// Состояния «Активен», «Инициализация IO» и «Блокирован» не изменяются:
// активный процесс в это время не ожидает, а процесс, блокированный
// по вводу-выводу, по правилу лабораторной работы системное время
// не получает.
func (r *Regulator) IncreaseReadyTime(ticks int) {
	if ticks <= 0 {
		return
	}
	for _, p := range r.Table {
		if p.State == process.StateReady {
			p.SystemWaitTicks += ticks
		}
	}
}

// ----------------------------------------------------------------------------------------
// charge начисляет время системного действия: увеличивает ожидание готовых
// процессов и суммарные системные затраты. Собственные затраты процесса
// (LoadTicks, SwitchTicks и т. д.) начисляет вызвавшая подпрограмма.
func (r *Regulator) charge(action stats.SystemAction, ticks int) {
	r.IncreaseReadyTime(ticks)
	if r.Stats != nil {
		r.Stats.OnSystemAction(action, ticks)
	}
}

// ----------------------------------------------------------------------------------------
// Terminate — подпрограмма завершения процесса, используемая процессором
// при выполнении команды завершения задания.
func (r *Regulator) Terminate(p *process.PSW) {
	r.Process(p, EventTerminate)
}

// ----------------------------------------------------------------------------------------
// Destroy — уничтожение процесса оператором. Процесс не считается
// выполненным: счётчик Completed не увеличивается, средние времена
// завершённых процессов не меняются.
func (r *Regulator) Destroy(p *process.PSW) bool {
	if p == nil || p.State == process.StateNone {
		return false
	}
	r.Planner.Remove(p)
	r.Memory.Free(*p.Task)
	r.removeFromTable(p)
	p.State = process.StateNone
	r.Destroyed++
	if r.Stats != nil {
		r.Stats.OnDestroy()
	}
	return true
}

// ----------------------------------------------------------------------------------------
// ResumeAll возобновляет все приостановленные процессы. Используется
// директивой «Выполнить загруженные задания и выключить питание».
func (r *Regulator) ResumeAll() {
	for _, p := range r.Table {
		if p.State == process.StateSuspended {
			p.State = process.StateReady
			r.Planner.Add(p)
		}
	}
}

// ----------------------------------------------------------------------------------------
// FreeSlot сообщает, есть ли свободная запись в таблице слов состояний процессов.
func (r *Regulator) FreeSlot() bool {
	return len(r.Table) < r.MaxProcesses
}

// ----------------------------------------------------------------------------------------
// removeFromTable удаляет запись процесса из таблицы слов состояний процессов.
func (r *Regulator) removeFromTable(p *process.PSW) {
	for i, t := range r.Table {
		if t == p {
			r.Table = append(r.Table[:i], r.Table[i+1:]...)
			return
		}
	}
}
