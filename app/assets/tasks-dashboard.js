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
  assignees: [],
  activeUserID: "",
  theme: readTheme(),
  session: readSession(),
};

const els = {
  sessionSummaries: document.querySelectorAll("[data-session-summary]"),
  navProfiles: document.querySelectorAll("[data-nav-profile]"),
  navAvatars: document.querySelectorAll("[data-nav-avatar]"),
  navNames: document.querySelectorAll("[data-nav-name]"),
  authStatus: document.querySelector("#authStatus"),
  dashboardMeta: document.querySelector("#dashboardMeta"),
  assigneeTabs: document.querySelector("#assigneeTabs"),
  dashboardTasks: document.querySelector("#dashboardTasks"),
  refreshDashboard: document.querySelector("#refreshDashboard"),
  themeToggles: document.querySelectorAll("[data-theme-toggle]"),
  themeIcons: document.querySelectorAll("[data-theme-icon]"),
  drawer: document.querySelector("#mobileDrawer"),
  drawerOpen: document.querySelector("[data-drawer-open]"),
  drawerClose: document.querySelector("[data-drawer-close]"),
  drawerBackdrop: document.querySelector("[data-drawer-backdrop]"),
  loginLinks: document.querySelectorAll("[data-login-link]"),
  logoutButtons: document.querySelectorAll("[data-logout-button]"),
};

init();

function init() {
  hydrateIcons();
  els.logoutButtons.forEach((button) => button.addEventListener("click", handleLogout));
  bindTheme(state, els);
  const drawer = bindDrawer(els);
  els.refreshDashboard.addEventListener("click", loadDashboard);
  els.assigneeTabs.addEventListener("click", handleTabClick);
  els.assigneeTabs.addEventListener("keydown", handleTabKeydown);
  els.dashboardTasks.addEventListener("click", handleTaskClick);
  window.addEventListener("keydown", (event) => {
    if (event.key === "Escape") drawer.close();
  });

  renderSession();
  hydrateInitialView();
}

async function hydrateInitialView() {
  await hydrateCurrentUser(false);
  await loadDashboard();
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
  setAuthStatus("Logged out.", "success");
  renderSession();
  renderDashboard();
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
  renderSessionChrome(els, user);
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
    setAuthStatus("Admin access required.", "error");
    renderDashboard();
    return;
  }

  els.dashboardTasks.innerHTML = `<div class="loading-state" aria-label="Loading dashboard"></div>`;
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
    setAuthStatus(error.message, "error");
  }
}

function renderDashboard() {
  if (!state.session?.user) {
    els.dashboardMeta.textContent = "";
    els.assigneeTabs.innerHTML = "";
    els.dashboardTasks.innerHTML = `
      <div class="empty-state action-empty">
        <span>Login with an admin account to monitor and complete household assignments.</span>
        <a class="primary-action" href="/login?return_to=${encodeURIComponent(window.location.pathname)}">Login</a>
      </div>
    `;
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
    els.dashboardTasks.innerHTML = `<div class="empty-state">No assignments available today. Create recurring tasks or check back tomorrow.</div>`;
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
  const totalCount = openCount + completedCount;
  const progress = totalCount === 0 ? 100 : Math.round((completedCount / totalCount) * 100);
  return `
    <button class="assignee-tab${active ? " is-active" : ""}" type="button" role="tab" aria-selected="${active ? "true" : "false"}" tabindex="${active ? "0" : "-1"}" aria-controls="dashboardTasks" aria-label="${escapeAttribute(`${user.display_name}: ${openCount} open, ${completedCount} complete, ${user.points ?? 0} points`)}" data-user-id="${escapeAttribute(user.id)}">
      <span>${escapeHTML(user.display_name)}</span>
      <small>${openCount} open · ${completedCount} done · ${user.points ?? 0} pts</small>
      <span class="progress-track" aria-hidden="true"><span style="--progress: ${progress}%"></span></span>
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
      <button class="primary-action complete-action" type="button" data-action="complete-task" data-task-id="${escapeAttribute(task.id)}">Complete +${task.point_value}</button>
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

function handleTabKeydown(event) {
  const keys = ["ArrowLeft", "ArrowRight", "Home", "End"];
  if (!keys.includes(event.key)) return;

  const tabs = Array.from(els.assigneeTabs.querySelectorAll("[role='tab']"));
  if (!tabs.length) return;

  event.preventDefault();
  const currentIndex = Math.max(0, tabs.findIndex((tab) => tab.dataset.userId === state.activeUserID));
  let nextIndex = currentIndex;
  if (event.key === "ArrowLeft") nextIndex = (currentIndex - 1 + tabs.length) % tabs.length;
  if (event.key === "ArrowRight") nextIndex = (currentIndex + 1) % tabs.length;
  if (event.key === "Home") nextIndex = 0;
  if (event.key === "End") nextIndex = tabs.length - 1;

  state.activeUserID = tabs[nextIndex].dataset.userId;
  renderDashboard();
  window.requestAnimationFrame(() => {
    els.assigneeTabs.querySelector(`[data-user-id="${CSS.escape(state.activeUserID)}"]`)?.focus();
  });
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
  button.closest(".dashboard-task")?.classList.add("is-completing");
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
    setAuthStatus(`${assignee.user.display_name}: +${response?.points_awarded ?? 0} points.`, "success");
    renderDashboard();
  } catch (error) {
    button.disabled = false;
    setAuthStatus(error.message, "error");
  }
}

function activeAssignee() {
  return state.assignees.find((assignee) => assignee.user.id === state.activeUserID) ?? null;
}

function formatDateLabel(value) {
  const date = new Date(`${value}T00:00:00`);
  return date.toLocaleDateString(undefined, { weekday: "long", month: "short", day: "numeric" });
}

function formatTime(value) {
  return new Date(value).toLocaleTimeString(undefined, { hour: "numeric", minute: "2-digit" });
}

function setAuthStatus(message, tone = "") {
  setStatus(els.authStatus, message, tone);
}
