// ============================================================================
// Модель ОС — интерфейс.
//
// Обмен с ядром идёт через события Wails:
//   os:update    — снимок состояния из ядра (Go → JS);
//   os:directive — управляющие директивы оператора (JS → Go).
// ============================================================================

"use strict";

const STATE_NAMES = ["Отсутствует", "Готов", "Активен", "Блокирован (IO)"];
const STATE_CLASSES = ["st-none", "st-ready", "st-active", "st-blocked"];
const CPU_NAMES = ["Ожидание", "Работа"];
const OP_NAMES = ["+", "−", "×", "÷"];

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
  $(id).textContent = value;
}

function statePill(state) {
  const name = STATE_NAMES[state] ?? "?";
  const cls = STATE_CLASSES[state] ?? "st-none";
  return `<span class="pill ${cls}">${name}</span>`;
}

function formatSpeed(value) {
  return Number(value).toLocaleString("ru-RU", { maximumFractionDigits: 1 });
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

function renderRows(tableId, rows, emptyId, rowBuilder) {
  const tbody = $(tableId).querySelector("tbody");
  tbody.innerHTML = rows.map(rowBuilder).join("");
  $(emptyId).style.display = rows.length ? "none" : "block";
}

// ----------------------------------------------------------------------------
// Отрисовка снимка состояния
// ----------------------------------------------------------------------------

function render(snapshot) {
  paused = snapshot.paused;
  finished = snapshot.quit;

  // Шапка
  setText("m-time", snapshot.time);
  setText("m-speed", formatSpeed(snapshot.speed));

  const status = $("m-status");
  if (snapshot.quit) {
    status.textContent = "Завершено";
    status.className = "pill st-none";
  } else if (snapshot.paused) {
    status.textContent = "Пауза";
    status.className = "pill st-blocked";
  } else {
    status.textContent = "Работа";
    status.className = "pill st-active";
  }

  $("btn-pause").textContent = snapshot.paused ? "Продолжить" : "Пауза";
  $("btn-pause").disabled = snapshot.quit;
  $("btn-slower").disabled = snapshot.quit;
  $("btn-faster").disabled = snapshot.quit;
  $("btn-quit").disabled = snapshot.quit;

  // Центральный процессор
  const cpuState = $("cpu-state");
  cpuState.textContent = CPU_NAMES[snapshot.cpuState] ?? "—";
  cpuState.className = "pill " + (snapshot.cpuState === 1 ? "st-active" : "st-none");
  setText("cpu-active", snapshot.activeId ? `#${snapshot.activeId}` : "—");
  setText("cpu-command", commandText(snapshot));

  // Память
  const free = snapshot.memoryTotal - snapshot.memoryUsed;
  const percent = snapshot.memoryTotal
    ? Math.round((snapshot.memoryUsed / snapshot.memoryTotal) * 100)
    : 0;
  $("mem-bar").style.width = percent + "%";
  setText("mem-percent", percent + " %");
  setText("mem-used", snapshot.memoryUsed);
  setText("mem-free", free);
  setText("mem-total", snapshot.memoryTotal);

  // Система
  setText("sys-generated", snapshot.generated);
  setText("sys-completed", snapshot.completed);
  setText("sys-ready", snapshot.ready.length);
  setText("sys-blocked", snapshot.blocked.length);
  setText("sys-table", `${snapshot.processes.length} / ${snapshot.maxProcesses}`);
  setText("ready-count", snapshot.ready.length);
  setText("blocked-count", snapshot.blocked.length);

  // Очередь готовности
  const maxPriority = Math.max(1, ...snapshot.ready.map((p) => p.dynamicPriority));
  renderRows("ready-table", snapshot.ready, "ready-empty", (p) => `
    <tr>
      <td class="mono">#${p.id}</td>
      <td>${statePill(p.state)}</td>
      <td class="mono">${p.pc}</td>
      <td class="mono">${p.size}</td>
      <td>
        <div class="prio">
          <span class="mono">${p.dynamicPriority}</span>
          <span class="prio-bar"><i style="width:${Math.round((p.dynamicPriority / maxPriority) * 100)}%"></i></span>
        </div>
      </td>
    </tr>`);

  // Блокированные на вводе-выводе
  const maxTicks = Math.max(1, ...snapshot.blocked.map((p) => p.ioTicksLeft));
  renderRows("blocked-table", snapshot.blocked, "blocked-empty", (p) => `
    <tr>
      <td class="mono">#${p.id}</td>
      <td class="mono">${p.pc}</td>
      <td class="mono">${p.ioTicksLeft}</td>
      <td>
        <span class="prio-bar"><i style="width:${Math.round((p.ioTicksLeft / maxTicks) * 100)}%"></i></span>
      </td>
    </tr>`);

  // Таблица процессов
  renderRows("proc-table", snapshot.processes, "proc-empty", (p) => `
    <tr class="${p.state === 2 ? "row-active" : ""}">
      <td class="mono">#${p.id}</td>
      <td>${statePill(p.state)}</td>
      <td class="mono">${p.pc}</td>
      <td class="mono">${p.totalCommands}</td>
      <td class="mono">${p.size}</td>
      <td class="mono">${p.basePriority} → ${p.dynamicPriority}</td>
      <td class="mono">${p.ioPercent} %</td>
      <td class="mono">${p.ioTicksLeft}</td>
    </tr>`);

  // Параметры генерации заданий
  const tp = snapshot.taskParams;
  setText(
    "task-params",
    `задание: размер ${tp.sizeMin}–${tp.sizeMax} · команд ${tp.commandsMin}–${tp.commandsMax} · IO ${tp.ioPercentMin}–${tp.ioPercentMax} % · приоритет ${tp.priorityMin}–${tp.priorityMax}`
  );
}

// ----------------------------------------------------------------------------
// Приём снимков состояния из ядра
// ----------------------------------------------------------------------------

if (window.runtime) {
  window.runtime.EventsOn("os:update", render);
} else {
  console.warn("Wails runtime не найден — запустите приложение через wails dev / wails build");
}
