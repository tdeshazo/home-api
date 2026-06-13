const state = {
  limit: 20,
  offset: 0,
  posts: [],
  hasMore: false,
  replies: {},
  session: readSession(),
};

const els = {
  sessionSummary: document.querySelector("#sessionSummary"),
  authStatus: document.querySelector("#authStatus"),
  timelineMeta: document.querySelector("#timelineMeta"),
  postsList: document.querySelector("#postsList"),
  pageSummary: document.querySelector("#pageSummary"),
  prevPage: document.querySelector("#prevPage"),
  nextPage: document.querySelector("#nextPage"),
  refreshPosts: document.querySelector("#refreshPosts"),
  postForm: document.querySelector("#postForm"),
  postBody: document.querySelector("#postForm textarea"),
  postCounter: document.querySelector("#postCounter"),
  loginForm: document.querySelector("#loginForm"),
  registerForm: document.querySelector("#registerForm"),
  logoutButton: document.querySelector("#logoutButton"),
  devLogin: document.querySelector("#devLogin"),
  devUserID: document.querySelector("#devUserID"),
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
  els.postForm.addEventListener("submit", handleCreatePost);
  els.postBody.addEventListener("input", updatePostCounter);
  els.postsList.addEventListener("click", handleTimelineClick);
  els.postsList.addEventListener("input", handleTimelineInput);
  els.postsList.addEventListener("submit", handleReplySubmit);
  els.refreshPosts.addEventListener("click", () => loadPosts());
  els.prevPage.addEventListener("click", () => {
    state.offset = Math.max(0, state.offset - state.limit);
    loadPosts();
  });
  els.nextPage.addEventListener("click", () => {
    if (!state.hasMore) return;
    state.offset += state.limit;
    loadPosts();
  });

  updatePostCounter();
  renderSession();
  loadPosts();
  if (state.session?.accessToken) {
    hydrateCurrentUser();
  }
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
    const response = await apiFetch("/auth/login", {
      method: "POST",
      body: {
        email: form.get("email"),
        password: form.get("password"),
      },
    });
    saveTokenSession(response);
    els.loginForm.reset();
    setAuthStatus("Logged in.");
    renderSession();
  } catch (error) {
    setAuthStatus(error.message);
  }
}

