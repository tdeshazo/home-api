const state = {
  tasks: [],
  dailyTasks: [],
  users: [],
  theme: readTheme(),
  session: readSession(),
};

const els = {
  sessionSummary: document.querySelector("#sessionSummary"),
  authStatus: document.querySelector("#authStatus"),
  tasksMeta: document.querySelector("#tasksMeta"),
  tasksList: document.querySelector("#tasksList"),
  refreshTasks: document.querySelector("#refreshTasks"),
  themeToggle: document.querySelector("#themeToggle"),
  themeIcon: document.querySelector("#themeIcon"),
  taskForm: document.querySelector("#taskForm"),
  taskTitle: document.querySelector("#taskForm input[name='title']"),
  taskCounter: document.querySelector("#taskCounter"),
  loginLink: document.querySelector("#loginLink"),
  logoutButton: document.querySelector("#logoutButton"),
};

init();

function init() {
  els.logoutButton.addEventListener("click", handleLogout);
  els.themeToggle.addEventListener("click", toggleTheme);
  els.refreshTasks.addEventListener("click", () => loadTaskView());
  els.taskForm.addEventListener("submit", handleCreateTask);
  els.taskForm.addEventListener("change", handleTaskFormChange);
  els.taskTitle.addEventListener("input", updateTaskCounter);
  els.tasksList.addEventListener("click", handleTaskClick);

  renderTheme();
  updateTaskCounter();
  renderSession();
  hydrateInitialView();
}

async function hydrateInitialView() {
  await hydrateCurrentUser(false);
  await loadTaskView();
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
  state.users = [];
  state.dailyTasks = [];
  writeSession(null);
  setAuthStatus("Logged out.");
  renderSession();
  await loadTaskView();
}

async function hydrateCurrentUser(showError = true) {
  try {
    const user = await apiFetch("/api/me", { auth: true });
    state.session = { user };
    writeSession(state.session);
    renderSession();
  } catch (error) {
    state.session = null;
    writeSession(null);
    renderSession();
    if (showError) setAuthStatus(error.message);
  }
}

function renderSession() {
  const user = state.session?.user;
  const loggedIn = Boolean(user);
  const isAdmin = Boolean(user?.is_admin);
  els.loginLink.href = `/login?return_to=${encodeURIComponent(window.location.pathname)}`;
  els.loginLink.classList.toggle("is-hidden", loggedIn);
  els.logoutButton.classList.toggle("is-hidden", !loggedIn);
  els.taskForm.classList.toggle("is-hidden", !isAdmin);
  els.sessionSummary.textContent = loggedIn
    ? `@${user.handle} · ${user.points ?? 0} pts`
    : "Signed out";
  els.tasksMeta.textContent = isAdmin
    ? "Create assigned recurring tasks"
    : loggedIn ? "Your daily tasks" : "Login to view assigned daily tasks";
  renderAssignees();
  renderTasks();
}

async function loadTaskView() {
  if (state.session?.user?.is_admin) {
    await Promise.all([loadAllTasks(), loadUsers()]);
    return;
  }
  if (state.session?.user) {
    await loadDailyTasks();
    return;
  }
  state.tasks = [];
  state.dailyTasks = [];
  renderTasks();
}

async function loadAllTasks() {
  els.tasksList.innerHTML = `<div class="empty-state">Loading...</div>`;
  try {
    const response = await apiFetch("/api/tasks");
    state.tasks = response.tasks ?? [];
    renderTasks();
  } catch (error) {
    els.tasksList.innerHTML = `<div class="error-state">${escapeHTML(error.message)}</div>`;
  }
}

async function loadDailyTasks() {
  els.tasksList.innerHTML = `<div class="empty-state">Loading...</div>`;
  try {
    const response = await apiFetch("/api/me/tasks", { auth: true });
    state.dailyTasks = response.tasks ?? [];
    renderTasks();
  } catch (error) {
    els.tasksList.innerHTML = `<div class="error-state">${escapeHTML(error.message)}</div>`;
  }
}

async function loadUsers() {
  try {
    const response = await apiFetch("/api/users", { auth: true });
    state.users = response.users ?? [];
    renderAssignees();
  } catch (error) {
    setAuthStatus(error.message);
  }
}

async function handleCreateTask(event) {
  event.preventDefault();
  if (!state.session?.user?.is_admin) {
    setAuthStatus("Admin access required.");
    return;
  }

  const payload = readTaskForm();
  if (!payload) return;

  els.taskForm.querySelector("button[type='submit']").disabled = true;
  try {
    const task = await apiFetch("/api/tasks", {
      method: "POST",
      auth: true,
      body: payload,
    });
    state.tasks = [...state.tasks, task];
    els.taskForm.reset();
    els.taskForm.elements.frequency_kind.value = "daily";
    els.taskForm.elements.is_active.checked = true;
    handleTaskFormChange();
    updateTaskCounter();
    renderTasks();
  } catch (error) {
    setAuthStatus(error.message);
  } finally {
    els.taskForm.querySelector("button[type='submit']").disabled = false;
  }
}

