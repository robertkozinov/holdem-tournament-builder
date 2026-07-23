import {createTournament} from "/js/api.js";
import {
    formatDateValue,
    formatTimeValue,
    isIntegerKeyAllowed,
    isIntegerText
} from "/js/input-formatters.mjs";

const form = document.querySelector("#tournament-form");
const dateInput = document.querySelector("#tournament-date");
const timeInput = document.querySelector("#tournament-time");
const styleInput = document.querySelector("#style-select");
const levelDurationInput = document.querySelector("#level-duration-input");
const playersList = document.querySelector("#players-list");
const playerTemplate = document.querySelector("#player-row-template");
const chipsList = document.querySelector("#chips-list");
const chipTemplate = document.querySelector("#chip-row-template");
const rebuyAllowed = document.querySelector("#rebuy-allowed");
const rebuyMaxLevel = document.querySelector("#rebuy-max-level");
const customPayoutField = document.querySelector("#custom-payout-field");
const customPayoutInput = document.querySelector("[name='fixed_buy_ins']");
const formError = document.querySelector("#form-error");
const createButton = document.querySelector("#create-button");

const levelDurationPresets = {
    turbo: 10,
    standard: 20,
    deep: 30
};

export function initializeCreateTournament(onCreated) {
    dateInput.addEventListener("input", () => {
        dateInput.value = formatDateValue(dateInput.value);
    });
    timeInput.addEventListener("input", () => {
        timeInput.value = formatTimeValue(timeInput.value);
    });
    form.addEventListener("keydown", restrictIntegerKey);
    form.addEventListener("paste", restrictIntegerPaste);
    document.querySelector("#add-player-button").addEventListener("click", () => addPlayer(""));
    document.querySelector("#add-chip-button").addEventListener("click", () => addChip("", ""));
    document.querySelector("#basic-chip-preset-button").addEventListener("click", applyBasicChipPreset);
    document.querySelector("#set-current-date-button").addEventListener("click", () => setDateTime(new Date()));
    styleInput.addEventListener("change", updateLevelDurationPlaceholder);
    rebuyAllowed.addEventListener("change", updateRebuyField);
    document.querySelectorAll("[name='payout_mode']").forEach((input) => {
        input.addEventListener("change", updatePayoutField);
    });
    form.addEventListener("submit", (event) => submitForm(event, onCreated));

    return {reset: resetForm};
}

function restrictIntegerKey(event) {
    if (!isIntegerInput(event.target)) {
        return;
    }

    if (!isIntegerKeyAllowed(event.key, event.ctrlKey || event.metaKey)) {
        event.preventDefault();
    }
}

function restrictIntegerPaste(event) {
    if (!isIntegerInput(event.target)) {
        return;
    }

    const pastedText = event.clipboardData?.getData("text") ?? "";
    if (!isIntegerText(pastedText)) {
        event.preventDefault();
    }
}

function isIntegerInput(target) {
    return target instanceof HTMLInputElement
        && target.matches('input[type="number"][step="1"]');
}

function resetForm() {
    form.reset();
    formError.textContent = "";
    playersList.replaceChildren();
    chipsList.replaceChildren();
    addPlayer("");
    addPlayer("");
    updateRebuyField();
    updatePayoutField();
    updateLevelDurationPlaceholder();
}

function setDateTime(date) {
    const day = String(date.getDate()).padStart(2, "0");
    const month = String(date.getMonth() + 1).padStart(2, "0");
    const year = date.getFullYear();
    const hours = String(date.getHours()).padStart(2, "0");
    const minutes = String(date.getMinutes()).padStart(2, "0");

    dateInput.value = `${day}/${month}/${year}`;
    timeInput.value = `${hours}:${minutes}`;
}

function addPlayer(value) {
    const row = playerTemplate.content.firstElementChild.cloneNode(true);
    const input = row.querySelector("[data-player-name]");
    input.value = value;
    row.querySelector("[data-remove-player]").addEventListener("click", () => {
        row.remove();
        updatePlayerRows();
    });
    playersList.append(row);
    updatePlayerRows();
    if (!value) {
        input.focus();
    }
}

function updatePlayerRows() {
    const rows = [...playersList.querySelectorAll(".repeat-row")];
    rows.forEach((row, index) => {
        row.querySelector(".player-row-label").textContent = `Игрок ${index + 1}`;
        row.querySelector("[data-remove-player]").disabled = rows.length <= 2;
    });
}

function addChip(value, count) {
    const row = chipTemplate.content.firstElementChild.cloneNode(true);
    const valueInput = row.querySelector("[data-chip-value]");
    valueInput.value = value;
    row.querySelector("[data-chip-count]").value = count;
    row.querySelector("[data-remove-chip]").addEventListener("click", () => {
        row.remove();
        updateChipRows();
    });
    chipsList.append(row);
    updateChipRows();
    if (value === "") {
        valueInput.focus();
    }
}

