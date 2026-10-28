// ============================================================================
// Модель ОС — интерфейс (вариант 11).
//
// Обмен с ядром идёт через события Wails:
//   os:update    — снимок состояния из ядра (Go → JS);
//   os:directive — управляющие директивы оператора (JS → Go).
//
// Все имена, относящиеся к отображению, имеют префикс UI_ (константы)
// или ui (функции и переменные), чтобы не путать данные интерфейса
// с полями ядра модели.
// ============================================================================

"use strict";

const UI_STATE_NAMES = ["Отсутствует", "Готов", "Активен", "Инициализация IO", "Блокирован", "Приостановлен"];
const UI_STATE_CLASSES = ["st-none", "st-ready", "st-active", "st-io-init", "st-blocked", "st-suspended"];
const UI_CPU_NAMES = ["Ожидание", "Работа"];
const UI_OP_NAMES = ["+", "−", "×", "÷"];
const UI_INTERRUPT_NAMES = ["окончание ВВ", "таймер"];

const uiById = (id) => document.getElementById(id);

let uiPaused = false;
let uiFinished = false;
let uiLastView = null;
let uiSelectedProcessId = 0;

// ----------------------------------------------------------------------------
// Отправка директив в ядро
// ----------------------------------------------------------------------------

function uiSendDirective(name, payload = {}) {
  if (window.runtime) {
    window.runtime.EventsEmit("os:directive", { name, ...payload });
  }
}

uiById("btn-pause").addEventListener("click", () => uiSendDirective(uiPaused ? "resume" : "pause"));
uiById("btn-slower").addEventListener("click", () => uiSendDirective("speed-down"));
uiById("btn-faster").addEventListener("click", () => uiSendDirective("speed-up"));
uiById("btn-quit").addEventListener("click", () => {
  if (!uiFinished && confirm("Выключить питание?")) {
    uiSendDirective("quit");
  }
});

uiById("btn-autoload").addEventListener("click", () => {
  const enabled = uiLastView?.autoLoadEnabled;
  uiSendDirective(enabled ? "autoload-pause" : "autoload-resume");
});

uiById("btn-load-task").addEventListener("click", () => uiSendDirective("load-task"));

uiById("btn-finish").addEventListener("click", () => {
  if (!uiFinished && confirm("Выполнить загруженные задания и выключить питание?")) {
    uiSendDirective("finish-and-quit");
  }
});

uiById("btn-apply-task").addEventListener("click", () => {
  const task = {
    size: Number(uiById("dir-task-size").value),
    commands: Number(uiById("dir-task-commands").value),
    ioPercent: Number(uiById("dir-task-io").value),
    priority: Number(uiById("dir-task-priority").value),
  };
  if (!task.size || !task.commands || !task.priority) {
    alert("Заполните размер, число команд и приоритет.");
    return;
  }
  if (task.ioPercent < 0 || task.ioPercent > 100) {
    alert("Процент команд ВВ должен быть в диапазоне 0..100.");
    return;
  }
  uiSendDirective("set-next-task", task);
});

uiById("btn-clear-task").addEventListener("click", () => uiSendDirective("clear-next-task"));

uiById("btn-set-memory").addEventListener("click", () => {
  const size = Number(uiById("dir-memory").value);
  if (!size || size <= 0) {
    alert("Введите положительный размер памяти.");
    return;
  }
  uiSendDirective("set-memory", { memorySize: size });
});

uiById("btn-suspend").addEventListener("click", () => {
  if (uiSelectedProcessId) {
    uiSendDirective("suspend-process", { processId: uiSelectedProcessId });
  }
});

uiById("btn-resume").addEventListener("click", () => {
  if (uiSelectedProcessId) {
    uiSendDirective("resume-process", { processId: uiSelectedProcessId });
  }
});

uiById("btn-destroy").addEventListener("click", () => {
  if (uiSelectedProcessId && confirm(`Уничтожить процесс #${uiSelectedProcessId}?`)) {
    uiSendDirective("destroy-process", { processId: uiSelectedProcessId });
  }
});

