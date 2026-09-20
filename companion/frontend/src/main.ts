import { ServerService } from "../bindings/github.com/nyakaspeter/white-raven/companion";

const form = document.querySelector<HTMLFormElement>("#settings")!;
const toggle = document.querySelector<HTMLButtonElement>("#toggle")!;
const serverControl = document.querySelector<HTMLElement>(".server-control")!;
const serverState = document.querySelector<HTMLElement>("#server-state")!;
const hint = document.querySelector<HTMLAnchorElement>("#server-hint")!;
const logs = document.querySelector<HTMLElement>("#logs")!;
const copy = document.querySelector<HTMLButtonElement>("#copy")!;
const copyStatus = document.querySelector<HTMLElement>("#copy-status")!;
const error = document.querySelector<HTMLElement>("#error")!;
const harbrrToggle = document.querySelector<HTMLButtonElement>("#harbrr-toggle")!;
const harbrrControl = document.querySelector<HTMLElement>("#harbrr-control")!;
const harbrrState = document.querySelector<HTMLElement>("#harbrr-state")!;
const harbrrHint = document.querySelector<HTMLAnchorElement>("#harbrr-hint")!;
const harbrrLogs = document.querySelector<HTMLElement>("#harbrr-logs")!;
const harbrrCopy = document.querySelector<HTMLButtonElement>("#harbrr-copy")!;
const harbrrCopyStatus = document.querySelector<HTMLElement>("#harbrr-copy-status")!;
const harbrrError = document.querySelector<HTMLElement>("#harbrr-error")!;
const sshForm = document.querySelector<HTMLFormElement>("#ssh-settings")!;
const installRooted = document.querySelector<HTMLButtonElement>("#install-rooted")!;
const appSyncToggle = document.querySelector<HTMLButtonElement>("#app-sync-toggle")!;
const syncInstructions = document.querySelector<HTMLElement>("#sync-instructions")!;
const syncAddresses = document.querySelectorAll<HTMLElement>(".sync-address");
const syncStatus = document.querySelector<HTMLElement>("#sync-status")!;
const rootedStatus = document.querySelector<HTMLElement>("#rooted-status")!;
const torznabFeeds = document.querySelector<HTMLElement>("#torznab-feeds")!;
const addTorznabFeedButton = document.querySelector<HTMLButtonElement>("#add-torznab-feed")!;
let running = false;
let currentConfig: any = {};
let currentLogs = "";
let harbrrRunning = false;
let currentHarbrrLogs = "";
let saveTimer: number | undefined;
let saveRevision = 0;
let saveQueue = Promise.resolve();

function field(name: string): HTMLInputElement {
  return form.elements.namedItem(name) as HTMLInputElement;
}

function setForm(config: any) {
  currentConfig = { ...config };
  renderTorznabFeeds(Array.isArray(config.torznabFeeds) ? config.torznabFeeds : []);
  for (const [name, value] of Object.entries(config)) {
    const element = form.elements.namedItem(name) as HTMLInputElement | null;
    if (!element) continue;
    if (element.type === "checkbox") element.checked = Boolean(value);
    else element.value = String(value ?? "");
  }
}

function getForm(): any {
  const data: any = { ...currentConfig };
  new FormData(form).forEach((value, name) => data[name] = value);
  for (const name of ["memorySize", "downloadRate", "uploadRate", "maxConnections"])
    data[name] = Number(data[name]);
  data.noDHT = field("noDHT").checked;
  data.disableIPv6 = field("disableIPv6").checked;
  data.disableUTP = field("disableUTP").checked;
  data.torznabFeeds = Array.from(torznabFeeds.querySelectorAll<HTMLElement>(".torznab-feed"))
    .map(feed => ({
      url: feed.querySelector<HTMLInputElement>('[data-field="url"]')!.value.trim(),
      apiKey: feed.querySelector<HTMLInputElement>('[data-field="apiKey"]')!.value.trim(),
    }))
    .filter(feed => feed.url !== "");
  return data;
}

function renderTorznabFeeds(feeds: any[]) {
  torznabFeeds.replaceChildren();
  feeds.forEach(addTorznabFeed);
}

function addTorznabFeed(feed: any = {}) {
  const container = document.createElement("article");
  container.className = "torznab-feed";
  container.innerHTML = `
    <label class="stacked-field"><span>Torznab API endpoint</span><input data-field="url" type="url" inputmode="url" placeholder="http://host:port/path/to/api" /></label>
    <label class="stacked-field"><span>API key</span><input data-field="apiKey" type="password" autocomplete="off" /></label>
    <div class="feed-actions">
      <button class="feed-action-button remove-feed-button" type="button" aria-label="Remove Torznab feed" title="Remove feed">
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 7h16M9 7V4h6v3M7 7l1 13h8l1-13M10 11v5M14 11v5"/></svg>
        <span>Remove feed</span>
      </button>
    </div>`;
  container.querySelector<HTMLInputElement>('[data-field="url"]')!.value = String(feed.url || "");
  container.querySelector<HTMLInputElement>('[data-field="apiKey"]')!.value = String(feed.apiKey || "");
  container.querySelector<HTMLButtonElement>(".remove-feed-button")!.addEventListener("click", () => {
    container.remove();
    scheduleSave();
  });
  torznabFeeds.appendChild(container);
}

