const state = {
  theme: readTheme(),
  session: readSession(),
};

const els = {
  sessionSummary: document.querySelector("#sessionSummary"),
  authStatus: document.querySelector("#authStatus"),
  loginForm: document.querySelector("#loginForm"),
  registerForm: document.querySelector("#registerForm"),
  logoutButton: document.querySelector("#logoutButton"),
  devLogin: document.querySelector("#devLogin"),
  devUserID: document.querySelector("#devUserID"),
  themeToggle: document.querySelector("#themeToggle"),
  themeIcon: document.querySelector("#themeIcon"),
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
  els.logoutButton.addEventListener("click", handleLogout);
  els.devLogin.addEventListener("click", handleDevLogin);
  els.devUserButtons.forEach((button) => {
    button.addEventListener("click", () => {
      els.devUserID.value = button.dataset.devUser;
      handleDevLogin();
    });
  });
  els.themeToggle.addEventListener("click", toggleTheme);

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
  els.themeIcon.textContent = state.theme === "dark" ? "☀" : "◐";
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
    const user = await apiFetch("/api/me");
    saveSession(user);
  } catch (error) {
    state.session = null;
    writeSession(null);
    renderSession();
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
  if (!response.ok) {
    throw new Error(payload?.error || `Request failed with status ${response.status}`);
  }
  return payload;
}

function saveSession(user) {
  state.session = { user };
  writeSession(state.session);
  renderSession();
}

function renderSession() {
  const user = state.session?.user;
  const loggedIn = Boolean(user);
  els.logoutButton.classList.toggle("is-hidden", !loggedIn);
  els.sessionSummary.textContent = loggedIn
    ? `@${user.handle} · ${user.points ?? 0} pts`
    : "Signed out";
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