uiById("proc-table").addEventListener("click", (event) => {
  const row = event.target.closest("tr[data-id]");
  if (!row) {
    return;
  }
  uiSelectedProcessId = Number(row.dataset.id);
  if (uiLastView) {
    uiRenderProcesses(uiLastView);
    uiRenderDirectives(uiLastView);
  }
});

window.addEventListener("keydown", (event) => {
  if (uiFinished || event.repeat) {
    return;
  }

  const tag = event.target?.tagName;
  if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") {
    return;
  }

  if (event.code === "Space") {
    event.preventDefault();
    uiSendDirective(uiPaused ? "resume" : "pause");
    return;
  }

  if (event.key === "+" || event.key === "=") {
    uiSendDirective("speed-up");
    return;
  }

  if (event.key === "-" || event.key === "_") {
    uiSendDirective("speed-down");
    return;
  }

  if (event.key === "a" || event.key === "A" || event.key === "ф" || event.key === "Ф") {
    const enabled = uiLastView?.autoLoadEnabled;
    uiSendDirective(enabled ? "autoload-pause" : "autoload-resume");
    return;
  }

  if (event.key === "l" || event.key === "L" || event.key === "д" || event.key === "Д") {
    uiSendDirective("load-task");
    return;
  }

  if (event.key === "p" || event.key === "P" || event.key === "з" || event.key === "З") {
    const process = selectedProcess();
    if (!process) {
      return;
    }
    if (process.state === 5) {
      uiSendDirective("resume-process", { processId: process.id });
    } else {
      uiSendDirective("suspend-process", { processId: process.id });
    }
    return;
  }

  if (event.key === "Delete") {
    if (uiSelectedProcessId && confirm(`Уничтожить процесс #${uiSelectedProcessId}?`)) {
      uiSendDirective("destroy-process", { processId: uiSelectedProcessId });
    }
    return;
  }

  if (event.key === "f" || event.key === "F" || event.key === "а" || event.key === "А") {
    if (!uiFinished && confirm("Выполнить загруженные задания и выключить питание?")) {
      uiSendDirective("finish-and-quit");
    }
    return;
  }

  if ("qQйЙ".includes(event.key)) {
    if (confirm("Выключить питание?")) {
      uiSendDirective("quit");
    }
  }
});

// ----------------------------------------------------------------------------
// Вспомогательные функции отображения
// ----------------------------------------------------------------------------

function uiSetText(id, value) {
  const el = uiById(id);
  if (el) {
    el.textContent = value;
  }
}

// uiFormatInt — целое число в русской локали; uiFormat1 — с одним знаком.
function uiFormatInt(value) {
  return Number(value ?? 0).toLocaleString("ru-RU", { maximumFractionDigits: 0 });
}

function uiFormat1(value) {
  return Number(value ?? 0).toLocaleString("ru-RU", {
    minimumFractionDigits: 1,
    maximumFractionDigits: 1,
  });
}

function uiStateMark(state) {
  const name = UI_STATE_NAMES[state] ?? "?";
  const cls = UI_STATE_CLASSES[state] ?? "st-none";
  return `<span class="state ${cls}">${name}</span>`;
}

function selectedProcess() {
  if (!uiLastView || !uiSelectedProcessId) {
    return null;
  }
  return (uiLastView.processes ?? []).find((p) => p.id === uiSelectedProcessId) || null;
}

function uiCommandText(view) {
  if (!view.activeId) {
    return "—";
  }

  switch (view.commandType) {
    case 0:
      return `вычислительная: ${UI_OP_NAMES[view.commandCode] ?? "?"} (A1=${view.commandAddr1}, A2=${view.commandAddr2})`;
    case 1:
      return `ввод-вывод: ${view.commandDuration} такт.`;
    case 2:
      return "завершение задания";
    default:
      return "—";
  }
}

// uiRenderRows отрисовывает таблицу или заглушку «нет данных», чтобы панель
// не «дёргалась» при изменении числа строк.
function uiRenderRows(tableId, rows, emptyId, rowBuilder) {
  const tbody = uiById(tableId).querySelector("tbody");
  tbody.innerHTML = rows.map(rowBuilder).join("");

  uiById(tableId).parentElement.style.display = rows.length ? "" : "none";
  uiById(emptyId).style.display = rows.length ? "none" : "flex";
}

