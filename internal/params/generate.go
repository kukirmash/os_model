package params

import "math/rand/v2"

// ----------------------------------------------------------------------------------------
// Generate — подпрограмма «Генерация значения параметра».
// Возвращает случайное значение из диапазона [Min, Max] с шагом Step.
func (p Param) Generate(r *rand.Rand) int {
	if p.Step <= 0 || p.Max <= p.Min || r == nil {
		return p.Min
	}

	count := (p.Max-p.Min)/p.Step + 1
	if count <= 0 {
		return p.Min
	}
	return p.Min + r.IntN(count)*p.Step
}

// ----------------------------------------------------------------------------------------
// GenerateSystem — подпрограмма «Генерация параметров системы».
// Генерирует фактические значения системных параметров по диапазонам.
func (s Set) GenerateSystem(r *rand.Rand) SystemValues {
	return SystemValues{
		MemorySize:     s.System.MemorySize.Generate(r),
		TickDurationMs: s.System.TickDurationMs.Generate(r),
		IOMaxDuration:  s.System.IOMaxDuration.Generate(r),
		Costs: CostsValues{
			Load:        s.System.Costs.Load.Generate(r),
			Dispatch:    s.System.Costs.Dispatch.Generate(r),
			IOInit:      s.System.Costs.IOInit.Generate(r),
			IOInterrupt: s.System.Costs.IOInterrupt.Generate(r),
			Terminate:   s.System.Costs.Terminate.Generate(r),
		},
	}
}

// ----------------------------------------------------------------------------------------
// GenerateTask — обновлённая подпрограмма «Генерация нового задания».
// Генерирует параметры задания по диапазонам из файла.
func (t Task) GenerateTask(r *rand.Rand) TaskValues {
	return TaskValues{
		Size:         t.Size.Generate(r),
		Commands:     t.Commands.Generate(r),
		IOPercent:    t.IOPercent.Generate(r),
		BasePriority: t.BasePriority.Generate(r),
	}
}
