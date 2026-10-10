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
)

// ----------------------------------------------------------------------------------------
// Regulator — регулировщик модели. Включает планировщик (Planner),
// выбирающий следующий процесс из очереди готовности.
type Regulator struct {
	Planner *Planner // планировщик: очередь готовности и диспетчеризация

	Memory       *memory.Allocator // супервизор памяти: освобождается при завершении
	MaxProcesses int               // число записей в таблице слов состояний процессов

	Table     []*process.PSW // таблица слов состояний процессов (резидентные процессы)
	Completed int            // число выполненных (завершённых) заданий
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
		p.State = process.StateReady
		r.Table = append(r.Table, p)
		r.Planner.Add(p)

	case EventActivate:
		// Планировщик выбрал процесс для выполнения на процессоре.
		if p.State != process.StateReady {
			return false
		}
		p.State = process.StateActive
		r.Planner.Remove(p)

	case EventIORequest:
		// Задание инициализирует операцию ввода-вывода: слово состояния
		// процесса сохраняется, процесс переводится в «Инициализация IO».
		if p.State != process.StateActive {
			return false
		}
		p.State = process.StateInitIO

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
		p.State = process.StateReady
		r.Planner.Add(p)

	case EventTerminate:
		// Завершение задания: выгрузка процесса.
		if p.State != process.StateActive {
			return false
		}
		r.Planner.Remove(p)
		r.Memory.Free(*p.Task)
		r.removeFromTable(p)
		p.State = process.StateNone
		r.Completed++

	default:
		return false
	}

	return true
}

// ----------------------------------------------------------------------------------------
// Terminate — подпрограмма завершения процесса, используемая процессором
// при выполнении команды завершения задания.
func (r *Regulator) Terminate(p *process.PSW) {
	r.Process(p, EventTerminate)
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