// ----------------------------------------------------------------------------
// Разделы индикации
// ----------------------------------------------------------------------------

function uiRenderHeader(view) {
  uiPaused = view.paused;
  uiFinished = view.quit;

  uiSetText("m-time", uiFormatInt(view.time));
  uiSetText("m-speed", uiFormat1(view.speed));

  const status = uiById("m-status");
  if (view.quit) {
    status.textContent = "Завершено";
    status.className = "state st-none";
  } else if (view.finishAndQuit) {
    status.textContent = "Завершение";
    status.className = "state st-blocked";
  } else if (view.paused) {
    status.textContent = "Пауза";
    status.className = "state st-blocked";
  } else {
    status.textContent = "Работа";
    status.className = "state st-active";
  }

  uiById("btn-pause").textContent = view.paused ? "Продолжить" : "Пауза";
  uiById("btn-pause").disabled = view.quit;
  uiById("btn-slower").disabled = view.quit;
  uiById("btn-faster").disabled = view.quit;
  uiById("btn-quit").disabled = view.quit;
}

function uiRenderSystem(view) {
  const stats = view.stats ?? {};
  const free = view.memoryTotal - view.memoryUsed;
  const percent = view.memoryTotal
    ? Math.round((view.memoryUsed / view.memoryTotal) * 100)
    : 0;

  uiSetText("sys-generated", uiFormatInt(view.generated));
  uiSetText("sys-completed", `${uiFormatInt(view.completed)} / ${uiFormatInt(stats.destroyedCount)}`);
  uiSetText("sys-table", `${view.processes.length} / ${view.maxProcesses}`);
  uiSetText("sys-int", uiFormatInt(view.interruptsHandled));
  uiSetText(
    "sys-ready",
    `${uiFormatInt(stats.readyLen)} / ${uiFormat1(stats.readyAvg)} / ${uiFormatInt(stats.readyMax)}`
  );
  uiSetText(
    "sys-io",
    `${uiFormatInt(stats.ioBusy)} / ${uiFormat1(stats.ioBusyAvg)} / ${uiFormatInt(stats.ioBusyMax)}`
  );
  uiSetText("sys-throughput", uiFormat1(stats.throughput));

  uiById("mem-bar").style.width = percent + "%";
  uiSetText("mem-percent", percent + " %");
  uiSetText("mem-used", uiFormatInt(view.memoryUsed));
  uiSetText("mem-free", uiFormatInt(free));
  uiSetText("mem-total", uiFormatInt(view.memoryTotal));
}

function uiRenderCPU(view) {
  const stats = view.stats ?? {};

  const cpuState = uiById("cpu-state");
  cpuState.textContent = UI_CPU_NAMES[view.cpuState] ?? "—";
  cpuState.className = "state " + (view.cpuState === 1 ? "st-active" : "st-none");

  uiSetText("cpu-active", view.activeId ? `#${view.activeId}` : "—");
  uiSetText("cpu-command", uiCommandText(view));
  uiSetText("cpu-busy", `${uiFormatInt(stats.cpuBusy)} / ${uiFormatInt(stats.cpuIdle)}`);
  uiSetText("cpu-load", uiFormat1(stats.cpuUtilization) + " %");

  const load = Math.max(0, Math.min(100, stats.cpuUtilization ?? 0));
  uiById("cpu-load-bar").style.width = load + "%";
}

