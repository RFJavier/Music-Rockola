"use strict";

const $ = (selector) => document.querySelector(selector);
const state = { songs: [], player: null, searchTimer: null, toastTimer: null };

async function api(path, options = {}) {
  const response = await fetch(path, {
    ...options,
    headers: options.body ? { "Content-Type": "application/json", ...options.headers } : options.headers
  });
  if (response.status === 204) return null;
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(payload.error?.message || `HTTP ${response.status}`);
  return payload;
}

function notify(message, error = false) {
  const toast = $("#toast");
  toast.textContent = message;
  toast.className = `toast visible${error ? " error" : ""}`;
  clearTimeout(state.toastTimer);
  state.toastTimer = setTimeout(() => { toast.className = "toast"; }, 3500);
}

async function loadSongs(query = "") {
  try {
    const path = query.trim() ? `/api/songs/search?q=${encodeURIComponent(query.trim())}` : "/api/songs";
    state.songs = await api(path);
    renderSongs();
  } catch (error) {
    notify(error.message, true);
  }
}

function renderSongs() {
  const grid = $("#songGrid");
  grid.replaceChildren();
  $("#resultCount").textContent = `${state.songs.length} CANCION${state.songs.length === 1 ? "" : "ES"}`;
  $("#emptyCatalog").classList.toggle("hidden", state.songs.length > 0);
  state.songs.forEach((song, index) => {
    const card = document.createElement("article");
    card.className = "song-card";

    const disc = document.createElement("span");
    disc.className = "song-index";
    disc.textContent = String(index + 1).padStart(2, "0");

    const copy = document.createElement("div");
    copy.className = "song-copy";
    const title = document.createElement("strong");
    title.textContent = song.title;
    const artist = document.createElement("span");
    artist.textContent = song.artist || "Artista desconocido";
    copy.append(title, artist);

    const add = document.createElement("button");
    add.className = "add-song";
    add.textContent = "+";
    add.title = `Agregar ${song.title}`;
    add.addEventListener("click", () => addSong(song, add));
    card.append(disc, copy, add);
    grid.append(card);
  });
}

async function addSong(song, button) {
  button.disabled = true;
  try {
    await api("/api/queue", { method: "POST", body: JSON.stringify({ song_id: song.id }) });
    notify(`${song.title} agregada a la cola`);
    await Promise.all([loadQueue(), loadCredits()]);
    const player = await api("/api/player");
    if (player.status === "stopped") await api("/api/player/play", { method: "POST" });
    if (player.status === "ended") await api("/api/player/next", { method: "POST" });
    await loadPlayer();
  } catch (error) {
    notify(error.message, true);
  } finally {
    button.disabled = false;
  }
}

async function loadCredits() {
  try {
    const balance = await api("/api/credits");
    $("#creditBalance").textContent = balance.balance;
  } catch (error) {
    notify(error.message, true);
  }
}

async function addCredits(amount) {
  try {
    const balance = await api("/api/credits/add", {
      method: "POST", body: JSON.stringify({ amount })
    });
    $("#creditBalance").textContent = balance.balance;
    notify(`+${amount} créditos agregados`);
  } catch (error) {
    notify(error.message, true);
  }
}

async function loadQueue() {
  try {
    const entries = await api("/api/queue");
    const list = $("#queueList");
    list.replaceChildren();
    $("#emptyQueue").classList.toggle("hidden", entries.length > 0);
    entries.forEach((entry) => {
      const item = document.createElement("li");
      item.className = "queue-item";

      const position = document.createElement("span");
      position.className = "queue-position";
      position.textContent = String(entry.position).padStart(2, "0");

      const copy = document.createElement("div");
      copy.className = "queue-copy";
      const title = document.createElement("strong");
      title.textContent = entry.song.title;
      const artist = document.createElement("span");
      artist.textContent = `${entry.song.artist || "Artista desconocido"} / ${entry.status.toUpperCase()}`;
      copy.append(title, artist);

      const remove = document.createElement("button");
      remove.className = "remove-queue";
      remove.textContent = "X";
      remove.title = "Quitar de la cola";
      remove.disabled = entry.status === "playing";
      remove.addEventListener("click", () => removeQueue(entry.id));
      item.append(position, copy, remove);
      list.append(item);
    });
  } catch (error) {
    notify(error.message, true);
  }
}

async function removeQueue(id) {
  try {
    await api(`/api/queue/${id}`, { method: "DELETE" });
    await loadQueue();
  } catch (error) {
    notify(error.message, true);
  }
}

async function clearQueue() {
  try {
    await api("/api/queue", { method: "DELETE" });
    await Promise.all([loadQueue(), loadPlayer()]);
    notify("Cola cancelada");
  } catch (error) {
    notify(error.message, true);
  }
}