function applyBasicChipPreset() {
    chipsList.replaceChildren();
    [5, 10, 25, 50, 100, 500].forEach((value) => addChip(value, 50));
}

function updateChipRows() {
    const rows = [...chipsList.querySelectorAll(".repeat-row")];
    rows.forEach((row) => {
        row.querySelector("[data-remove-chip]").disabled = rows.length <= 1;
    });
}

function updateRebuyField() {
    rebuyMaxLevel.disabled = !rebuyAllowed.checked;
}

function updatePayoutField() {
    const custom = document.querySelector("[name='payout_mode']:checked").value === "custom";
    customPayoutField.hidden = !custom;
    customPayoutInput.disabled = !custom;
}

function updateLevelDurationPlaceholder() {
    const preset = levelDurationPresets[styleInput.value];
    levelDurationInput.placeholder = preset ? `Авто: ${preset}` : "Авто по стилю";
}

async function submitForm(event, onCreated) {
    event.preventDefault();
    formError.textContent = "";

    let input;
    try {
        input = buildInput();
    } catch (error) {
        formError.textContent = error.message;
        return;
    }

    setPending(true);
    try {
        const result = await createTournament(input);
        await onCreated(result.id);
    } catch (error) {
        formError.textContent = error.message;
    } finally {
        setPending(false);
    }
}

function buildInput() {
    const data = new FormData(form);
    const players = [...playersList.querySelectorAll("[data-player-name]")]
        .map((input) => input.value.trim())
        .filter(Boolean);

    if (players.length < 2) {
        throw new Error("Добавьте минимум двух игроков");
    }
    if (new Set(players).size !== players.length) {
        throw new Error("Имена игроков не должны повторяться");
    }

    const chips = [...chipsList.querySelectorAll(".repeat-row")].map((row) => ({
        value: Number(row.querySelector("[data-chip-value]").value),
        count: Number(row.querySelector("[data-chip-count]").value)
    }));
    if (chips.length === 0) {
        throw new Error("Добавьте хотя бы один номинал фишек");
    }
    if (new Set(chips.map((chip) => chip.value)).size !== chips.length) {
        throw new Error("Номиналы фишек не должны повторяться");
    }
    if (chips.some((chip) => chip.value <= 0 || chip.count <= 0)) {
        throw new Error("Номинал и количество фишек должны быть больше нуля");
    }

    const duration = Number(data.get("duration_minutes"));
    const levelDuration = Number(data.get("level_duration_minutes"));
    const effectiveLevelDuration = levelDuration || levelDurationPresets[data.get("style")];
    if (duration < effectiveLevelDuration * 2) {
        throw new Error("В структуре должно быть минимум два уровня блайндов");
    }

    const payoutMode = data.get("payout_mode");
    const name = data.get("name").trim();
    if (!name) {
        throw new Error("Введите название турнира");
    }

    return {
        name,
        date: parseDateTime(data.get("date"), data.get("time")).toISOString(),
        players,
        buy_in_amount: Number(data.get("buy_in_amount")),
        chips,
        style: data.get("style"),
        duration_minutes: duration,
        level_duration_minutes: levelDuration,
        rebuy: {
            allowed: rebuyAllowed.checked,
            max_level: rebuyAllowed.checked ? Number(rebuyMaxLevel.value) - 1 : 0
        },
        payout: {
            payout_mode: payoutMode,
            fixed_buy_ins: payoutMode === "custom" ? parseFixedBuyIns(customPayoutInput.value) : null
        }
    };
}

function parseDateTime(dateValue, timeValue) {
    const dateMatch = dateValue.match(/^(\d{2})\/(\d{2})\/(\d{4})$/);
    const timeMatch = timeValue.match(/^(\d{2}):(\d{2})$/);
    if (!dateMatch || !timeMatch) {
        throw new Error("Введите дату в формате ДД/ММ/ГГГГ и время ЧЧ:ММ");
    }

    const [, day, month, year] = dateMatch.map(Number);
    const [, hours, minutes] = timeMatch.map(Number);
    const date = new Date(year, month - 1, day, hours, minutes);

    const valid = date.getFullYear() === year
        && date.getMonth() === month - 1
        && date.getDate() === day
        && date.getHours() === hours
        && date.getMinutes() === minutes;
    if (!valid) {
        throw new Error("Введите корректные дату и время");
    }

    return date;
}

function parseFixedBuyIns(value) {
    if (!value.trim()) {
        return [];
    }
    const result = value.split(",").map((item) => Number(item.trim()));
    if (result.some((item) => !Number.isInteger(item) || item <= 0)) {
        throw new Error("Выплаты указываются целыми buy-in через запятую");
    }
    return result;
}

function setPending(pending) {
    createButton.disabled = pending;
    createButton.textContent = pending ? "Создаём..." : "Создать турнир";
}
