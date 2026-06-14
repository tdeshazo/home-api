const state = {
  theme: readTheme(),
  session: readSession(),
};

const els = {
  sessionSummaries: document.querySelectorAll("[data-session-summary]"),
  navProfiles: document.querySelectorAll("[data-nav-profile]"),
  navAvatars: document.querySelectorAll("[data-nav-avatar]"),
  navNames: document.querySelectorAll("[data-nav-name]"),
  authStatus: document.querySelector("#authStatus"),
  loginForm: document.querySelector("#loginForm"),
  registerForm: document.querySelector("#registerForm"),
  logoutButtons: document.querySelectorAll("[data-logout-button]"),
  devLogin: document.querySelector("#devLogin"),
  devUserID: document.querySelector("#devUserID"),
  themeToggles: document.querySelectorAll("[data-theme-toggle]"),
  themeIcons: document.querySelectorAll("[data-theme-icon]"),
  drawer: document.querySelector("#mobileDrawer"),
  drawerOpen: document.querySelector("[data-drawer-open]"),
  drawerClose: document.querySelector("[data-drawer-close]"),
  drawerBackdrop: document.querySelector("[data-drawer-backdrop]"),
  authTabs: Array.from(document.querySelectorAll("[data-auth-tab]")),
  authViews: Array.from(document.querySelectorAll("[data-auth-view]")),
  devUserButtons: Array.from(document.querySelectorAll("[data-dev-user]")),
};

init();

function init() {
  els.authTabs.forEach((button) => {
    button.addEventListener("click", () => selectAuthTab(button.dataset.authTab));
  });
  els.loginForm.addEventListener("submit", handleLogin);
  els.registerForm.addEventListener("submit", handleRegister);
  els.logoutButtons.forEach((button) => button.addEventListener("click", handleLogout));
  els.devLogin.addEventListener("click", handleDevLogin);
  els.devUserButtons.forEach((button) => {
    button.addEventListener("click", () => {
      els.devUserID.value = button.dataset.devUser;
      handleDevLogin();
    });
  });
  els.themeToggles.forEach((button) => button.addEventListener("click", toggleTheme));
  els.drawerOpen?.addEventListener("click", openDrawer);
  els.drawerClose?.addEventListener("click", closeDrawer);
  els.drawerBackdrop?.addEventListener("click", closeDrawer);
  window.addEventListener("keydown", (event) => {
    if (event.key === "Escape") closeDrawer();
  });

  renderTheme();
  renderSession();
  hydrateCurrentUser(false);
}

function selectAuthTab(tab) {
  els.authTabs.forEach((button) => {
    button.classList.toggle("is-active", button.dataset.authTab === tab);
  });
  els.authViews.forEach((view) => {
    view.classList.toggle("is-hidden", view.dataset.authView !== tab);
  });
  setAuthStatus("");
}

function toggleTheme() {
  state.theme = state.theme === "dark" ? "light" : "dark";
  writeTheme(state.theme);
  renderTheme();
}

function renderTheme() {
  document.documentElement.dataset.theme = state.theme;
  els.themeIcons.forEach((icon) => {
    icon.textContent = state.theme === "dark" ? "☀" : "◐";
  });
}

function openDrawer() {
  els.drawer?.classList.add("is-open");
  els.drawer?.setAttribute("aria-hidden", "false");
  if (els.drawerBackdrop) els.drawerBackdrop.hidden = false;
  els.drawerOpen?.setAttribute("aria-expanded", "true");
  document.body.classList.add("drawer-open");
}

function closeDrawer() {
  els.drawer?.classList.remove("is-open");
  els.drawer?.setAttribute("aria-hidden", "true");
  if (els.drawerBackdrop) els.drawerBackdrop.hidden = true;
  els.drawerOpen?.setAttribute("aria-expanded", "false");
  document.body.classList.remove("drawer-open");
}

async function handleLogin(event) {
  event.preventDefault();
  const form = new FormData(els.loginForm);
  setAuthStatus("Logging in...");
  try {
    const response = await apiFetch("/api/auth/login", {
      method: "POST",
      body: {
        email: form.get("email"),
        password: form.get("password"),
      },
    });
    saveSession(response.user);
    els.loginForm.reset();
    setAuthStatus("Logged in.");
    redirectAfterLogin();
  } catch (error) {
    setAuthStatus(error.message);
  }
}

async function handleRegister(event) {
  event.preventDefault();
  const form = new FormData(els.registerForm);
  setAuthStatus("Creating account...");
  try {
    const response = await apiFetch("/api/auth/register", {
      method: "POST",
      body: {
        email: form.get("email"),
        handle: form.get("handle"),
        display_name: form.get("display_name"),
        password: form.get("password"),
      },
    });
    saveSession(response.user);
    els.registerForm.reset();
    setAuthStatus("Account ready.");
    redirectAfterLogin();
  } catch (error) {
    setAuthStatus(error.message);
  }
}

async function handleDevLogin() {
  const userID = els.devUserID.value.trim();
  if (!userID) {
    setAuthStatus("User UUID is required.");
    return;
  }

  setAuthStatus("Selecting dev user...");
  try {
    const response = await apiFetch("/api/auth/dev-login", {
      method: "POST",
      body: { user_id: userID },
    });
    saveSession(response.user);
    setAuthStatus("Dev user selected.");
    redirectAfterLogin();
  } catch (error) {
    setAuthStatus(error.message);
  }
}

async function handleLogout() {
  try {
    await apiFetch("/api/auth/logout", { method: "POST" });
  } catch {
    // Local logout should still clear stale browser credentials.
  }
  state.session = null;
  writeSession(null);
  setAuthStatus("Logged out.");
  renderSession();
}

async function hydrateCurrentUser(showError = true) {
  try {
    const user = await apiFetch("/api/me", { auth: true });
    saveSession(user);
  } catch (error) {
    if (isAuthError(error)) {
      state.session = null;
      writeSession(null);
      renderSession();
    }
    if (showError) setAuthStatus(error.message);
  }
}

async function apiFetch(path, options = {}) {
  const headers = new Headers(options.headers ?? {});
  headers.set("Accept", "application/json");
  if (options.body) headers.set("Content-Type", "application/json");

  const response = await fetch(path, {
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

function saveSession(user) {
  state.session = { user };
  writeSession(state.session);
  renderSession();
}

function renderSession() {
  const user = state.session?.user;
  const loggedIn = Boolean(user);
  els.logoutButtons.forEach((button) => button.classList.toggle("is-hidden", !loggedIn));
  els.navProfiles.forEach((profile) => {
    profile.hidden = !loggedIn;
  });
  els.sessionSummaries.forEach((summary) => {
    summary.textContent = loggedIn ? `@${user.handle} · ${user.points ?? 0} pts` : "Signed out";
  });
  els.navNames.forEach((name) => {
    name.textContent = loggedIn ? user.display_name || user.handle : "";
  });
  els.navAvatars.forEach((avatar) => {
    avatar.textContent = loggedIn ? userInitials(user) : "";
  });
}

function userInitials(user) {
  const source = user.display_name || user.handle || "?";
  return source
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0].toUpperCase())
    .join("");
}

function redirectAfterLogin() {
  const params = new URLSearchParams(window.location.search);
  const returnTo = params.get("return_to");
  window.location.assign(safeReturnPath(returnTo));
}

function safeReturnPath(value) {
  if (!value || !value.startsWith("/") || value.startsWith("//")) return "/posts";
  return value;
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
