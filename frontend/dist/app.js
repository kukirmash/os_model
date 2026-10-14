// ============================================================================
// Модель ОС — интерфейс (вариант 11).
//
// Обмен с ядром идёт через события Wails:
//   os:update    — снимок состояния из ядра (Go → JS);
//   os:directive — управляющие директивы оператора (JS → Go).
//
// Разделы индикации соответствуют лабораторной работе 6: параметры системы,
// параметры нового задания, параметры центрального процессора, параметры
// процессов, последовательность выполнения процессов и средние значения.
// ============================================================================

"use strict";

const STATE_NAMES = ["Отсутствует", "Готов", "Активен", "Инициализация IO", "Блокирован"];
const STATE_CLASSES = ["st-none", "st-ready", "st-active", "st-io-init", "st-blocked"];
const CPU_NAMES = ["Ожидание", "Работа"];
const OP_NAMES = ["+", "−", "×", "÷"];
const INTERRUPT_NAMES = ["окончание ВВ", "таймер"];

const $ = (id) => document.getElementById(id);

let paused = false;
let finished = false;

// ----------------------------------------------------------------------------
// Отправка директив в ядро
// ----------------------------------------------------------------------------

function send(name) {
  if (window.runtime) {
    window.runtime.EventsEmit("os:directive", name);
  }
}

$("btn-pause").addEventListener("click", () => send(paused ? "resume" : "pause"));
$("btn-slower").addEventListener("click", () => send("speed-down"));
$("btn-faster").addEventListener("click", () => send("speed-up"));
$("btn-quit").addEventListener("click", () => {
  if (!finished && confirm("Завершить моделирование?")) {
    send("quit");
  }
});

window.addEventListener("keydown", (event) => {
  if (finished || event.repeat) {
    return;
  }

  if (event.code === "Space") {
    event.preventDefault();
    send(paused ? "resume" : "pause");
    return;
  }

  if (event.key === "+" || event.key === "=") {
    send("speed-up");
    return;
  }

  if (event.key === "-" || event.key === "_") {
    send("speed-down");
    return;
  }

  if ("qQйЙ".includes(event.key) && confirm("Завершить моделирование?")) {
    send("quit");
  }
});

// ----------------------------------------------------------------------------
// Вспомогательные функции отображения
// ----------------------------------------------------------------------------

function setText(id, value) {
  const el = $(id);
  if (el) {
    el.textContent = value;
  }
}

// fmtInt — целое число в русской локали; fmt1 — число с одним знаком после запятой.
function fmtInt(value) {
  return Number(value ?? 0).toLocaleString("ru-RU", { maximumFractionDigits: 0 });
}

function fmt1(value) {
  return Number(value ?? 0).toLocaleString("ru-RU", {
    minimumFractionDigits: 1,
    maximumFractionDigits: 1,
  });
}

function stateMark(state) {
  const name = STATE_NAMES[state] ?? "?";
  const cls = STATE_CLASSES[state] ?? "st-none";
  return `<span class="state ${cls}">${name}</span>`;
}

function commandText(snapshot) {
  if (!snapshot.activeId) {
    return "—";
  }

  switch (snapshot.commandType) {
    case 0:
      return `вычислительная: ${OP_NAMES[snapshot.commandCode] ?? "?"} (A1=${snapshot.commandAddr1}, A2=${snapshot.commandAddr2})`;
    case 1:
      return `ввод-вывод: ${snapshot.commandDuration} такт.`;
    case 2:
      return "завершение задания";
    default:
      return "—";
  }
}

// renderRows отрисовывает таблицу или заглушку «нет данных», чтобы панель
// не «дёргалась» при изменении числа строк.
function renderRows(tableId, rows, emptyId, rowBuilder) {
  const tbody = $(tableId).querySelector("tbody");
  tbody.innerHTML = rows.map(rowBuilder).join("");

  $(tableId).parentElement.style.display = rows.length ? "" : "none";
  $(emptyId).style.display = rows.length ? "none" : "flex";
}

// ----------------------------------------------------------------------------
// Разделы индикации
// ----------------------------------------------------------------------------

