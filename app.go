package main

import (
	"context"
	"encoding/json"
	stdos "os"
	"time"

	"os_model/internal/os"
	"os_model/internal/params"
	"os_model/internal/process"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App — мост между ядром модели и интерфейсом.
type App struct {
	ctx    context.Context
	system *os.System

	seedOverride uint64
	lastViewEmit time.Time
}

// NewApp создаёт объект приложения. seedOverride = 0 означает использование
// значения seed из params.json.
func NewApp(seedOverride uint64) *App {
	return &App{seedOverride: seedOverride}
}

// ----------------------------------------------------------------------------------------
// directiveMessage — структура директивы, пришедшей из интерфейса.
type directiveMessage struct {
	Name       string `json:"name"`
	ProcessID  int    `json:"processId"`
	Size       int    `json:"size"`
	Commands   int    `json:"commands"`
	IOPercent  int    `json:"ioPercent"`
	Priority   int    `json:"priority"`
	MemorySize int    `json:"memorySize"`
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
	if a.seedOverride != 0 {
		set.Seed = a.seedOverride
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
	msg, ok := parseDirectiveMessage(data)
	if !ok {
		return
	}

	directive, ok := directiveFromMessage(msg)
	if !ok {
		return
	}

	// Неблокирующая отправка: интерфейс не должен ждать ядро.
	select {
	case a.system.Directives <- directive:
	default:
	}
}

// ----------------------------------------------------------------------------------------
// parseDirectiveMessage поддерживает строковое имя (старый формат)
// и JSON-объект с параметрами директивы.
func parseDirectiveMessage(data []interface{}) (directiveMessage, bool) {
	if len(data) == 0 {
		return directiveMessage{}, false
	}
	if name, ok := data[0].(string); ok {
		return directiveMessage{Name: name}, name != ""
	}

	raw, err := json.Marshal(data[0])
	if err != nil {
		return directiveMessage{}, false
	}
	var msg directiveMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return directiveMessage{}, false
	}
	return msg, msg.Name != ""
}

// ----------------------------------------------------------------------------------------
// directiveFromMessage преобразует сообщение интерфейса в директиву ядра.
func directiveFromMessage(msg directiveMessage) (os.Directive, bool) {
	switch msg.Name {
	case "pause":
		return os.Directive{Kind: os.DirectivePause}, true
	case "resume":
		return os.Directive{Kind: os.DirectiveResume}, true
	case "speed-up":
		return os.Directive{Kind: os.DirectiveSpeedUp}, true
	case "speed-down":
		return os.Directive{Kind: os.DirectiveSpeedDown}, true
	case "quit":
		return os.Directive{Kind: os.DirectiveQuit}, true
	case "autoload-pause":
		return os.Directive{Kind: os.DirectiveAutoLoadPause}, true
	case "autoload-resume":
		return os.Directive{Kind: os.DirectiveAutoLoadResume}, true
	case "load-task":
		return os.Directive{Kind: os.DirectiveLoadTask}, true
	case "set-next-task":
		return os.Directive{
			Kind: os.DirectiveSetNextTask,
			Task: &process.Task{
				Size:          msg.Size,
				TotalCommands: msg.Commands,
				IOPercent:     msg.IOPercent,
				BasePriority:  msg.Priority,
			},
		}, true
	case "clear-next-task":
		return os.Directive{Kind: os.DirectiveClearNextTask}, true
	case "suspend-process":
		return os.Directive{Kind: os.DirectiveSuspendProcess, ProcessID: msg.ProcessID}, true
	case "resume-process":
		return os.Directive{Kind: os.DirectiveResumeProcess, ProcessID: msg.ProcessID}, true
	case "destroy-process":
		return os.Directive{Kind: os.DirectiveDestroyProcess, ProcessID: msg.ProcessID}, true
	case "finish-and-quit":
		return os.Directive{Kind: os.DirectiveFinishAndQuit}, true
	case "set-memory":
		return os.Directive{Kind: os.DirectiveSetMemory, MemorySize: msg.MemorySize}, true
	default:
		return os.Directive{}, false
	}
}
