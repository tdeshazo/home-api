import {
  apiFetch,
  bindDrawer,
  bindTheme,
  escapeAttribute,
  escapeHTML,
  hydrateIcons,
  isAuthError,
  readSession,
  readTheme,
  renderSessionChrome,
  setStatus,
  writeSession,
} from "./shared.js";

const state = {
  tasks: [],
  dailyTasks: [],
  users: [],
  theme: readTheme(),
  session: readSession(),
};

const els = {
  sessionSummaries: document.querySelectorAll("[data-session-summary]"),
  navProfiles: document.querySelectorAll("[data-nav-profile]"),
  navAvatars: document.querySelectorAll("[data-nav-avatar]"),
  navNames: document.querySelectorAll("[data-nav-name]"),
  authStatus: document.querySelector("#authStatus"),
  tasksMeta: document.querySelector("#tasksMeta"),
  tasksList: document.querySelector("#tasksList"),
  refreshTasks: document.querySelector("#refreshTasks"),
  themeToggles: document.querySelectorAll("[data-theme-toggle]"),
  themeIcons: document.querySelectorAll("[data-theme-icon]"),
  drawer: document.querySelector("#mobileDrawer"),
  drawerOpen: document.querySelector("[data-drawer-open]"),
  drawerClose: document.querySelector("[data-drawer-close]"),
  drawerBackdrop: document.querySelector("[data-drawer-backdrop]"),
  taskComposerShell: document.querySelector("#taskComposerShell"),
  taskForm: document.querySelector("#taskForm"),
  taskTitle: document.querySelector("#taskForm input[name='title']"),
  taskCounter: document.querySelector("#taskCounter"),
  loginLinks: document.querySelectorAll("[data-login-link]"),
  logoutButtons: document.querySelectorAll("[data-logout-button]"),
};

init();

function init() {
  hydrateIcons();
  els.logoutButtons.forEach((button) => button.addEventListener("click", handleLogout));
  bindTheme(state, els);
  const drawer = bindDrawer(els);
  els.refreshTasks.addEventListener("click", () => loadTaskView());
  els.taskForm.addEventListener("submit", handleCreateTask);
  els.taskForm.addEventListener("change", handleTaskFormChange);
  els.taskTitle.addEventListener("input", updateTaskCounter);
  els.tasksList.addEventListener("click", handleTaskClick);
  window.addEventListener("keydown", (event) => {
    if (event.key === "Escape") drawer.close();
  });

  updateTaskCounter();
  renderSession();
  hydrateInitialView();
}

async function hydrateInitialView() {
  await hydrateCurrentUser(false);
  await loadTaskView();
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
  setAuthStatus("Logged out.", "success");
  renderSession();
  await loadTaskView();
}

async function hydrateCurrentUser(showError = true) {
  if (!state.session?.user) return;
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
    if (showError) setAuthStatus(error.message, "error");
  }
}

function renderSession() {
  const user = state.session?.user;
  const loggedIn = renderSessionChrome(els, user);
  const isAdmin = Boolean(user?.is_admin);
  els.taskComposerShell.hidden = !isAdmin;
  els.tasksMeta.textContent = isAdmin
    ? "Create assigned recurring tasks"
    : loggedIn ? "Complete today's assigned tasks" : "Login to view assigned daily tasks";
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
  els.tasksList.innerHTML = `<div class="loading-state" aria-label="Loading tasks"></div>`;
  try {
    const response = await apiFetch("/api/tasks");
    state.tasks = response.tasks ?? [];
    renderTasks();
  } catch (error) {
    els.tasksList.innerHTML = `<div class="error-state">${escapeHTML(error.message)}</div>`;
    setAuthStatus(error.message, "error");
  }
}

async function loadDailyTasks() {
  els.tasksList.innerHTML = `<div class="loading-state" aria-label="Loading daily tasks"></div>`;
  try {
    const response = await apiFetch("/api/me/tasks", { auth: true });
    state.dailyTasks = response.tasks ?? [];
    renderTasks();
  } catch (error) {
    els.tasksList.innerHTML = `<div class="error-state">${escapeHTML(error.message)}</div>`;
    setAuthStatus(error.message, "error");
  }
}

