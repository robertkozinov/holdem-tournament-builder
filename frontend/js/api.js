export class APIError extends Error {
    constructor(message, status = 0) {
        super(message);
        this.name = "APIError";
        this.status = status;
    }
}

async function request(path, options = {}) {
    const headers = {...options.headers};
    if (options.body) {
        headers["Content-Type"] = "application/json";
    }

    let response;
    try {
        response = await fetch(path, {...options, headers});
    } catch {
        throw new APIError("Не удалось связаться с сервером");
    }

    const isJSON = response.headers.get("content-type")?.includes("application/json");
    const body = isJSON ? await response.json() : null;

    if (!response.ok) {
        throw new APIError(body?.error || "Сервер не смог выполнить запрос", response.status);
    }

    return body;
}

export function createTournament(input) {
    return request("/tournaments", {
        method: "POST",
        body: JSON.stringify(input)
    });
}

export function getTournament(id) {
    return request(`/tournaments/${encodeURIComponent(id)}`);
}

export function deleteTournament(id) {
    return request(`/tournaments/${encodeURIComponent(id)}`, {method: "DELETE"});
}

export function runTournamentAction(id, action) {
    return request(`/tournaments/${encodeURIComponent(id)}/${action}`, {method: "POST"});
}

export function runPlayerAction(id, action, playerName) {
    return request(`/tournaments/${encodeURIComponent(id)}/${action}`, {
        method: "POST",
        body: JSON.stringify({player_name: playerName})
    });
}
