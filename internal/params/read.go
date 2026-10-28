package params

import (
	"encoding/json"
	"fmt"
	"os"
)

// ----------------------------------------------------------------------------------------
// ReadFile читает JSON-файл параметров. Файл должен читаться один раз при
// запуске модели; повторное чтение при генерации заданий не выполняется.
func ReadFile(path string) (Set, error) {
	file, err := os.Open(path)
	if err != nil {
		return Set{}, fmt.Errorf("открыть файл параметров %q: %w", path, err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()

	var set Set
	if err := decoder.Decode(&set); err != nil {
		return Set{}, fmt.Errorf("разобрать файл параметров %q: %w", path, err)
	}

	if set.Seed == 0 {
		set.Seed = DefaultSeed
	}
	if err := set.Validate(); err != nil {
		return Set{}, fmt.Errorf("некорректные параметры в %q: %w", path, err)
	}

	return set, nil
}
