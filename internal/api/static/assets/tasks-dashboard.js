const state = {
  assignees: [],
  activeUserID: "",
  theme: readTheme(),
  session: readSession(),
};

const els = {
  sessionSummary: document.querySelector("#sessionSummary"),
  authStatus: document.querySelector("#authStatus"),
  dashboardMeta: document.querySelector("#dashboardMeta"),
  assigneeTabs: document.querySelector("#assigneeTabs"),
  dashboardTasks: document.querySelector("#dashboardTasks"),
  refreshDashboard: document.querySelector("#refreshDashboard"),
  themeToggle: document.querySelector("#themeToggle"),
  themeIcon: document.querySelector("#themeIcon"),
  loginLink: document.querySelector("#loginLink"),
  logoutButton: document.querySelector("#logoutButton"),
};

init();

function init() {
  els.logoutButton.addEventListener("click", handleLogout);
  els.themeToggle.addEventListener("click", toggleTheme);
  els.refreshDashboard.addEventListener("click", loadDashboard);
  els.assigneeTabs.addEventListener("click", handleTabClick);
  els.dashboardTasks.addEventListener("click", handleTaskClick);

  renderTheme();
  renderSession();
  hydrateInitialView();
}

async function hydrateInitialView() {
  await hydrateCurrentUser(false);
  await loadDashboard();
}

function toggleTheme() {
  state.theme = state.theme === "dark" ? "light" : "dark";
  writeTheme(state.theme);
  renderTheme();
}

function renderTheme() {
  document.documentElement.dataset.theme = state.theme;
  els.themeIcon.textContent = state.theme === "dark" ? "☀" : "◐";
}

async function handleLogout() {
  try {
    await apiFetch("/api/auth/logout", { method: "POST" });
  } catch {
    // Local logout should still clear stale browser credentials.
  }
  state.session = null;
  state.assignees = [];
  state.activeUserID = "";
  writeSession(null);
  setAuthStatus("Logged out.");
  renderSession();
  renderDashboard();
}

async function hydrateCurrentUser(showError = true) {
  try {
    const user = await apiFetch("/api/me", { auth: true });
    state.session = { user };
    writeSession(state.session);
    renderSession();
  } catch (error) {
    if (isAuthError(error)) {
      state.session = null;
      writeSession(null);
      renderSession();
    }
    if (showError) setAuthStatus(error.message);
  }
}

function renderSession() {
  const user = state.session?.user;
  const loggedIn = Boolean(user);
  els.loginLink.href = `/login?return_to=${encodeURIComponent(window.location.pathname)}`;
  els.loginLink.classList.toggle("is-hidden", loggedIn);
  els.logoutButton.classList.toggle("is-hidden", !loggedIn);
  els.sessionSummary.textContent = loggedIn
    ? `@${user.handle} · ${user.points ?? 0} pts`
    : "Signed out";
}

async function loadDashboard() {
  if (!state.session?.user) {
    state.assignees = [];
    state.activeUserID = "";
    renderDashboard();
    return;
  }
  if (!state.session?.user) {
    await hydrateCurrentUser();
  }
  if (!state.session?.user?.is_admin) {
    state.assignees = [];
    state.activeUserID = "";
    setAuthStatus("Admin access required.");
    renderDashboard();
    return;
  }

  els.dashboardTasks.innerHTML = `<div class="empty-state">Loading...</div>`;
  try {
    const response = await apiFetch("/api/tasks/dashboard/data", { auth: true });
    state.assignees = response.assignees ?? [];
    if (!state.assignees.some((assignee) => assignee.user.id === state.activeUserID)) {
      state.activeUserID = state.assignees[0]?.user.id ?? "";
    }
    els.dashboardMeta.textContent = response.date ? formatDateLabel(response.date) : "";
    renderDashboard();
  } catch (error) {
    els.dashboardTasks.innerHTML = `<div class="error-state">${escapeHTML(error.message)}</div>`;
  }
}

