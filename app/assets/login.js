import {
  apiFetch,
  bindDrawer,
  bindTheme,
  hydrateIcons,
  isAuthError,
  readSession,
  readTheme,
  renderSessionChrome,
  setStatus,
  writeSession,
} from "./shared.js";

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
  hydrateIcons();
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
  bindTheme(state, els);
  bindDrawer(els);

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
    setAuthStatus("Logged in.", "success");
    redirectAfterLogin();
  } catch (error) {
    setAuthStatus(error.message, "error");
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
    setAuthStatus("Account ready.", "success");
    redirectAfterLogin();
  } catch (error) {
    setAuthStatus(error.message, "error");
  }
}

async function handleDevLogin() {
  const userID = els.devUserID.value.trim();
  if (!userID) {
    setAuthStatus("User UUID is required.", "error");
    return;
  }

  setAuthStatus("Selecting dev user...");
  try {
    const response = await apiFetch("/api/auth/dev-login", {
      method: "POST",
      body: { user_id: userID },
    });
    saveSession(response.user);
    setAuthStatus("Dev user selected.", "success");
    redirectAfterLogin();
  } catch (error) {
    setAuthStatus(error.message, "error");
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
  setAuthStatus("Logged out.", "success");
  renderSession();
}

async function hydrateCurrentUser(showError = true) {
  if (!state.session?.user) return;
  try {
    const user = await apiFetch("/api/me", { auth: true });
    saveSession(user);
  } catch (error) {
    if (isAuthError(error)) {
      state.session = null;
      writeSession(null);
      renderSession();
    }
    if (showError) setAuthStatus(error.message, "error");
  }
}

function saveSession(user) {
  state.session = { user };
  writeSession(state.session);
  renderSession();
}

function renderSession() {
  const user = state.session?.user;
  renderSessionChrome(els, user);
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

function setAuthStatus(message, tone = "") {
  setStatus(els.authStatus, message, tone);
}