function showPage(pageID: string) {
  document.querySelectorAll<HTMLElement>(".page").forEach(page =>
    page.classList.toggle("active", page.id === pageID));
  document.querySelectorAll<HTMLButtonElement>(".nav-item").forEach(item => {
    const active = item.dataset.page === pageID;
    item.classList.toggle("active", active);
    if (active) item.setAttribute("aria-current", "page");
    else item.removeAttribute("aria-current");
  });
  window.scrollTo({ top: 0, behavior: "smooth" });
}

function setSettingsDisabled(disabled: boolean) {
  form.querySelectorAll<HTMLInputElement>("input").forEach(input => input.disabled = disabled);
  addTorznabFeedButton.disabled = disabled;
  torznabFeeds.querySelectorAll<HTMLButtonElement>("button").forEach(button => button.disabled = disabled);
}

async function refresh() {
  try {
    const next = await ServerService.Status();
    running = next.running;
    setSettingsDisabled(running);
    serverControl.classList.toggle("running", running);
    serverState.textContent = running ? "Server running" : "Server stopped";
    hint.hidden = !running;
    hint.textContent = running ? next.address : "";
    hint.href = running ? "http://localhost:9000" : "";
    const toggleAction = running ? "Stop server" : "Start server";
    toggle.setAttribute("aria-label", toggleAction);
    toggle.title = toggleAction;
    const nextLogs = await ServerService.Logs();
    const displayLogs = nextLogs || (running ? "No log entries yet." : "Server is stopped.");
    currentLogs = nextLogs;
    copy.disabled = !currentLogs;
    if (logs.textContent !== displayLogs) {
      const atBottom = logs.scrollHeight - logs.scrollTop - logs.clientHeight < 35;
      logs.textContent = displayLogs;
      if (atBottom) logs.scrollTop = logs.scrollHeight;
    }
  } catch (cause) {
    error.textContent = String(cause);
  }
}

async function refreshHarbrr() {
  try {
    const next = await ServerService.HarbrrStatus();
    harbrrRunning = next.running;
    harbrrControl.classList.toggle("running", harbrrRunning);
    harbrrState.textContent = harbrrRunning ? "Harbrr running" : "Harbrr stopped";
    harbrrHint.hidden = !harbrrRunning;
    harbrrHint.textContent = harbrrRunning ? next.address : "";
    harbrrHint.href = harbrrRunning ? "http://localhost:7478" : "";
    const toggleAction = harbrrRunning ? "Stop Harbrr" : "Start Harbrr";
    harbrrToggle.setAttribute("aria-label", toggleAction);
    harbrrToggle.title = toggleAction;
    const nextLogs = await ServerService.HarbrrLogs();
    const displayLogs = nextLogs || (harbrrRunning ? "No log entries yet." : "Harbrr is stopped.");
    currentHarbrrLogs = nextLogs;
    harbrrCopy.disabled = !currentHarbrrLogs;
    if (harbrrLogs.textContent !== displayLogs) {
      const atBottom = harbrrLogs.scrollHeight - harbrrLogs.scrollTop - harbrrLogs.clientHeight < 35;
      harbrrLogs.textContent = displayLogs;
      if (atBottom) harbrrLogs.scrollTop = harbrrLogs.scrollHeight;
    }
  } catch (cause) {
    harbrrError.textContent = String(cause);
  }
}

function selectInstallType(rooted: boolean) {
  document.querySelector<HTMLElement>("#unrooted-install")!.hidden = rooted;
  document.querySelector<HTMLElement>("#rooted-install")!.hidden = !rooted;
  const unrootedButton = document.querySelector<HTMLButtonElement>("#choose-unrooted")!;
  const rootedButton = document.querySelector<HTMLButtonElement>("#choose-rooted")!;
  unrootedButton.classList.toggle("active", !rooted);
  rootedButton.classList.toggle("active", rooted);
  unrootedButton.setAttribute("aria-selected", String(!rooted));
  rootedButton.setAttribute("aria-selected", String(rooted));
  syncStatus.hidden = rooted;
  rootedStatus.hidden = !rooted;
}