function renderDashboard() {
  if (!state.session?.user) {
    els.dashboardMeta.textContent = "";
    els.assigneeTabs.innerHTML = "";
    els.dashboardTasks.innerHTML = `<div class="empty-state">Login as an admin.</div>`;
    return;
  }
  if (!state.session?.user?.is_admin) {
    els.dashboardMeta.textContent = "";
    els.assigneeTabs.innerHTML = "";
    els.dashboardTasks.innerHTML = `<div class="error-state">Admin access required.</div>`;
    return;
  }
  if (state.assignees.length === 0) {
    els.assigneeTabs.innerHTML = "";
    els.dashboardTasks.innerHTML = `<div class="empty-state">No assignments available today.</div>`;
    return;
  }

  if (!state.activeUserID) {
    state.activeUserID = state.assignees[0].user.id;
  }
  const active = activeAssignee();
  els.assigneeTabs.innerHTML = state.assignees.map((assignee) => renderAssigneeTab(assignee)).join("");
  els.dashboardTasks.innerHTML = active ? renderAssigneeTasks(active) : `<div class="empty-state">No assignments available today.</div>`;
}

function renderAssigneeTab(assignee) {
  const user = assignee.user;
  const active = user.id === state.activeUserID;
  const openCount = assignee.tasks?.length ?? 0;
  const completedCount = assignee.completed_tasks?.length ?? 0;
  return `
    <button class="assignee-tab${active ? " is-active" : ""}" type="button" role="tab" aria-selected="${active ? "true" : "false"}" data-user-id="${escapeAttribute(user.id)}">
      <span>${escapeHTML(user.display_name)}</span>
      <small>${openCount} open · ${completedCount} done · ${user.points ?? 0} pts</small>
    </button>
  `;
}

function renderAssigneeTasks(assignee) {
  const openTasks = assignee.tasks ?? [];
  const completedTasks = assignee.completed_tasks ?? [];
  if (openTasks.length === 0 && completedTasks.length === 0) {
    return `<div class="empty-state">No assignments available for ${escapeHTML(assignee.user.display_name)} today.</div>`;
  }

  const openHTML = openTasks.length ? `
    <section class="dashboard-task-section" aria-label="Open tasks">
      <div class="dashboard-section-head">
        <h3>Open</h3>
        <span>${openTasks.length}</span>
      </div>
      ${openTasks.map((task) => renderOpenTask(task)).join("")}
    </section>
  ` : `
    <section class="dashboard-task-section" aria-label="Open tasks">
      <div class="dashboard-section-head">
        <h3>Open</h3>
        <span>0</span>
      </div>
      <div class="empty-state compact-empty">${escapeHTML(assignee.user.display_name)} is done for today.</div>
    </section>
  `;

  const completedHTML = completedTasks.length ? `
    <section class="dashboard-task-section" aria-label="Completed tasks">
      <div class="dashboard-section-head">
        <h3>Completed</h3>
        <span>${completedTasks.length}</span>
      </div>
      ${completedTasks.map((task) => renderCompletedTask(task)).join("")}
    </section>
  ` : "";

  return `${openHTML}${completedHTML}`;
}

function renderOpenTask(task) {
  return `
    <article class="dashboard-task" data-task-id="${escapeAttribute(task.id)}">
      <div>
        <h3>${escapeHTML(task.title)}</h3>
        <p>${escapeHTML(task.frequency_kind)} · ${task.point_value} points</p>
      </div>
      <button class="primary-action complete-action" type="button" data-action="complete-task" data-task-id="${escapeAttribute(task.id)}">Complete</button>
    </article>
  `;
}

function renderCompletedTask(task) {
  return `
    <article class="dashboard-task is-completed" data-task-id="${escapeAttribute(task.id)}">
      <div>
        <h3>${escapeHTML(task.title)}</h3>
        <p>${escapeHTML(task.frequency_kind)} · ${task.point_value} points${task.completed_at ? ` · ${escapeHTML(formatTime(task.completed_at))}` : ""}</p>
      </div>
      <span class="completion-mark">Done</span>
    </article>
  `;
}

function handleTabClick(event) {
  const button = event.target.closest("[data-user-id]");
  if (!button) return;
  state.activeUserID = button.dataset.userId;
  renderDashboard();
}

