package main

import (
	"context"
	"time"

	"os_model/internal/os"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App — мост между ядром модели и интерфейсом.
type App struct {
	ctx    context.Context
	system *os.System

	lastEmit time.Time
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

	a.system = &os.System{
		Config: os.Config{
			MemorySize:     8192,
			TickDurationMs: 100,
			IOMaxDuration:  10,
		},
	}
	a.system.Init()

	// Индикация: каждый такт модели отправляем снимок состояния.
	a.system.OnTick = a.emitSnapshot

	// Управление: директивы оператора из интерфейса.
	wailsRuntime.EventsOn(ctx, "os:directive", a.handleDirective)

	go a.system.RunLoop()
}

// emitSnapshot отправляет снимок состояния в интерфейс
// (не чаще uiRefreshInterval; завершающий снимок — всегда).
func (a *App) emitSnapshot(sys *os.System) {
	if !sys.Quit && time.Since(a.lastEmit) < uiRefreshInterval {
		return
	}
	a.lastEmit = time.Now()
	wailsRuntime.EventsEmit(a.ctx, "os:update", sys.Snapshot())
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