function renderHeader(snapshot) {
  paused = snapshot.paused;
  finished = snapshot.quit;

  setText("m-time", fmtInt(snapshot.time));
  setText("m-speed", fmt1(snapshot.speed));

  const status = $("m-status");
  if (snapshot.quit) {
    status.textContent = "Завершено";
    status.className = "state st-none";
  } else if (snapshot.paused) {
    status.textContent = "Пауза";
    status.className = "state st-blocked";
  } else {
    status.textContent = "Работа";
    status.className = "state st-active";
  }

  $("btn-pause").textContent = snapshot.paused ? "Продолжить" : "Пауза";
  $("btn-pause").disabled = snapshot.quit;
  $("btn-slower").disabled = snapshot.quit;
  $("btn-faster").disabled = snapshot.quit;
  $("btn-quit").disabled = snapshot.quit;
}

function renderSystem(snapshot) {
  const stats = snapshot.stats ?? {};
  const free = snapshot.memoryTotal - snapshot.memoryUsed;
  const percent = snapshot.memoryTotal
    ? Math.round((snapshot.memoryUsed / snapshot.memoryTotal) * 100)
    : 0;

  setText("sys-generated", fmtInt(snapshot.generated));
  setText("sys-completed", fmtInt(snapshot.completed));
  setText("sys-table", `${snapshot.processes.length} / ${snapshot.maxProcesses}`);
  setText("sys-int", fmtInt(snapshot.interruptsHandled));
  setText(
    "sys-ready",
    `${fmtInt(stats.readyLen)} / ${fmt1(stats.readyAvg)} / ${fmtInt(stats.readyMax)}`
  );
  setText(
    "sys-io",
    `${fmtInt(stats.ioBusy)} / ${fmt1(stats.ioBusyAvg)} / ${fmtInt(stats.ioBusyMax)}`
  );
  setText("sys-throughput", fmt1(stats.throughput));

  $("mem-bar").style.width = percent + "%";
  setText("mem-percent", percent + " %");
  setText("mem-used", fmtInt(snapshot.memoryUsed));
  setText("mem-free", fmtInt(free));
  setText("mem-total", fmtInt(snapshot.memoryTotal));
}

function renderCPU(snapshot) {
  const stats = snapshot.stats ?? {};

  const cpuState = $("cpu-state");
  cpuState.textContent = CPU_NAMES[snapshot.cpuState] ?? "—";
  cpuState.className = "state " + (snapshot.cpuState === 1 ? "st-active" : "st-none");

  setText("cpu-active", snapshot.activeId ? `#${snapshot.activeId}` : "—");
  setText("cpu-command", commandText(snapshot));
  setText("cpu-busy", `${fmtInt(stats.cpuBusy)} / ${fmtInt(stats.cpuIdle)}`);
  setText("cpu-load", fmt1(stats.cpuUtilization) + " %");

  const load = Math.max(0, Math.min(100, stats.cpuUtilization ?? 0));
  $("cpu-load-bar").style.width = load + "%";
}

function renderTask(snapshot) {
  const task = snapshot.lastTask ?? {};
  // Задание может быть ещё не загружено (нет места в таблице PSW или
  // памяти) — тогда выводится заявка без номера и со статусом ошибки.
  const hasTask = task.size > 0 || task.totalCommands > 0;
  const status = $("task-status");

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

  const reason = $("task-reason");
  if (hasTask && !task.loaded && task.reason) {
    reason.textContent = "Причина: " + task.reason;
    reason.hidden = false;
  } else {
    reason.textContent = "";
    reason.hidden = true;
  }

  setText("task-id", task.id ? `#${task.id}` : "заявка");
  setText("task-size", hasTask ? fmtInt(task.size) : "—");
  setText("task-commands", hasTask ? fmtInt(task.totalCommands) : "—");
  setText("task-io", hasTask ? task.ioPercent + " %" : "—");
  setText("task-priority", hasTask ? fmtInt(task.basePriority) : "—");

  const tp = snapshot.taskParams ?? {};
  setText(
    "task-ranges",
    `Диапазоны генерации: размер ${tp.sizeMin}–${tp.sizeMax} ед. · команд ${tp.commandsMin}–${tp.commandsMax} · ВВ ${tp.ioPercentMin}–${tp.ioPercentMax} % · приоритет ${tp.priorityMin}–${tp.priorityMax}`
  );
}