async function handleRegister(event) {
  event.preventDefault();
  const form = new FormData(els.registerForm);
  setAuthStatus("Creating account...");
  try {
    const response = await apiFetch("/auth/register", {
      method: "POST",
      body: {
        email: form.get("email"),
        handle: form.get("handle"),
        display_name: form.get("display_name"),
        password: form.get("password"),
      },
    });
    saveTokenSession(response);
    els.registerForm.reset();
    selectAuthTab("login");
    setAuthStatus("Account ready.");
    renderSession();
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

  state.session = {
    accessToken: `dev:${userID}`,
    refreshToken: "",
    tokenType: "Bearer",
    user: null,
  };
  writeSession(state.session);
  setAuthStatus("Dev token selected.");
  renderSession();
  await hydrateCurrentUser();
}

async function handleLogout() {
  const refreshToken = state.session?.refreshToken;
  if (refreshToken) {
    try {
      await apiFetch("/auth/logout", {
        method: "POST",
        body: { refresh_token: refreshToken },
      });
    } catch {
      // Local logout should still clear stale browser credentials.
    }
  }
  state.session = null;
  writeSession(null);
  setAuthStatus("Logged out.");
  renderSession();
}

async function hydrateCurrentUser() {
  try {
    const user = await apiFetch("/me", { auth: true });
    state.session = { ...state.session, user };
    writeSession(state.session);
    renderSession();
  } catch (error) {
    setAuthStatus(error.message);
  }
}

async function handleCreatePost(event) {
  event.preventDefault();
  const body = els.postBody.value.trim();
  if (!body) return;
  if (!state.session?.accessToken) {
    setAuthStatus("Login required.");
    return;
  }

  els.postForm.querySelector("button").disabled = true;
  try {
    await apiFetch("/posts", {
      method: "POST",
      auth: true,
      body: { body },
    });
    els.postForm.reset();
    updatePostCounter();
    state.offset = 0;
    await loadPosts();
  } catch (error) {
    setAuthStatus(error.message);
  } finally {
    els.postForm.querySelector("button").disabled = false;
  }
}

async function loadPosts() {
  els.postsList.innerHTML = `<div class="empty-state">Loading...</div>`;
  try {
    const params = new URLSearchParams({
      limit: String(state.limit + 1),
      offset: String(state.offset),
    });
    const response = await apiFetch(`/posts?${params.toString()}`);
    const posts = response.posts ?? [];
    state.hasMore = posts.length > state.limit;
    state.posts = posts.slice(0, state.limit);
    state.replies = {};
    renderPosts();
  } catch (error) {
    els.postsList.innerHTML = `<div class="error-state">${escapeHTML(error.message)}</div>`;
  }
}

function renderSession() {
  const user = state.session?.user;
  const loggedIn = Boolean(state.session?.accessToken);
  els.logoutButton.disabled = !loggedIn;
  els.postForm.classList.toggle("is-hidden", !loggedIn);
  els.sessionSummary.textContent = loggedIn
    ? user ? `@${user.handle}` : "Authenticated"
    : "Signed out";
  els.timelineMeta.textContent = loggedIn
    ? "Top-level posts"
    : "Top-level posts";
  renderPosts();
}

function renderPosts() {
  if (state.posts.length === 0) {
    els.postsList.innerHTML = `<div class="empty-state">No posts found.</div>`;
  } else {
    els.postsList.innerHTML = state.posts.map((post) => renderPost(post)).join("");
  }
  els.prevPage.disabled = state.offset === 0;
  els.nextPage.disabled = !state.hasMore;
  const page = Math.floor(state.offset / state.limit) + 1;
  els.pageSummary.textContent = `Page ${page}`;
}

function renderPost(post, depth = 0) {
  const initials = post.user_id.slice(0, 2).toUpperCase();
  const timestamp = formatDate(post.created_at);
  const replies = replyState(post.id);
  const expanded = replies.expanded;
  const canReply = Boolean(state.session?.accessToken);
  const className = depth === 0 ? "post" : "reply";
  const repliesHTML = expanded ? renderReplies(post.id, depth + 1) : "";
  const replyFormHTML = canReply ? renderReplyForm(post.id) : "";

  return `
    <article class="${className}" data-post-id="${escapeHTML(post.id)}">
      <div class="post-header">
        <span class="post-user">
          <span class="avatar">${escapeHTML(initials)}</span>
          <span>${escapeHTML(shortID(post.user_id))}</span>
        </span>
        <time datetime="${escapeHTML(post.created_at)}">${escapeHTML(timestamp)}</time>
      </div>
      <p class="post-body">${escapeHTML(post.body)}</p>
      <div class="post-actions">
        <button class="text-action" type="button" data-action="toggle-replies" data-post-id="${escapeHTML(post.id)}">
          ${expanded ? "Hide replies" : "Replies"}
        </button>
        ${canReply ? `<button class="text-action" type="button" data-action="toggle-reply-form" data-post-id="${escapeHTML(post.id)}">Reply</button>` : ""}
      </div>
      ${replyFormHTML}
      ${repliesHTML}
    </article>
  `;
}

function renderReplies(postID, depth) {
  const replies = replyState(postID);
  if (replies.loading) {
    return `<div class="reply-list"><div class="reply-note">Loading...</div></div>`;
  }
  if (replies.error) {
    return `<div class="reply-list"><div class="reply-note error">${escapeHTML(replies.error)}</div></div>`;
  }
  if (!replies.items.length) {
    return `<div class="reply-list"><div class="reply-note">No replies yet.</div></div>`;
  }
  return `
    <div class="reply-list">
      ${replies.items.map((reply) => renderPost(reply, depth)).join("")}
    </div>
  `;
}

function renderReplyForm(postID) {
  const replies = replyState(postID);
  return `
    <form class="reply-form ${replies.formOpen ? "" : "is-hidden"}" data-reply-form="${escapeHTML(postID)}">
      <textarea name="body" maxlength="280" rows="3" required></textarea>
      <div class="composer-actions">
        <span>0/280</span>
        <button class="primary-action" type="submit">Reply</button>
      </div>
    </form>
  `;
}

function handleTimelineClick(event) {
  const button = event.target.closest("[data-action]");
  if (!button) return;

  const postID = button.dataset.postId;
  if (!postID) return;

  if (button.dataset.action === "toggle-replies") {
    toggleReplies(postID);
    return;
  }

  if (button.dataset.action === "toggle-reply-form") {
    const replies = replyState(postID);
    replies.formOpen = !replies.formOpen;
    renderPosts();
  }
}

function handleTimelineInput(event) {
  const textarea = event.target.closest(".reply-form textarea");
  if (!textarea) return;
  const counter = textarea.closest(".reply-form").querySelector(".composer-actions span");
  counter.textContent = `${textarea.value.length}/280`;
}

async function handleReplySubmit(event) {
  const form = event.target.closest(".reply-form");
  if (!form) return;
  event.preventDefault();

  const postID = form.dataset.replyForm;
  const body = new FormData(form).get("body")?.trim();
  if (!postID || !body) return;

  const button = form.querySelector("button");
  button.disabled = true;
  try {
    await apiFetch(`/posts/${encodeURIComponent(postID)}/replies`, {
      method: "POST",
      auth: true,
      body: { body },
    });
    const replies = replyState(postID);
    replies.expanded = true;
    replies.formOpen = false;
    replies.hasLoaded = false;
    await loadReplies(postID);
  } catch (error) {
    setAuthStatus(error.message);
  } finally {
    button.disabled = false;
  }
}

async function toggleReplies(postID) {
  const replies = replyState(postID);
  replies.expanded = !replies.expanded;
  renderPosts();
  if (replies.expanded && !replies.hasLoaded) {
    await loadReplies(postID);
  }
}

async function loadReplies(postID) {
  const replies = replyState(postID);
  replies.loading = true;
  replies.error = "";
  renderPosts();
  try {
    const params = new URLSearchParams({ limit: "50", offset: "0" });
    const response = await apiFetch(`/posts/${encodeURIComponent(postID)}/replies?${params.toString()}`);
    replies.items = response.replies ?? [];
    replies.hasLoaded = true;
  } catch (error) {
    replies.error = error.message;
  } finally {
    replies.loading = false;
    renderPosts();
  }
}

function replyState(postID) {
  state.replies[postID] ||= {
    expanded: false,
    formOpen: false,
    loading: false,
    hasLoaded: false,
    error: "",
    items: [],
  };
  return state.replies[postID];
}

function updatePostCounter() {
  els.postCounter.textContent = `${els.postBody.value.length}/280`;
}

async function apiFetch(path, options = {}) {
  const headers = new Headers(options.headers ?? {});
  headers.set("Accept", "application/json");
  if (options.body) headers.set("Content-Type", "application/json");
  if (options.auth) {
    if (!state.session?.accessToken) throw new Error("Login required.");
    headers.set("Authorization", `${state.session.tokenType || "Bearer"} ${state.session.accessToken}`);
  }

  const response = await fetch(path, {
    method: options.method ?? "GET",
    headers,
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

function saveTokenSession(response) {
  state.session = {
    accessToken: response.access_token,
    refreshToken: response.refresh_token,
    tokenType: response.token_type || "Bearer",
    user: response.user,
  };
  writeSession(state.session);
}

function readSession() {
  try {
    return JSON.parse(localStorage.getItem("home-api-session"));
  } catch {
    return null;
  }
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

function shortID(value) {
  return value ? `${value.slice(0, 8)}...${value.slice(-4)}` : "unknown";
}

function formatDate(value) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return new Intl.DateTimeFormat(undefined, {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  }).format(date);
}

function escapeHTML(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}