function uiRenderTask(view) {
  const task = view.lastTask ?? {};
  // Задание может быть ещё не загружено (нет места в таблице PSW или
  // памяти) — тогда выводится заявка без номера и со статусом ошибки.
  const hasTask = task.size > 0 || task.totalCommands > 0;
  const status = uiById("task-status");

  if (!hasTask) {
    status.textContent = "—";
    status.className = "state st-none";
    status.removeAttribute("title");
  } else if (task.loaded) {
    status.textContent = "Загружено";
    status.className = "state st-active";
    status.removeAttribute("title");
  } else {
    status.textContent = "Не загружено";
    status.className = "state st-blocked";
    status.title = task.reason ?? "";
  }

  const reason = uiById("task-reason");
  if (hasTask && !task.loaded && task.reason) {
    reason.textContent = "Причина: " + task.reason;
    reason.hidden = false;
  } else {
    reason.textContent = "";
    reason.hidden = true;
  }

  uiSetText("task-id", task.id ? `#${task.id}` : "заявка");
  uiSetText("task-size", hasTask ? uiFormatInt(task.size) : "—");
  uiSetText("task-commands", hasTask ? uiFormatInt(task.totalCommands) : "—");
  uiSetText("task-io", hasTask ? task.ioPercent + " %" : "—");
  uiSetText("task-priority", hasTask ? uiFormatInt(task.basePriority) : "—");

  const tp = view.taskParams ?? {};
  uiSetText(
    "task-ranges",
    `Диапазоны генерации: размер ${tp.sizeMin}–${tp.sizeMax} (шаг ${tp.sizeStep}) ед. · команд ${tp.commandsMin}–${tp.commandsMax} (шаг ${tp.commandsStep}) · ВВ ${tp.ioPercentMin}–${tp.ioPercentMax} % (шаг ${tp.ioPercentStep} %) · приоритет ${tp.priorityMin}–${tp.priorityMax} (шаг ${tp.priorityStep})`
  );
}

function uiRenderParams(view) {
  const info = view.params ?? {};
  const rows = info.rows ?? [];
  const status = uiById("params-status");

  if (info.loaded) {
    status.textContent = "Загружен";
    status.className = "state st-active";
  } else {
    status.textContent = "По умолчанию";
    status.className = "state st-blocked";
  }

  const error = info.error ? ` · ${info.error}` : "";
  uiSetText("params-file", `Файл: ${info.file ?? "—"} · seed: ${info.seed ?? 0}${error}`);

  const tbody = uiById("params-table").querySelector("tbody");
  tbody.innerHTML = rows.map((r) => `
    <tr>
      <td>${r.name}</td>
      <td class="mono">${r.min}</td>
      <td class="mono">${r.max}</td>
      <td class="mono">${r.step}</td>
      <td>${r.comment || "—"}</td>
    </tr>`).join("");
}

function uiRenderDirectives(view) {
  const process = selectedProcess();
  if (uiSelectedProcessId && !process) {
    uiSelectedProcessId = 0;
  }

  const autoLoad = Boolean(view.autoLoadEnabled);
  uiSetText("dir-autoload", autoLoad ? "Автозагрузка: вкл" : "Автозагрузка: выкл");
  uiSetText("btn-autoload", autoLoad ? "Приостановить автозагрузку" : "Продолжить автозагрузку");

  const next = view.nextTask;
  uiSetText(
    "dir-next-task",
    next
      ? `Следующее задание: размер ${next.size}, команд ${next.totalCommands}, ВВ ${next.ioPercent} %, приоритет ${next.basePriority}`
      : "Переопределение не задано — параметры берутся из диапазонов"
  );

  const selected = uiById("dir-selected");
  if (process) {
    selected.textContent = `#${process.id}: ${UI_STATE_NAMES[process.state] ?? "?"}`;
  } else {
    selected.textContent = "Процесс не выбран";
  }

  uiById("btn-suspend").disabled = !process || (process.state !== 1 && process.state !== 2);
  uiById("btn-resume").disabled = !process || process.state !== 5;
  uiById("btn-destroy").disabled = !process;
  uiById("btn-autoload").disabled = Boolean(view.quit);
  uiById("btn-load-task").disabled = Boolean(view.quit);
  uiById("btn-finish").disabled = Boolean(view.quit) || Boolean(view.finishAndQuit);
  uiById("btn-apply-task").disabled = Boolean(view.quit);
  uiById("btn-clear-task").disabled = Boolean(view.quit);
  uiById("btn-set-memory").disabled = Boolean(view.quit);

  const directive = view.lastDirective ?? {};
  const status = directive.message
    ? `${directive.time} такт · ${directive.name}: ${directive.message}`
    : "Результат директивы: —";
  const message = uiById("dir-message");
  message.textContent = status;
  message.className = directive.message && !directive.ok ? "hint hint-error" : "hint";
}