function renderAverages(snapshot) {
  const stats = snapshot.stats ?? {};

  setText("avg-loaded-count", fmtInt(stats.loadedCount) + " загружено");
  setText("avg-size", fmt1(stats.avgSize));
  setText("avg-commands", fmt1(stats.avgCommands));
  setText("avg-io-percent", fmt1(stats.avgIoPercent) + " %");
  setText("avg-priority", fmt1(stats.avgPriority));

  setText("avg-completed-count", fmtInt(stats.completedCount) + " завершено");
  setText("avg-wait", fmt1(stats.avgWait));
  setText("avg-exec", fmt1(stats.avgExec));
  setText("avg-io", fmt1(stats.avgIo));
  setText("avg-turnaround", fmt1(stats.avgTurnaround));
}

function renderHistory(snapshot) {
  const history = (snapshot.stats ?? {}).execHistory ?? [];
  const box = $("exec-history");

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

  setText("history-count", `${history.length} / 16`);
}

function renderReady(snapshot) {
  const rows = snapshot.ready ?? [];
  setText("ready-count", fmtInt(rows.length));

  renderRows("ready-table", rows, "ready-empty", (p) => `
    <tr>
      <td class="mono">#${p.id}</td>
      <td>${stateMark(p.state)}</td>
      <td class="mono">${p.pc}</td>
      <td class="mono">${p.size}</td>
      <td class="mono">${p.dynamicPriority}</td>
    </tr>`);
}

function renderIO(snapshot) {
  const rows = snapshot.operations ?? [];
  setText("io-count", fmtInt(rows.length));

  renderRows("io-table", rows, "io-empty", (op) => {
    const done = op.totalTicks > 0
      ? Math.round(((op.totalTicks - op.ticksLeft) / op.totalTicks) * 100)
      : 0;
    return `
    <tr>
      <td class="mono">#${op.processorId}</td>
      <td class="mono">#${op.processId}</td>
      <td>${stateMark(op.state)}</td>
      <td class="mono">${op.ticksLeft} / ${op.totalTicks}</td>
      <td class="mono">${done} %</td>
    </tr>`;
  });
}

function renderInterrupts(snapshot) {
  const rows = [...(snapshot.interruptLog ?? [])].reverse();
  setText("int-count", fmtInt(snapshot.interruptsHandled));

  renderRows("int-table", rows, "int-empty", (s) => `
    <tr>
      <td class="mono">${s.time}</td>
      <td>${INTERRUPT_NAMES[s.type] ?? "?"}</td>
      <td class="mono">ВВ #${s.sourceId}</td>
      <td class="mono">#${s.processId}</td>
    </tr>`);
}

function renderProcesses(snapshot) {
  const rows = snapshot.processes ?? [];
  setText("proc-count", `${rows.length} / ${snapshot.maxProcesses}`);

  renderRows("proc-table", rows, "proc-empty", (p) => `
    <tr class="${p.state === 2 ? "row-active" : ""}">
      <td class="mono">#${p.id}</td>
      <td>${stateMark(p.state)}</td>
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

// ----------------------------------------------------------------------------
// Отрисовка снимка состояния
// ----------------------------------------------------------------------------

function render(snapshot) {
  renderHeader(snapshot);
  renderSystem(snapshot);
  renderCPU(snapshot);
  renderTask(snapshot);
  renderAverages(snapshot);
  renderHistory(snapshot);
  renderReady(snapshot);
  renderIO(snapshot);
  renderInterrupts(snapshot);
  renderProcesses(snapshot);
}

// ----------------------------------------------------------------------------
// Приём снимков состояния из ядра
// ----------------------------------------------------------------------------

if (window.runtime) {
  window.runtime.EventsOn("os:update", render);
} else {
  console.warn("Wails runtime не найден — запустите приложение через wails dev / wails build");
}
