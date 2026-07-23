import test from "node:test";
import assert from "node:assert/strict";

import {
    formatDateValue,
    formatTimeValue,
    isIntegerKeyAllowed,
    isIntegerText
} from "./input-formatters.mjs";

test("formats date digits as DD/MM/YYYY", () => {
    assert.equal(formatDateValue("2"), "2");
    assert.equal(formatDateValue("230"), "23/0");
    assert.equal(formatDateValue("23072026"), "23/07/2026");
    assert.equal(formatDateValue("23.07.2026"), "23/07/2026");
});

test("formats time digits as HH:MM", () => {
    assert.equal(formatTimeValue("1"), "1");
    assert.equal(formatTimeValue("194"), "19:4");
    assert.equal(formatTimeValue("1945"), "19:45");
    assert.equal(formatTimeValue("19.45"), "19:45");
});

test("allows only integer characters and control keys", () => {
    assert.equal(isIntegerKeyAllowed("7"), true);
    assert.equal(isIntegerKeyAllowed("Backspace"), true);
    assert.equal(isIntegerKeyAllowed("e"), false);
    assert.equal(isIntegerKeyAllowed("-"), false);
    assert.equal(isIntegerKeyAllowed(".", true), true);
});

test("accepts only digit-only pasted text", () => {
    assert.equal(isIntegerText("123"), true);
    assert.equal(isIntegerText("12e3"), false);
    assert.equal(isIntegerText("12.3"), false);
    assert.equal(isIntegerText(""), false);
});
