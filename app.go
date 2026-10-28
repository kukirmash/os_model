package main

import (
	"context"
	stdos "os"
	"time"

	"os_model/internal/os"
	"os_model/internal/params"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App — мост между ядром модели и интерфейсом.
type App struct {
	ctx    context.Context
	system *os.System

	lastViewEmit time.Time
}

// NewApp создаёт объект приложения.
func NewApp() *App {
	return &App{}
}

// uiRefreshInterval — минимальный период отправки снимков в интерфейс.
const uiRefreshInterval = 50 * time.Millisecond

// startup вызывается Wails при запуске приложения.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Файл параметров читается один раз при запуске. При ошибке модель
	// продолжает работу со значениями по умолчанию, а причина выводится
	// в интерфейсе.
	path := stdos.Getenv("OS_MODEL_PARAMS")
	if path == "" {
		path = params.DefaultPath
	}
	set, err := params.ReadFile(path)
	loaded := err == nil
	if err != nil {
		set = params.Default()
	}

	a.system = &os.System{
		Params:       set,
		ParamsPath:   path,
		ParamsLoaded: loaded,
	}
	if err != nil {
		a.system.ParamsError = err.Error()
	}
	a.system.Init()

	// Индикация: каждый такт модели отправляем снимок состояния.
	a.system.OnTick = a.emitViewSnapshot

	// Управление: директивы оператора из интерфейса.
	wailsRuntime.EventsOn(ctx, "os:directive", a.handleDirective)

	go a.system.RunLoop()
}

// emitViewSnapshot отправляет снимок состояния в интерфейс
// (не чаще uiRefreshInterval; завершающий снимок — всегда).
func (a *App) emitViewSnapshot(sys *os.System) {
	if !sys.Quit && time.Since(a.lastViewEmit) < uiRefreshInterval {
		return
	}
	a.lastViewEmit = time.Now()
	wailsRuntime.EventsEmit(a.ctx, "os:update", sys.ViewSnapshot())
}

// handleDirective принимает директиву оператора из интерфейса.
func (a *App) handleDirective(data ...interface{}) {
	if len(data) == 0 {
		return
	}

	name, ok := data[0].(string)
	if !ok {
		return
	}

	directive, ok := directiveByName(name)
	if !ok {
		return
	}

	// Неблокирующая отправка: интерфейс не должен ждать ядро.
	select {
	case a.system.Directives <- directive:
	default:
	}
}

// directiveByName отображает имя директивы из интерфейса в директиву ядра.
func directiveByName(name string) (os.Directive, bool) {
	switch name {
	case "pause":
		return os.DirectivePause, true
	case "resume":
		return os.DirectiveResume, true
	case "speed-up":
		return os.DirectiveSpeedUp, true
	case "speed-down":
		return os.DirectiveSpeedDown, true
	case "quit":
		return os.DirectiveQuit, true
	default:
		return 0, false
	}
}
