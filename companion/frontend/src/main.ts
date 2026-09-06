import { ServerService } from "../bindings/github.com/nyakaspeter/white-raven/companion";
import { Browser } from "@wailsio/runtime";

const form = document.querySelector<HTMLFormElement>("#settings")!;
const toggle = document.querySelector<HTMLButtonElement>("#toggle")!;
const serverControl = document.querySelector<HTMLElement>(".server-control")!;
const serverState = document.querySelector<HTMLElement>("#server-state")!;
const hint = document.querySelector<HTMLAnchorElement>("#server-hint")!;
const logs = document.querySelector<HTMLElement>("#logs")!;
const copy = document.querySelector<HTMLButtonElement>("#copy")!;
const copyStatus = document.querySelector<HTMLElement>("#copy-status")!;
const error = document.querySelector<HTMLElement>("#error")!;
let running = false;
let currentConfig: any = {};
let currentLogs = "";
let saveTimer: number | undefined;
let saveRevision = 0;
let saveQueue = Promise.resolve();

function field(name: string): HTMLInputElement {
  return form.elements.namedItem(name) as HTMLInputElement;
}

function setForm(config: any) {
  currentConfig = { ...config };
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
  return data;
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
    hint.href = running ? next.address : "";
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

document.querySelectorAll<HTMLButtonElement>(".nav-item").forEach(item =>
  item.addEventListener("click", () => showPage(item.dataset.page!)));

hint.addEventListener("click", event => {
  event.preventDefault();
  if (hint.href) void Browser.OpenURL(hint.href);
});

document.querySelector("#clear")!.addEventListener("click", async () => {
  await ServerService.ClearLogs();
  await refresh();
});

copy.addEventListener("click", async () => {
  if (!currentLogs) return;
  try {
    await navigator.clipboard.writeText(currentLogs);
    copyStatus.textContent = "Copied";
  } catch {
    const helper = document.createElement("textarea");
    helper.value = currentLogs;
    document.body.appendChild(helper);
    helper.select();
    document.execCommand("copy");
    helper.remove();
    copyStatus.textContent = "Copied";
  }
  setTimeout(() => copyStatus.textContent = "", 1600);
});

setForm(await ServerService.LoadConfig());
await refresh();
setInterval(refresh, 750);
