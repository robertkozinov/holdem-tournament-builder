import {
    deleteTournament,
    getTournament,
    runPlayerAction,
    runTournamentAction
} from "/js/api.js";

const numberFormat = new Intl.NumberFormat("ru-RU");
const elements = {
    name: document.querySelector("#tournament-name"),
    status: document.querySelector("#tournament-status"),
    meta: document.querySelector("#tournament-meta"),
    currentBlinds: document.querySelector("#current-blinds"),
    currentAnte: document.querySelector("#current-ante"),
    nextBlinds: document.querySelector("#next-blinds"),
    nextAnte: document.querySelector("#next-ante"),
    timerPanel: document.querySelector("#timer-panel"),
    timer: document.querySelector("#level-timer"),
    levelCounter: document.querySelector("#level-counter"),
    startButton: document.querySelector("#start-button"),
    pauseButton: document.querySelector("#pause-button"),
    resumeButton: document.querySelector("#resume-button"),
    levelUpButton: document.querySelector("#level-up-button"),
    finishButton: document.querySelector("#finish-button"),
    deleteButton: document.querySelector("#delete-tournament-button"),
    pot: document.querySelector("#summary-pot"),
    playersCount: document.querySelector("#summary-players"),
    stack: document.querySelector("#summary-stack"),
    stackDistribution: document.querySelector("#stack-distribution"),
    buyIn: document.querySelector("#summary-buy-in"),
    rebuyWindow: document.querySelector("#rebuy-window"),
    players: document.querySelector("#active-players"),
    payouts: document.querySelector("#payouts-list"),
    results: document.querySelector("#results-list"),
    blinds: document.querySelector("#blind-structure")
};

const state = {
    tournament: null,
    busy: false,
    refreshInterval: null,
    clockInterval: null,
    showNotice: null,
    onDeleted: null
};

export function initializeTournamentRuntime({showNotice, onDeleted}) {
    state.showNotice = showNotice;
    state.onDeleted = onDeleted;
    elements.startButton.addEventListener("click", () => performAction("start", "Турнир запущен"));
    elements.pauseButton.addEventListener("click", () => performAction("pause", "Турнир приостановлен"));
    elements.resumeButton.addEventListener("click", () => performAction("resume", "Турнир продолжен"));
    elements.levelUpButton.addEventListener("click", () => performAction("level-up", "Уровень повышен"));
    elements.finishButton.addEventListener("click", () => performAction("finish", "Турнир завершён"));
    elements.deleteButton.addEventListener("click", removeTournament);

    return {
        load: (id) => loadTournament(id),
        stop: stopLoops
    };
}

async function loadTournament(id) {
    stopLoops();
    state.tournament = await getTournament(id);
    renderTournament();
    startLoops();
}

async function refreshTournament() {
    if (!state.tournament) {
        return;
    }
    state.tournament = await getTournament(state.tournament.id);
    renderTournament();
}

function startLoops() {
    state.clockInterval = window.setInterval(renderTimer, 1000);
    state.refreshInterval = window.setInterval(() => {
        if (state.tournament?.status === "running" && !state.busy) {
            refreshTournament().catch(() => {});
        }
    }, 4000);
}

function stopLoops() {
    window.clearInterval(state.clockInterval);
    window.clearInterval(state.refreshInterval);
    state.clockInterval = null;
    state.refreshInterval = null;
}

function renderTournament() {
    const tournament = state.tournament;
    if (!tournament) {
        return;
    }

    const status = statusInfo(tournament.status);
    elements.name.textContent = tournament.name;
    elements.status.textContent = status.label;
    elements.status.className = `badge status-badge status-badge--${status.className}`;
    elements.meta.textContent = `${formatDate(tournament.date)} · ID ${tournament.id.slice(0, 8)}`;

    renderBlindLevel(elements.currentBlinds, elements.currentAnte, tournament.current_blind_level);
    renderBlindLevel(elements.nextBlinds, elements.nextAnte, tournament.next_blind_level);

    elements.pot.textContent = formatNumber(tournament.pot);
    elements.playersCount.textContent = tournament.players.length;
    elements.stack.textContent = formatNumber(tournament.starting_stack.total);
    renderStackDistribution(tournament.starting_stack.distribution);
    elements.buyIn.textContent = formatNumber(tournament.buy_in_amount);
    elements.levelCounter.textContent = `Уровень ${tournament.current_level + 1} из ${tournament.blind_structure.length}`;
    elements.rebuyWindow.textContent = tournament.rebuy_rules.allowed
        ? `Ребаи до уровня ${tournament.rebuy_rules.max_level + 1}`
        : "Без ребаев";

    elements.startButton.hidden = tournament.status !== "created";
    elements.pauseButton.hidden = tournament.status !== "running";
    elements.resumeButton.hidden = tournament.status !== "paused";
    elements.levelUpButton.hidden = tournament.status !== "running"
        || tournament.current_level >= tournament.blind_structure.length - 1;
    elements.finishButton.hidden = tournament.status !== "running" || tournament.players.length !== 1;

    renderPlayers(tournament);
    renderPayouts(tournament);
    renderResults(tournament.results || []);
    renderBlindStructure(tournament);
    renderTimer();
}

