export class APIError extends Error {
  constructor(message, status) {
    super(message);
    this.name = "APIError";
    this.status = status;
  }
}

const ICONS = {
  alert: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0Z"/><path d="M12 9v4"/><path d="M12 17h.01"/></svg>`,
  check: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m20 6-11 11-5-5"/></svg>`,
  close: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>`,
  info: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 16v-4"/><path d="M12 8h.01"/><path d="M22 12a10 10 0 1 1-20 0 10 10 0 0 1 20 0Z"/></svg>`,
  menu: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 7h16"/><path d="M4 12h16"/><path d="M4 17h16"/></svg>`,
  moon: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M20.9 13.4A8.2 8.2 0 0 1 10.6 3.1 9 9 0 1 0 20.9 13.4Z"/></svg>`,
  refresh: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M21 12a9 9 0 0 1-15.3 6.4L3 15.8"/><path d="M3 21v-5.2h5.2"/><path d="M3 12A9 9 0 0 1 18.3 5.6L21 8.2"/><path d="M21 3v5.2h-5.2"/></svg>`,
  smile: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M8 14s1.5 2 4 2 4-2 4-2"/><path d="M9 9h.01"/><path d="M15 9h.01"/><path d="M22 12a10 10 0 1 1-20 0 10 10 0 0 1 20 0Z"/></svg>`,
  sun: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 2v2"/><path d="M12 20v2"/><path d="m4.9 4.9 1.4 1.4"/><path d="m17.7 17.7 1.4 1.4"/><path d="M2 12h2"/><path d="M20 12h2"/><path d="m6.3 17.7-1.4 1.4"/><path d="m19.1 4.9-1.4 1.4"/><path d="M18 12a6 6 0 1 1-12 0 6 6 0 0 1 12 0Z"/></svg>`,
};

export function icon(name) {
  return ICONS[name] ?? "";
}

export function hydrateIcons(root = document) {
  root.querySelectorAll("[data-icon]").forEach((node) => {
    node.innerHTML = icon(node.dataset.icon);
  });
}

export async function apiFetch(path, options = {}) {
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

export function isAuthError(error) {
  return error instanceof APIError && (error.status === 401 || error.status === 403);
}

export function readSession() {
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

export function writeSession(session) {
  if (!session) {
    localStorage.removeItem("home-api-session");
    return;
  }
  localStorage.setItem("home-api-session", JSON.stringify(session));
}

export function readTheme() {
  const saved = localStorage.getItem("home-api-theme");
  if (saved === "dark" || saved === "light") return saved;
  return window.matchMedia?.("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

export function writeTheme(theme) {
  localStorage.setItem("home-api-theme", theme);
}

export function bindTheme(state, els) {
  const render = () => {
    document.documentElement.dataset.theme = state.theme;
    els.themeIcons?.forEach((icon) => {
      icon.innerHTML = state.theme === "dark" ? ICONS.sun : ICONS.moon;
    });
    els.themeToggles?.forEach((button) => {
      button.setAttribute("aria-pressed", String(state.theme === "dark"));
      button.setAttribute("title", state.theme === "dark" ? "Use light mode" : "Use dark mode");
      button.setAttribute("aria-label", state.theme === "dark" ? "Use light mode" : "Use dark mode");
    });
  };

  els.themeToggles?.forEach((button) => {
    button.addEventListener("click", () => {
      state.theme = state.theme === "dark" ? "light" : "dark";
      writeTheme(state.theme);
      render();
    });
  });
  render();
  return render;
}

export function bindDrawer(els) {
  let previousFocus = null;

  const focusableSelector = [
    "a[href]",
    "button:not([disabled])",
    "input:not([disabled])",
    "select:not([disabled])",
    "textarea:not([disabled])",
    "[tabindex]:not([tabindex='-1'])",
  ].join(",");

  const drawerIsOpen = () => Boolean(els.drawer?.classList.contains("is-open"));
  const drawerFocusables = () => Array.from(els.drawer?.querySelectorAll(focusableSelector) ?? [])
    .filter((node) => node.offsetParent !== null);

  const open = () => {
    previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    els.drawer?.classList.add("is-open");
    els.drawer?.setAttribute("aria-hidden", "false");
    if (els.drawerBackdrop) els.drawerBackdrop.hidden = false;
    els.drawerOpen?.setAttribute("aria-expanded", "true");
    document.body.classList.add("drawer-open");
    window.requestAnimationFrame(() => drawerFocusables()[0]?.focus());
  };

  const close = () => {
    if (!drawerIsOpen()) return;
    els.drawer?.classList.remove("is-open");
    els.drawer?.setAttribute("aria-hidden", "true");
    if (els.drawerBackdrop) els.drawerBackdrop.hidden = true;
    els.drawerOpen?.setAttribute("aria-expanded", "false");
    document.body.classList.remove("drawer-open");
    previousFocus?.focus?.();
    previousFocus = null;
  };

  els.drawerOpen?.addEventListener("click", open);
  els.drawerClose?.addEventListener("click", close);
  els.drawerBackdrop?.addEventListener("click", close);
  window.addEventListener("keydown", (event) => {
    if (event.key === "Escape") {
      close();
      return;
    }
    if (event.key !== "Tab" || !drawerIsOpen()) return;

    const focusables = drawerFocusables();
    if (!focusables.length) return;
    const first = focusables[0];
    const last = focusables.at(-1);
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  });

  return { open, close };
}

export function userInitials(user) {
  const source = user?.display_name || user?.handle || "?";
  return source
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0].toUpperCase())
    .join("");
}

export function renderSessionChrome(els, user, options = {}) {
  const loggedIn = Boolean(user);
  const returnPath = options.returnPath ?? window.location.pathname;

  els.loginLinks?.forEach((link) => {
    link.href = `/login?return_to=${encodeURIComponent(returnPath)}`;
    link.classList.toggle("is-hidden", loggedIn);
  });
  els.logoutButtons?.forEach((button) => button.classList.toggle("is-hidden", !loggedIn));
  els.navProfiles?.forEach((profile) => {
    profile.hidden = !loggedIn;
  });
  els.sessionSummaries?.forEach((summary) => {
    summary.textContent = loggedIn ? `@${user.handle} · ${user.points ?? 0} pts` : "Signed out";
  });
  els.navNames?.forEach((name) => {
    name.textContent = loggedIn ? user.display_name || user.handle : "";
  });
  els.navAvatars?.forEach((avatar) => {
    avatar.textContent = loggedIn ? userInitials(user) : "";
  });

  return loggedIn;
}

export function setStatus(element, message, tone = "") {
  if (!element) return;
  element.textContent = message;
  if (message && tone) {
    element.dataset.tone = tone;
  } else {
    element.removeAttribute("data-tone");
  }
}

export function escapeHTML(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

export function escapeAttribute(value) {
  return escapeHTML(value).replaceAll("`", "&#096;");
}

function apiPath(path) {
  if (path.startsWith("/api/")) return path;
  return `/api${path}`;
}