function uiRenderAverages(view) {
  const stats = view.stats ?? {};

  uiSetText("avg-loaded-count", uiFormatInt(stats.loadedCount) + " загружено");
  uiSetText("avg-size", uiFormat1(stats.avgSize));
  uiSetText("avg-commands", uiFormat1(stats.avgCommands));
  uiSetText("avg-io-percent", uiFormat1(stats.avgIoPercent) + " %");
  uiSetText("avg-priority", uiFormat1(stats.avgPriority));

  uiSetText("avg-completed-count", uiFormatInt(stats.completedCount) + " завершено");
  uiSetText("avg-wait", uiFormat1(stats.avgWait));
  uiSetText("avg-exec", uiFormat1(stats.avgExec));
  uiSetText("avg-io", uiFormat1(stats.avgIo));
  uiSetText("avg-turnaround", uiFormat1(stats.avgTurnaround));
}

function uiRenderComparison(view) {
  const stats = view.stats ?? {};

  uiSetText(
    "cmp-mono",
    `${uiFormatInt(stats.sumMono)} / ${uiFormat1(stats.avgMono)}`
  );
  uiSetText("cmp-multi", uiFormat1(stats.avgMulti));
  uiSetText(
    "cmp-time",
    `${uiFormatInt(stats.tModel)} / ${uiFormatInt(stats.tSystem)} / ${uiFormatInt(stats.tEffective)}`
  );
  uiSetText(
    "cmp-tasks",
    `${uiFormat1(stats.mMono)} / ${uiFormatInt(stats.mMulti)}`
  );
  uiSetText("cmp-performance", uiFormat1(stats.performance) + " %");
  uiSetText("cmp-system-share", uiFormat1(stats.systemShare) + " %");
  uiSetText("cmp-avg-dexe", uiFormat1(stats.avgDExe) + " %");
  uiSetText("cmp-avg-dready", uiFormat1(stats.avgDReady) + " %");
}

function uiRenderCosts(view) {
  const costs = view.costs ?? {};
  const stats = view.stats ?? {};

  uiSetText("cost-load", `${uiFormatInt(costs.load)} такт.`);
  uiSetText("cost-dispatch", `${uiFormatInt(costs.dispatch)} такт.`);
  uiSetText("cost-io-init", `${uiFormatInt(costs.ioInit)} такт.`);
  uiSetText("cost-io-int", `${uiFormatInt(costs.ioInterrupt)} такт.`);
  uiSetText("cost-term", `${uiFormatInt(costs.terminate)} такт.`);
  uiSetText("cost-total", `${uiFormatInt(stats.tSystem)} такт.`);
}

function uiRenderHistory(view) {
  const history = (view.stats ?? {}).execHistory ?? [];
  const box = uiById("exec-history");

  if (!history.length) {
    box.innerHTML = `<span class="empty-steps">процессы на ЦПр ещё не выбирались</span>`;
  } else {
    box.innerHTML = history
      .map((id, index) =>
        index === history.length - 1
          ? `<span class="step current">${id}</span>`
          : `<span class="step">${id}</span>`
      )
      .join("");
  }

  uiSetText("history-count", `${history.length} / 16`);
}

function uiRenderReady(view) {
  const rows = view.ready ?? [];
  uiSetText("ready-count", uiFormatInt(rows.length));

  uiRenderRows("ready-table", rows, "ready-empty", (p) => `
    <tr>
      <td class="mono">#${p.id}</td>
      <td>${uiStateMark(p.state)}</td>
      <td class="mono">${p.pc}</td>
      <td class="mono">${p.size}</td>
      <td class="mono">${p.dynamicPriority}</td>
    </tr>`);
}