async function refreshWidgetStatus() {
  try {
    const status = await ServerService.WidgetStatus();
    syncStatus.textContent = status.mode === "unrooted" ? status.message || "" : "";
    rootedStatus.textContent = status.mode === "rooted" ? status.message || "" : "";
    installRooted.disabled = status.busy;
    appSyncToggle.disabled = status.busy;
    appSyncToggle.textContent = status.syncRunning ? "Stop App Sync server" : "Start App Sync server";
    syncInstructions.hidden = !status.syncRunning;
    const address = status.address ? status.address.replace(/^https?:\/\//, "") : "—";
    syncAddresses.forEach(element => element.textContent = address);
  } catch (cause) {
    const rooted = !document.querySelector<HTMLElement>("#rooted-install")!.hidden;
    (rooted ? rootedStatus : syncStatus).textContent = String(cause);
  }
}

toggle.addEventListener("click", async () => {
  if (!running && !form.reportValidity()) {
    showPage("settings-page");
    return;
  }
  toggle.disabled = true;
  error.textContent = "";
  try {
    if (running) await ServerService.StopServer();
    else {
      currentConfig = getForm();
      await ServerService.StartServer(currentConfig);
    }
  } catch (cause) {
    error.textContent = String(cause);
  } finally {
    toggle.disabled = false;
    await refresh();
  }
});

harbrrToggle.addEventListener("click", async () => {
  harbrrToggle.disabled = true;
  harbrrError.textContent = "";
  try {
    if (harbrrRunning) await ServerService.StopHarbrr();
    else await ServerService.StartHarbrr();
  } catch (cause) {
    harbrrError.textContent = String(cause);
  } finally {
    harbrrToggle.disabled = false;
    await refreshHarbrr();
  }
});

function scheduleSave() {
  if (running) return;
  window.clearTimeout(saveTimer);
  const revision = ++saveRevision;
  saveTimer = window.setTimeout(async () => {
    if (!form.checkValidity()) return;
    const config = getForm();
    currentConfig = config;
    try {
      saveQueue = saveQueue.catch(() => undefined).then(() => ServerService.SaveConfig(config));
      await saveQueue;
    } catch (cause) {
      if (revision !== saveRevision) return;
      console.error(cause);
    }
  }, 400);
}

form.addEventListener("input", scheduleSave);
form.addEventListener("submit", event => event.preventDefault());
addTorznabFeedButton.addEventListener("click", () => {
  addTorznabFeed();
  torznabFeeds.querySelector<HTMLInputElement>(".torznab-feed:last-child input")?.focus();
});

document.querySelectorAll<HTMLButtonElement>(".nav-item").forEach(item =>
  item.addEventListener("click", () => showPage(item.dataset.page!)));

document.querySelector("#choose-unrooted")!.addEventListener("click", () => selectInstallType(false));
document.querySelector("#choose-rooted")!.addEventListener("click", () => selectInstallType(true));

appSyncToggle.addEventListener("click", async () => {
  syncStatus.textContent = "";
  appSyncToggle.disabled = true;
  try {
    const status = await ServerService.WidgetStatus();
    if (status.syncRunning) await ServerService.StopAppSync();
    else await ServerService.StartAppSync();
  } catch (cause) {
    syncStatus.textContent = String(cause);
  } finally {
    await refreshWidgetStatus();
  }
});

sshForm.addEventListener("submit", async event => {
  event.preventDefault();
  if (!sshForm.reportValidity()) return;
  rootedStatus.textContent = "";
  installRooted.disabled = true;
  const data = new FormData(sshForm);
  try {
    currentConfig = getForm();
    await ServerService.InstallRooted({
      host: String(data.get("host") || ""),
      port: Number(data.get("port")),
      username: String(data.get("username") || "root"),
      password: String(data.get("password") || ""),
      installServer: (sshForm.elements.namedItem("installServer") as HTMLInputElement).checked,
      reboot: (sshForm.elements.namedItem("reboot") as HTMLInputElement).checked,
      config: currentConfig,
    });
    (sshForm.elements.namedItem("password") as HTMLInputElement).value = "";
  } catch (cause) {
    rootedStatus.textContent = String(cause);
  } finally {
    await refreshWidgetStatus();
  }
});

hint.addEventListener("click", event => {
  event.preventDefault();
  if (hint.href) void ServerService.OpenServerWebUI();
});

harbrrHint.addEventListener("click", event => {
  event.preventDefault();
  if (harbrrHint.href) void ServerService.OpenHarbrrWebUI();
});

document.querySelector("#clear")!.addEventListener("click", async () => {
  await ServerService.ClearLogs();
  await refresh();
});

document.querySelector("#harbrr-clear")!.addEventListener("click", async () => {
  await ServerService.ClearHarbrrLogs();
  await refreshHarbrr();
});

async function copyText(value: string, status: HTMLElement) {
  try {
    await navigator.clipboard.writeText(value);
  } catch {
    const helper = document.createElement("textarea");
    helper.value = value;
    document.body.appendChild(helper);
    helper.select();
    document.execCommand("copy");
    helper.remove();
  }
  status.textContent = "Copied";
  setTimeout(() => status.textContent = "", 1600);
}

copy.addEventListener("click", async () => {
  if (!currentLogs) return;
  await copyText(currentLogs, copyStatus);
});

harbrrCopy.addEventListener("click", async () => {
  if (!currentHarbrrLogs) return;
  await copyText(currentHarbrrLogs, harbrrCopyStatus);
});

setForm(await ServerService.LoadConfig());
await refresh();
await refreshHarbrr();
await refreshWidgetStatus();
setInterval(refresh, 750);
setInterval(refreshHarbrr, 750);
setInterval(refreshWidgetStatus, 750);