function renderStackDistribution(distribution) {
    elements.stackDistribution.replaceChildren();

    distribution.forEach((chip) => {
        const item = document.createElement("div");
        item.className = "stack-chip";

        const value = document.createElement("strong");
        value.textContent = formatNumber(chip.value);

        const count = document.createElement("span");
        count.textContent = `× ${chip.count}`;

        item.append(value, count);
        elements.stackDistribution.append(item);
    });
}

function renderBlindLevel(blindsElement, anteElement, level) {
    if (!level) {
        blindsElement.textContent = "— / —";
        anteElement.textContent = "Нет следующего уровня";
        return;
    }
    blindsElement.textContent = `${formatNumber(level.small_blind)} / ${formatNumber(level.big_blind)}`;
    anteElement.textContent = `Ante ${formatNumber(level.ante)}`;
}

function renderPlayers(tournament) {
    elements.players.replaceChildren();
    const rebuyOpen = tournament.rebuy_rules.allowed
        && tournament.current_level <= tournament.rebuy_rules.max_level;

    tournament.players.forEach((playerName, index) => {
        const row = document.createElement("div");
        row.className = "player-row";

        const identity = document.createElement("div");
        identity.className = "player-identity";

        const position = document.createElement("span");
        position.className = "player-position";
        position.textContent = String(index + 1).padStart(2, "0");

        const details = document.createElement("div");
        const name = document.createElement("strong");
        name.textContent = playerName;
        const contribution = document.createElement("small");
        contribution.textContent = `Взнос ${formatNumber(tournament.contributions[playerName] || 0)}`;
        details.append(name, contribution);
        identity.append(position, details);

        const actions = document.createElement("div");
        actions.className = "player-actions";
        actions.append(
            playerButton("Ребай", "btn-outline-success", !rebuyOpen, () => {
                performPlayerAction("rebuy", playerName, "Ребай добавлен");
            }),
            playerButton("Выбыл", "btn-outline-danger", tournament.players.length <= 1, () => {
                if (window.confirm(`Отметить вылет игрока ${playerName}?`)) {
                    performPlayerAction("knockout", playerName, `${playerName} выбыл`);
                }
            })
        );

        row.append(identity, actions);
        elements.players.append(row);
    });
}

function playerButton(label, style, disabled, handler) {
    const button = document.createElement("button");
    button.className = `btn btn-sm ${style}`;
    button.type = "button";
    button.textContent = label;
    button.disabled = disabled || state.busy || state.tournament.status !== "running";
    button.addEventListener("click", handler);
    return button;
}

function renderPayouts(tournament) {
    elements.payouts.replaceChildren();
    const fixedTotal = tournament.payout_spots.reduce((sum, spot) => {
        return spot.kind === "fixed" ? sum + spot.buy_ins_value * tournament.buy_in_amount : sum;
    }, 0);

    tournament.payout_spots.forEach((spot) => {
        const value = spot.kind === "fixed"
            ? spot.buy_ins_value * tournament.buy_in_amount
            : tournament.pot - fixedTotal;
        elements.payouts.append(dataRow(`${spot.place} место`, formatNumber(value)));
    });
}

function renderResults(results) {
    elements.results.replaceChildren();
    if (results.length === 0) {
        const empty = document.createElement("p");
        empty.className = "empty-state";
        empty.textContent = "Результатов пока нет";
        elements.results.append(empty);
        return;
    }

    [...results].sort((a, b) => a.place - b.place).forEach((result) => {
        elements.results.append(dataRow(`${result.place}. ${result.name}`, formatNumber(result.prize)));
    });
}

