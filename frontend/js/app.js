import {initializeCreateTournament} from "/js/create-tournament.js";
import {initializeTournamentRuntime} from "/js/tournament-runtime.js";

const createView = document.querySelector("#create-view");
const tournamentView = document.querySelector("#tournament-view");
const newTournamentButton = document.querySelector("#new-tournament-button");
const notice = document.querySelector("#notice");
let noticeTimeout;

const runtime = initializeTournamentRuntime({
    showNotice,
    onDeleted: showCreateView
});

const createTournamentForm = initializeCreateTournament(async (id) => {
    await showTournament(id);
    showNotice("Турнир создан", "success");
});

newTournamentButton.addEventListener("click", showCreateView);
window.addEventListener("beforeunload", runtime.stop);

const savedTournamentID = new URLSearchParams(window.location.search).get("tournament");
if (savedTournamentID) {
    showTournament(savedTournamentID).catch(() => {});
} else {
    showCreateView();
}

async function showTournament(id) {
    createView.hidden = true;
    tournamentView.hidden = false;
    newTournamentButton.hidden = false;
    window.history.replaceState({}, "", `/?tournament=${encodeURIComponent(id)}`);

    try {
        await runtime.load(id);
    } catch (error) {
        showNotice(error.message, "error");
        if (error.status === 404 || error.status === 400) {
            showCreateView();
        }
        throw error;
    }
}

function showCreateView() {
    runtime.stop();
    createTournamentForm.reset();
    createView.hidden = false;
    tournamentView.hidden = true;
    newTournamentButton.hidden = true;
    window.history.replaceState({}, "", "/");
}

function showNotice(message, type) {
    window.clearTimeout(noticeTimeout);
    notice.textContent = message;
    notice.className = `alert ${type === "success" ? "alert-success" : "alert-danger"} mb-4`;
    notice.hidden = false;
    noticeTimeout = window.setTimeout(() => {
        notice.hidden = true;
    }, 4000);
}