function uiRenderIO(view) {
  const rows = view.operations ?? [];
  uiSetText("io-count", uiFormatInt(rows.length));

  uiRenderRows("io-table", rows, "io-empty", (op) => {
    const done = op.totalTicks > 0
      ? Math.round(((op.totalTicks - op.ticksLeft) / op.totalTicks) * 100)
      : 0;
    return `
    <tr>
      <td class="mono">#${op.processorId}</td>
      <td class="mono">#${op.processId}</td>
      <td>${uiStateMark(op.state)}</td>
      <td class="mono">${op.ticksLeft} / ${op.totalTicks}</td>
      <td class="mono">${done} %</td>
    </tr>`;
  });
}

function uiRenderInterrupts(view) {
  const rows = [...(view.interruptLog ?? [])].reverse();
  uiSetText("int-count", uiFormatInt(view.interruptsHandled));

  uiRenderRows("int-table", rows, "int-empty", (s) => `
    <tr>
      <td class="mono">${s.time}</td>
      <td>${UI_INTERRUPT_NAMES[s.type] ?? "?"}</td>
      <td class="mono">ВВ #${s.sourceId}</td>
      <td class="mono">#${s.processId}</td>
    </tr>`);
}

function uiRenderProcesses(view) {
  const rows = view.processes ?? [];
  uiSetText("proc-count", `${rows.length} / ${view.maxProcesses}`);

  uiRenderRows("proc-table", rows, "proc-empty", (p) => `
    <tr data-id="${p.id}" class="${p.state === 2 ? "row-active" : ""} ${p.id === uiSelectedProcessId ? "row-selected" : ""}">
      <td class="mono">#${p.id}</td>
      <td>${uiStateMark(p.state)}</td>
      <td class="mono">${p.pc}</td>
      <td class="mono">${p.totalCommands}</td>
      <td class="mono">${p.size}</td>
      <td class="mono">${p.ioPercent} %</td>
      <td class="mono">${p.basePriority} → ${p.dynamicPriority}</td>
      <td class="mono">${p.waitTicks}</td>
      <td class="mono">${p.execTicks}</td>
      <td class="mono">${p.ioTicks}</td>
      <td class="mono">${p.suspendedTicks}</td>
      <td class="mono">${p.turnaround}</td>
    </tr>`);
}

function uiRenderProcessComparison(view) {
  const rows = view.processes ?? [];
  uiSetText("cmp-proc-count", `${rows.length} / ${view.maxProcesses}`);

  uiRenderRows("cmp-proc-table", rows, "cmp-proc-empty", (p) => `
    <tr class="${p.state === 2 ? "row-active" : ""}">
      <td class="mono">#${p.id}</td>
      <td class="mono">${p.tMono}</td>
      <td class="mono">${p.tMulti}</td>
      <td class="mono">${uiFormat1(p.dExe)}</td>
      <td class="mono">${p.tReady}</td>
      <td class="mono">${uiFormat1(p.dReady)}</td>
      <td class="mono">${p.switchTicks}</td>
      <td class="mono">${p.ioInitTicks}</td>
      <td class="mono">${p.ioIntrTicks}</td>
      <td class="mono">${p.systemWaitTicks}</td>
    </tr>`);
}

// ----------------------------------------------------------------------------
// Отрисовка снимка состояния
// ----------------------------------------------------------------------------

function uiRenderView(view) {
  uiLastView = view;
  uiRenderHeader(view);
  uiRenderSystem(view);
  uiRenderCPU(view);
  uiRenderTask(view);
  uiRenderParams(view);
  uiRenderDirectives(view);
  uiRenderAverages(view);
  uiRenderComparison(view);
  uiRenderCosts(view);
  uiRenderHistory(view);
  uiRenderReady(view);
  uiRenderIO(view);
  uiRenderInterrupts(view);
  uiRenderProcesses(view);
  uiRenderProcessComparison(view);
}

// ----------------------------------------------------------------------------
// Приём снимков состояния из ядра
// ----------------------------------------------------------------------------

if (window.runtime) {
  window.runtime.EventsOn("os:update", uiRenderView);
} else {
  console.warn("Wails runtime не найден — запустите приложение через wails dev / wails build");
}
