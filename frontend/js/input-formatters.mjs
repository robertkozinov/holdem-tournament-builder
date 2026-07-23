export function formatDateValue(value) {
    return formatDigitGroups(value, [2, 2, 4], "/");
}

export function formatTimeValue(value) {
    return formatDigitGroups(value, [2, 2], ":");
}

export function isIntegerKeyAllowed(key, modifierPressed = false) {
    return modifierPressed || key.length > 1 || /^[0-9]$/.test(key);
}

export function isIntegerText(value) {
    return /^[0-9]+$/.test(value);
}

function formatDigitGroups(value, groupSizes, separator) {
    const maxLength = groupSizes.reduce((sum, size) => sum + size, 0);
    const digits = value.replace(/\D/g, "").slice(0, maxLength);
    const groups = [];
    let offset = 0;

    for (const size of groupSizes) {
        const group = digits.slice(offset, offset + size);
        if (!group) {
            break;
        }
        groups.push(group);
        offset += size;
    }

    return groups.join(separator);
}