async function loadUsers() {
  try {
    const response = await apiFetch("/api/users", { auth: true });
    state.users = response.users ?? [];
    renderAssignees();
  } catch (error) {
    setAuthStatus(error.message, "error");
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
    els.taskForm.querySelector("input[name='frequency_kind'][value='daily']").checked = true;
    els.taskForm.elements.is_active.checked = true;
    handleTaskFormChange();
    updateTaskCounter();
    els.taskTitle.focus();
    setAuthStatus("Task created.", "success");
    renderTasks();
  } catch (error) {
    setAuthStatus(error.message, "error");
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

  if (frequencyKind === "weekly" && daysOfWeek.length === 0) {
    setAuthStatus("Choose at least one day for weekly tasks.", "error");
    return null;
  }

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
    els.tasksList.innerHTML = `
      <div class="empty-state action-empty">
        <span>Login to view and complete your assigned daily tasks.</span>
        <a class="primary-action" href="/login?return_to=${encodeURIComponent(window.location.pathname)}">Login</a>
      </div>
    `;
    return;
  }
  if (tasks.length === 0) {
    els.tasksList.innerHTML = `<div class="empty-state">${isAdmin ? "No recurring tasks yet. Add one above to start assigning work." : "No daily tasks assigned for today. You are clear for now."}</div>`;
    return;
  }

  els.tasksList.innerHTML = tasks.map((task) => renderTask(task, isAdmin)).join("");
}

function renderTask(task, canAdmin) {
  const frequency = task.frequency_kind === "weekly" && task.days_of_week?.length
    ? `weekly: ${formatDays(task.days_of_week)}`
    : task.frequency_kind;
  const stateLabel = task.is_active ? "active" : "inactive";
  return `
    <article class="task-item ${task.is_active ? "is-active" : "is-inactive"}" data-task-id="${escapeHTML(task.id)}">
      <div class="task-main">
        <div>
          <h3>${escapeHTML(task.title)}</h3>
          <p>
            <span class="meta-pill">${escapeHTML(frequency)}</span>
            <span class="meta-pill">${task.point_value} pts</span>
            <span class="meta-pill ${task.is_active ? "is-live" : "is-muted"}">${stateLabel}</span>
            ${task.individual ? `<span class="meta-pill">individual</span>` : ""}
          </p>
        </div>
        ${canAdmin ? `
          <div class="task-actions">
            ${task.is_active
              ? `<button class="text-action danger" type="button" data-action="delete-task" data-task-id="${escapeHTML(task.id)}">Deactivate</button>`
              : `<span class="task-state-pill">Inactive</span>`}
          </div>
        ` : `
          <div class="task-actions">
            <button class="primary-action compact-action" type="button" data-action="complete-task" data-task-id="${escapeHTML(task.id)}">Complete +${task.point_value}</button>
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
    setAuthStatus("Task deactivated.", "success");
    renderTasks();
  } catch (error) {
    setAuthStatus(error.message, "error");
  }
}

async function completeTask(taskID, button) {
  if (!taskID) return;
  button.disabled = true;
  button.closest(".task-item")?.classList.add("is-completing");
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
    setAuthStatus(`Completed. +${response?.points_awarded ?? 0} points.`, "success");
    renderSession();
  } catch (error) {
    button.disabled = false;
    setAuthStatus(error.message, "error");
  }
}

function updateTaskCounter() {
  els.taskCounter.textContent = `${els.taskTitle.value.length}/200`;
}

function formatDays(days) {
  const labels = {
    1: "Mon",
    2: "Tue",
    3: "Wed",
    4: "Thu",
    5: "Fri",
    6: "Sat",
    7: "Sun",
  };
  return days.map((day) => labels[day] ?? day).join(", ");
}

function setAuthStatus(message, tone = "") {
  setStatus(els.authStatus, message, tone);
}
