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

const UI_STATE_NAMES = ["Отсутствует", "Готов", "Активен", "Инициализация IO", "Блокирован"];
const UI_STATE_CLASSES = ["st-none", "st-ready", "st-active", "st-io-init", "st-blocked"];
const UI_CPU_NAMES = ["Ожидание", "Работа"];
const UI_OP_NAMES = ["+", "−", "×", "÷"];
const UI_INTERRUPT_NAMES = ["окончание ВВ", "таймер"];

const uiById = (id) => document.getElementById(id);

let uiPaused = false;
let uiFinished = false;

// ----------------------------------------------------------------------------
// Отправка директив в ядро
// ----------------------------------------------------------------------------

function uiSendDirective(name) {
  if (window.runtime) {
    window.runtime.EventsEmit("os:directive", name);
  }
}

uiById("btn-pause").addEventListener("click", () => uiSendDirective(uiPaused ? "resume" : "pause"));
uiById("btn-slower").addEventListener("click", () => uiSendDirective("speed-down"));
uiById("btn-faster").addEventListener("click", () => uiSendDirective("speed-up"));
uiById("btn-quit").addEventListener("click", () => {
  if (!uiFinished && confirm("Завершить моделирование?")) {
    uiSendDirective("quit");
  }
});

window.addEventListener("keydown", (event) => {
  if (uiFinished || event.repeat) {
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

  if ("qQйЙ".includes(event.key) && confirm("Завершить моделирование?")) {
    uiSendDirective("quit");
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
  uiSetText("sys-completed", uiFormatInt(view.completed));
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
    `Диапазоны генерации: размер ${tp.sizeMin}–${tp.sizeMax} ед. · команд ${tp.commandsMin}–${tp.commandsMax} · ВВ ${tp.ioPercentMin}–${tp.ioPercentMax} % · приоритет ${tp.priorityMin}–${tp.priorityMax}`
  );
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
    <tr class="${p.state === 2 ? "row-active" : ""}">
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
  uiRenderHeader(view);
  uiRenderSystem(view);
  uiRenderCPU(view);
  uiRenderTask(view);
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
