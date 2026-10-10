// Package io моделирует подсистему ввода-вывода — процессоры ВВ.
//
// Количество процессоров ввода-вывода достаточно для работы без очередей
// к ним, поэтому каждая операция выполняется на отдельном (условном)
// процессоре ВВ. По окончании операции процессор ВВ публикует сигнал
// прерывания; состояния процессов изменяет только регулировщик.
package io

import (
	"os_model/internal/interrupt"
	"os_model/internal/process"
	"os_model/internal/regulator"
)

// ----------------------------------------------------------------------------------------
// Operation — операция ввода-вывода, выполняемая на процессоре ВВ.
type Operation struct {
	ProcessorID int          // номер процессора ВВ
	Process     *process.PSW // обслуживаемый процесс
	TicksLeft   int          // осталось тактов выполнения
	TotalTicks  int          // полная длительность операции

	initialized bool // такт инициализации пройден
	started     bool // процесс переведён в состояние «Блокирован»
}

// ----------------------------------------------------------------------------------------
// Processors обслуживает процессоры ввода-вывода.
type Processors struct {
	Operations []*Operation // выполняемые операции (занятые процессоры ВВ)

	// Regulator — регулировщик: единственный, кто изменяет состояния процессов.
	Regulator *regulator.Regulator

	// Interrupts — супервизор прерываний: приёмник сигналов от процессоров ВВ.
	Interrupts *interrupt.Supervisor

	nextProcessor int // счётчик номеров процессоров ВВ
}

// ----------------------------------------------------------------------------------------
// InitIO — подпрограмма инициализации операции ввода-вывода: выделяет
// процессор ВВ, сохраняет слово состояния процесса (через регулировщика)
// и переводит его в состояние «Инициализация IO».
func (m *Processors) InitIO(p *process.PSW, duration int) {
	m.nextProcessor++

	// Смена состояния — только через регулировщика.
	m.Regulator.Process(p, regulator.EventIORequest)

	m.Operations = append(m.Operations, &Operation{
		ProcessorID: m.nextProcessor,
		Process:     p,
		TicksLeft:   duration,
		TotalTicks:  duration,
	})
}

// ----------------------------------------------------------------------------------------
// TickIO — подпрограмма работы процессоров ввода-вывода: выполняет один такт
// каждой операции, уменьшает счётчики тактов и по окончании операции
// публикует сигнал прерывания. Состояния процессов не изменяются.
func (m *Processors) TickIO(now int) {
	busy := m.Operations[:0]
	for _, op := range m.Operations {
		// Такт, в котором операция инициализирована, целиком уходит
		// на инициализацию: процессор ВВ принимает операцию.
		if !op.initialized {
			op.initialized = true
			busy = append(busy, op)
			continue
		}

		// Первый такт обслуживания: «Инициализация IO» → «Блокирован».
		if !op.started {
			op.started = true
			m.Regulator.Process(op.Process, regulator.EventIOStart)
		}

		op.TicksLeft--
		if op.TicksLeft > 0 {
			busy = append(busy, op)
			continue
		}

		// Счётчик тактов равен нулю — процессор ВВ публикует прерывание.
		m.Interrupts.Post(interrupt.Signal{
			Type:     interrupt.IOComplete,
			Time:     now,
			Process:  op.Process,
			SourceID: op.ProcessorID,
		})
	}
	m.Operations = busy
}