function handleTaskClick(event) {
  const button = event.target.closest("[data-action='complete-task']");
  if (!button) return;
  completeDashboardTask(button.dataset.taskId, button);
}

async function completeDashboardTask(taskID, button) {
  const assignee = activeAssignee();
  if (!assignee || !taskID) return;

  button.disabled = true;
  try {
    const task = assignee.tasks.find((item) => item.id === taskID);
    const response = await apiFetch(`/api/tasks/dashboard/users/${encodeURIComponent(assignee.user.id)}/tasks/${encodeURIComponent(taskID)}/complete`, {
      method: "POST",
      auth: true,
    });
    assignee.tasks = assignee.tasks.filter((task) => task.id !== taskID);
    if (task) {
      assignee.completed_tasks = [
        ...(assignee.completed_tasks ?? []),
        {
          ...task,
          completed_at: response?.completion?.created_at,
        },
      ];
    }
    if (response?.user) {
      assignee.user = response.user;
      if (state.session.user?.id === response.user.id) {
        state.session = { ...state.session, user: response.user };
        writeSession(state.session);
        renderSession();
      }
    }
    setAuthStatus(`${assignee.user.display_name}: +${response?.points_awarded ?? 0} points.`);
    renderDashboard();
  } catch (error) {
    button.disabled = false;
    setAuthStatus(error.message);
  }
}

function activeAssignee() {
  return state.assignees.find((assignee) => assignee.user.id === state.activeUserID) ?? null;
}

async function apiFetch(path, options = {}) {
  const headers = new Headers(options.headers ?? {});
  headers.set("Accept", "application/json");
  if (options.body) headers.set("Content-Type", "application/json");

  const response = await fetch(apiPath(path), {
    method: options.method ?? "GET",
    headers,
    credentials: "same-origin",
    body: options.body ? JSON.stringify(options.body) : undefined,
  });

  if (response.status === 204) return null;

  const contentType = response.headers.get("Content-Type") ?? "";
  const payload = contentType.includes("application/json") ? await response.json() : null;
  if (response.status === 401 && options.auth && !options.skipRefresh) {
    await apiFetch("/api/auth/refresh", { method: "POST", skipRefresh: true });
    return apiFetch(path, { ...options, skipRefresh: true });
  }
  if (!response.ok) {
    throw new APIError(payload?.error || `Request failed with status ${response.status}`, response.status);
  }
  return payload;
}

class APIError extends Error {
  constructor(message, status) {
    super(message);
    this.name = "APIError";
    this.status = status;
  }
}

function isAuthError(error) {
  return error instanceof APIError && (error.status === 401 || error.status === 403);
}

function apiPath(path) {
  if (path.startsWith("/api/")) return path;
  return `/api${path}`;
}

function saveTokenSession(response) {
  state.session = { user: response.user };
  writeSession(state.session);
}

function readSession() {
  try {
    const session = JSON.parse(localStorage.getItem("home-api-session"));
    if (!session?.user) return null;
    const sanitized = { user: session.user };
    localStorage.setItem("home-api-session", JSON.stringify(sanitized));
    return sanitized;
  } catch {
    return null;
  }
}

function readTheme() {
  const saved = localStorage.getItem("home-api-theme");
  if (saved === "dark" || saved === "light") return saved;
  return window.matchMedia?.("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

function writeTheme(theme) {
  localStorage.setItem("home-api-theme", theme);
}

function writeSession(session) {
  if (!session) {
    localStorage.removeItem("home-api-session");
    return;
  }
  localStorage.setItem("home-api-session", JSON.stringify(session));
}

function formatDateLabel(value) {
  const date = new Date(`${value}T00:00:00`);
  return date.toLocaleDateString(undefined, { weekday: "long", month: "short", day: "numeric" });
}

function formatTime(value) {
  return new Date(value).toLocaleTimeString(undefined, { hour: "numeric", minute: "2-digit" });
}

function setAuthStatus(message) {
  els.authStatus.textContent = message;
}

function escapeHTML(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

function escapeAttribute(value) {
  return escapeHTML(value).replaceAll("`", "&#096;");
}