async function loadPlayer() {
  try {
    state.player = await api("/api/player");
    renderPlayer();
  } catch (error) {
    notify(error.message, true);
  }
}

function renderPlayer() {
  const player = state.player;
  const entry = player?.queue_entry;
  $("#playerStatus").textContent = statusLabel(player?.status || "stopped");
  $("#currentTitle").textContent = entry?.song.title || "Sin canción";
  $("#currentArtist").textContent = entry?.song.artist || (entry ? "Artista desconocido" : "Agrega música a la cola");
  $("#position").textContent = formatTime(player?.position_ms || 0);
  $("#duration").textContent = formatTime(player?.duration_ms || 0);
  const percent = player?.duration_ms > 0 ? Math.min(100, player.position_ms / player.duration_ms * 100) : 0;
  $("#progressBar").style.width = `${percent}%`;
  $("#record").classList.toggle("spinning", player?.status === "playing");
  $("#playButton").textContent = player?.status === "playing" ? "PAUSE" : "PLAY";
}

function statusLabel(status) {
  return ({ playing: "REPRODUCIENDO", paused: "PAUSADO", ended: "FINALIZADO", stopped: "DETENIDO" })[status] || status.toUpperCase();
}

function formatTime(milliseconds) {
  const total = Math.max(0, Math.floor(milliseconds / 1000));
  return `${String(Math.floor(total / 60)).padStart(2, "0")}:${String(total % 60).padStart(2, "0")}`;
}

async function togglePlayback() {
  try {
    const status = state.player?.status || "stopped";
    let endpoint = "/api/player/play";
    if (status === "playing") endpoint = "/api/player/pause";
    if (status === "paused") endpoint = "/api/player/resume";
    if (status === "ended") endpoint = "/api/player/next";
    await api(endpoint, { method: "POST" });
    await Promise.all([loadPlayer(), loadQueue()]);
  } catch (error) {
    notify(error.message, true);
  }
}

async function playerCommand(command) {
  try {
    await api(`/api/player/${command}`, { method: "POST" });
    await Promise.all([loadPlayer(), loadQueue()]);
  } catch (error) {
    notify(error.message, true);
  }
}

async function setVolume(value) {
  $("#volumeValue").textContent = Math.round(value * 100);
  try {
    await api("/api/player/volume", {
      method: "POST", body: JSON.stringify({ volume: Number(value) })
    });
  } catch (error) {
    notify(error.message, true);
  }
}

async function scanFolder(event) {
  event.preventDefault();
  const button = $("#scanButton");
  const output = $("#scanResult");
  button.disabled = true;
  output.textContent = "Escaneando...";
  try {
    const result = await api("/api/catalog/scan", {
      method: "POST", body: JSON.stringify({ path: $("#folderPath").value.trim() })
    });
    output.textContent = `${result.files_found} archivos / ${result.added} nuevos / ${result.existing} existentes / ${result.incompatible} incompatibles`;
    await loadSongs($("#searchInput").value);
  } catch (error) {
    output.textContent = error.message;
  } finally {
    button.disabled = false;
  }
}

function bindEvents() {
  $("#searchInput").addEventListener("input", (event) => {
    clearTimeout(state.searchTimer);
    state.searchTimer = setTimeout(() => loadSongs(event.target.value), 220);
  });
  $("#searchInput").addEventListener("keydown", (event) => {
    if (event.key === "Escape") {
      event.target.value = "";
      loadSongs();
    }
  });
  $("#openScanner").addEventListener("click", () => $("#scannerDialog").showModal());
  $("#closeScanner").addEventListener("click", () => $("#scannerDialog").close());
  $("#scannerForm").addEventListener("submit", scanFolder);
  $("#clearQueue").addEventListener("click", clearQueue);
  $("#playButton").addEventListener("click", togglePlayback);
  $("#stopButton").addEventListener("click", () => playerCommand("stop"));
  $("#nextButton").addEventListener("click", () => playerCommand("next"));
  $("#volumeInput").addEventListener("input", (event) => {
    $("#volumeValue").textContent = Math.round(event.target.value * 100);
  });
  $("#volumeInput").addEventListener("change", (event) => setVolume(event.target.value));
  document.querySelectorAll("[data-credits]").forEach((button) => {
    button.addEventListener("click", () => addCredits(Number(button.dataset.credits)));
  });
}

async function initialize() {
  bindEvents();
  await Promise.all([loadSongs(), loadQueue(), loadCredits(), loadPlayer()]);
  setInterval(loadPlayer, 1000);
  setInterval(loadQueue, 3000);
  setInterval(loadCredits, 5000);
}

initialize();