function readTaskForm() {
  const form = els.taskForm;
  const title = form.elements.title.value.trim();
  if (!title) return null;
  const frequencyKind = form.elements.frequency_kind.value;
  const daysOfWeek = Array.from(form.querySelectorAll("input[name='days_of_week']:checked"))
    .map((input) => Number.parseInt(input.value, 10));
  const pointValue = Number.parseInt(form.elements.point_value.value, 10);
  const assigneeIDs = Array.from(form.querySelectorAll("input[name='assignee_ids']:checked"))
    .map((input) => input.value);

  return {
    title,
    frequency_kind: frequencyKind,
    days_of_week: frequencyKind === "weekly" ? daysOfWeek : [],
    point_value: Number.isFinite(pointValue) ? pointValue : 0,
    individual: form.elements.individual.checked,
    is_active: form.elements.is_active.checked,
    assignee_ids: assigneeIDs,
  };
}

function handleTaskFormChange() {
  const weekly = els.taskForm.elements.frequency_kind.value === "weekly";
  document.querySelector("#weeklyDays").classList.toggle("is-hidden", !weekly);
}

function renderAssignees() {
  const assignees = document.querySelector("#assignees");
  if (!assignees) return;
  if (!state.session?.user?.is_admin) {
    assignees.innerHTML = "";
    return;
  }
  if (!state.users.length) {
    assignees.innerHTML = `<div class="empty-inline">No users loaded.</div>`;
    return;
  }
  assignees.innerHTML = state.users.map((user) => `
    <label class="check-option">
      <input type="checkbox" name="assignee_ids" value="${escapeAttribute(user.id)}" />
      <span>${escapeHTML(user.display_name)} <small>@${escapeHTML(user.handle)}</small></span>
    </label>
  `).join("");
}

function renderTasks() {
  const isAdmin = Boolean(state.session?.user?.is_admin);
  const tasks = isAdmin ? state.tasks : state.dailyTasks;
  if (!state.session?.user && !isAdmin) {
    els.tasksList.innerHTML = `<div class="empty-state">Login to view assigned daily tasks.</div>`;
    return;
  }
  if (tasks.length === 0) {
    els.tasksList.innerHTML = `<div class="empty-state">${isAdmin ? "No tasks found." : "No daily tasks assigned for today."}</div>`;
    return;
  }

  els.tasksList.innerHTML = tasks.map((task) => renderTask(task, isAdmin)).join("");
}

function renderTask(task, canAdmin) {
  return `
    <article class="task-item" data-task-id="${escapeHTML(task.id)}">
      <div class="task-main">
        <div>
          <h3>${escapeHTML(task.title)}</h3>
          <p>${escapeHTML(task.frequency_kind)} · ${task.point_value} points · ${task.is_active ? "active" : "inactive"}</p>
        </div>
        ${canAdmin ? `
          <div class="task-actions">
            ${task.is_active
              ? `<button class="text-action danger" type="button" data-action="delete-task" data-task-id="${escapeHTML(task.id)}">Deactivate</button>`
              : `<span class="task-state-pill">Inactive</span>`}
          </div>
        ` : `
          <div class="task-actions">
            <button class="primary-action compact-action" type="button" data-action="complete-task" data-task-id="${escapeHTML(task.id)}">Complete</button>
          </div>
        `}
      </div>
    </article>
  `;
}

function handleTaskClick(event) {
  const button = event.target.closest("[data-action]");
  if (!button) return;
  if (button.dataset.action === "delete-task") {
    deleteTask(button.dataset.taskId);
  }
  if (button.dataset.action === "complete-task") {
    completeTask(button.dataset.taskId, button);
  }
}

async function deleteTask(taskID) {
  try {
    await apiFetch(`/api/tasks/${encodeURIComponent(taskID)}`, {
      method: "DELETE",
      auth: true,
    });
    state.tasks = state.tasks.map((task) => task.id === taskID ? { ...task, is_active: false } : task);
    renderTasks();
  } catch (error) {
    setAuthStatus(error.message);
  }
}

async function completeTask(taskID, button) {
  if (!taskID) return;
  button.disabled = true;
  try {
    const response = await apiFetch(`/api/tasks/${encodeURIComponent(taskID)}/complete`, {
      method: "POST",
      auth: true,
    });
    state.dailyTasks = state.dailyTasks.filter((task) => task.id !== taskID);
    if (response?.user) {
      state.session = { ...state.session, user: response.user };
      writeSession(state.session);
    }
    setAuthStatus(`Completed. +${response?.points_awarded ?? 0} points.`);
    renderSession();
  } catch (error) {
    button.disabled = false;
    setAuthStatus(error.message);
  }
}

function updateTaskCounter() {
  els.taskCounter.textContent = `${els.taskTitle.value.length}/200`;
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
  if (!response.ok) {
    throw new Error(payload?.error || `Request failed with status ${response.status}`);
  }
  return payload;
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
