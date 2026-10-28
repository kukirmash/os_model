// Команда osmodel — точка входа модели операционной системы (вариант 11).
package main

import (
	"flag"
	"log"

	"os_model/internal/os"
	"os_model/internal/params"
)

func main() {
	path := flag.String("params", params.DefaultPath, "JSON-файл параметров модели")
	flag.Parse()

	set, err := params.ReadFile(*path)
	loaded := err == nil
	if err != nil {
		log.Printf("файл параметров не загружен: %v; используются значения по умолчанию", err)
		set = params.Default()
	}

	system := &os.System{
		Params:       set,
		ParamsPath:   *path,
		ParamsLoaded: loaded,
	}
	if err != nil {
		system.ParamsError = err.Error()
	}

	// Запуск модели. Работает до получения директивы завершения
	// (или прерывается по Ctrl+C).
	system.Init()
	system.RunLoop()
}