function dataRow(label, value) {
    const row = document.createElement("div");
    row.className = "data-row";
    const name = document.createElement("span");
    name.textContent = label;
    const amount = document.createElement("strong");
    amount.textContent = value;
    row.append(name, amount);
    return row;
}

function renderBlindStructure(tournament) {
    elements.blinds.replaceChildren();
    tournament.blind_structure.forEach((level, index) => {
        const row = document.createElement("tr");
        row.className = index === tournament.current_level
            ? "is-current"
            : index < tournament.current_level ? "is-complete" : "";

        [
            index + 1,
            formatNumber(level.small_blind),
            formatNumber(level.big_blind),
            formatNumber(level.ante),
            level.duration_minutes
        ].forEach((value) => {
            const cell = document.createElement("td");
            cell.textContent = value;
            row.append(cell);
        });
        elements.blinds.append(row);
    });
}

function renderTimer() {
    const tournament = state.tournament;
    if (!tournament?.current_blind_level) {
        return;
    }
    const seconds = remainingSeconds(tournament);
    elements.timer.textContent = formatTime(seconds);
    elements.timerPanel.classList.toggle("is-urgent", tournament.status === "running" && seconds <= 60);
    elements.timerPanel.classList.toggle("is-paused", tournament.status === "paused");
}

function remainingSeconds(tournament) {
    const duration = tournament.current_blind_level.duration_minutes * 60;
    if (tournament.status === "created") {
        return duration;
    }
    if (tournament.status === "finished" || !tournament.level_started_at) {
        return 0;
    }

    const end = tournament.status === "paused" && tournament.paused_at
        ? new Date(tournament.paused_at).getTime()
        : Date.now();
    const start = new Date(tournament.level_started_at).getTime();
    return Math.max(0, duration - Math.floor(Math.max(0, end - start) / 1000));
}

async function performAction(action, message) {
    if (!state.tournament || state.busy) {
        return;
    }
    setBusy(true);
    try {
        await runTournamentAction(state.tournament.id, action);
        await refreshTournament();
        state.showNotice(message, "success");
    } catch (error) {
        state.showNotice(error.message, "error");
    } finally {
        setBusy(false);
    }
}

async function performPlayerAction(action, playerName, message) {
    if (!state.tournament || state.busy) {
        return;
    }
    setBusy(true);
    try {
        await runPlayerAction(state.tournament.id, action, playerName);
        await refreshTournament();
        state.showNotice(message, "success");
    } catch (error) {
        state.showNotice(error.message, "error");
    } finally {
        setBusy(false);
    }
}

async function removeTournament() {
    if (!state.tournament || state.busy) {
        return;
    }
    if (!window.confirm(`Удалить турнир «${state.tournament.name}»?`)) {
        return;
    }

    setBusy(true);
    try {
        await deleteTournament(state.tournament.id);
        stopLoops();
        state.tournament = null;
        state.onDeleted();
        state.showNotice("Турнир удалён", "success");
    } catch (error) {
        state.showNotice(error.message, "error");
    } finally {
        setBusy(false);
    }
}

function setBusy(busy) {
    state.busy = busy;
    [
        elements.startButton,
        elements.pauseButton,
        elements.resumeButton,
        elements.levelUpButton,
        elements.finishButton,
        elements.deleteButton
    ].forEach((button) => {
        button.disabled = busy;
    });
    if (state.tournament) {
        renderPlayers(state.tournament);
    }
}

function statusInfo(status) {
    return {
        created: {label: "Создан", className: "created"},
        running: {label: "Идёт", className: "running"},
        paused: {label: "Пауза", className: "paused"},
        finished: {label: "Завершён", className: "finished"}
    }[status] || {label: status, className: "created"};
}

function formatTime(totalSeconds) {
    const minutes = Math.floor(totalSeconds / 60);
    const seconds = totalSeconds % 60;
    return `${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
}

function formatNumber(value) {
    return numberFormat.format(value || 0);
}

function formatDate(value) {
    return new Intl.DateTimeFormat("ru-RU", {
        day: "2-digit",
        month: "long",
        year: "numeric",
        hour: "2-digit",
        minute: "2-digit"
    }).format(new Date(value));
}
