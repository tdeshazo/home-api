import {
  apiFetch,
  bindDrawer,
  bindTheme,
  escapeHTML,
  hydrateIcons,
  icon,
  isAuthError,
  readSession,
  readTheme,
  renderSessionChrome,
  setStatus,
  writeSession,
} from "./shared.js";

const state = {
  limit: 20,
  offset: 0,
  posts: [],
  hasMore: false,
  loadingPosts: false,
  replies: {},
  theme: readTheme(),
  session: readSession(),
};

const EMOJI_GROUPS = [
  { label: "Smileys", items: ["😀", "😄", "😂", "🤣", "😊", "😍", "🥰", "😎", "🤔", "😅", "😭", "😤"] },
  { label: "Gestures", items: ["👍", "👎", "👏", "🙌", "🙏", "🤝", "💪", "👀", "✌️", "🤞", "🤌", "🫶"] },
  { label: "Reactions", items: ["❤️", "🧡", "💛", "💚", "💙", "💜", "🔥", "✨", "💯", "✅", "❌", "⚠️"] },
  { label: "Objects", items: ["💡", "📌", "📣", "📝", "📚", "🔍", "🧠", "🛠️", "🚀", "🎯", "⏰", "☕"] },
  { label: "Nature", items: ["🌱", "🌿", "🌻", "🌙", "☀️", "⭐", "🌈", "⚡", "🌊", "🍕", "🍩", "🎉"] },
];

const els = {
  sessionSummaries: document.querySelectorAll("[data-session-summary]"),
  navProfiles: document.querySelectorAll("[data-nav-profile]"),
  navAvatars: document.querySelectorAll("[data-nav-avatar]"),
  navNames: document.querySelectorAll("[data-nav-name]"),
  authStatus: document.querySelector("#authStatus"),
  timelineMeta: document.querySelector("#timelineMeta"),
  postsList: document.querySelector("#postsList"),
  scrollStatus: document.querySelector("#scrollStatus"),
  refreshPosts: document.querySelector("#refreshPosts"),
  themeToggles: document.querySelectorAll("[data-theme-toggle]"),
  themeIcons: document.querySelectorAll("[data-theme-icon]"),
  drawer: document.querySelector("#mobileDrawer"),
  drawerOpen: document.querySelector("[data-drawer-open]"),
  drawerClose: document.querySelector("[data-drawer-close]"),
  drawerBackdrop: document.querySelector("[data-drawer-backdrop]"),
  postSignInPrompt: document.querySelector("#postSignInPrompt"),
  postForm: document.querySelector("#postForm"),
  postBody: document.querySelector("#postForm textarea"),
  postCounter: document.querySelector("#postCounter"),
  loginLinks: document.querySelectorAll("[data-login-link]"),
  logoutButtons: document.querySelectorAll("[data-logout-button]"),
};

init();

function init() {
  hydrateIcons();
  document.querySelectorAll("[data-emoji-popover]").forEach((popover) => {
    popover.innerHTML = renderEmojiPalette();
  });
  els.logoutButtons.forEach((button) => button.addEventListener("click", handleLogout));
  els.postForm.addEventListener("submit", handleCreatePost);
  els.postBody.addEventListener("input", updatePostCounter);
  els.postForm.addEventListener("click", handleComposerClick);
  els.postsList.addEventListener("click", handleTimelineClick);
  els.postsList.addEventListener("input", handleTimelineInput);
  els.postsList.addEventListener("submit", handleReplySubmit);
  bindTheme(state, els);
  const drawer = bindDrawer(els);
  els.refreshPosts.addEventListener("click", () => loadPosts({ reset: true }));
  window.addEventListener("scroll", maybeLoadMorePosts, { passive: true });
  window.addEventListener("keydown", (event) => {
    if (event.key === "Escape") drawer.close();
  });
  document.addEventListener("click", closeEmojiPopovers);

  updatePostCounter();
  renderSession();
  loadPosts({ reset: true });
  hydrateCurrentUser(false);
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

function handleComposerClick(event) {
  const emojiButton = event.target.closest("[data-emoji]");
  if (emojiButton) {
    insertEmoji(els.postBody, emojiButton.dataset.emoji);
    updatePostCounter();
    closeEmojiPopovers();
    return;
  }

  const toggle = event.target.closest("[data-action='toggle-emoji']");
  if (toggle) {
    event.stopPropagation();
    toggleEmojiPopover(toggle);
  }
}

async function handleCreatePost(event) {
  event.preventDefault();
  const body = els.postBody.value.trim();
  if (!body) return;
  if (!state.session?.user) {
    setAuthStatus("Login required to post.", "error");
    return;
  }

  els.postForm.querySelector("button").disabled = true;
  try {
    await apiFetch("/api/posts", {
      method: "POST",
      auth: true,
      body: { body },
    });
    els.postForm.reset();
    updatePostCounter();
    await loadPosts({ reset: true });
    setAuthStatus("Posted to the timeline.", "success");
  } catch (error) {
    setAuthStatus(error.message, "error");
  } finally {
    els.postForm.querySelector("button").disabled = false;
  }
}

async function loadPosts({ reset = false } = {}) {
  if (state.loadingPosts) return;
  if (!reset && !state.hasMore) return;

  if (reset) {
    state.offset = 0;
    state.posts = [];
    state.replies = {};
    state.hasMore = true;
    els.postsList.innerHTML = `<div class="loading-state" aria-label="Loading timeline"></div>`;
  }

  state.loadingPosts = true;
  renderScrollStatus();
  try {
    const params = new URLSearchParams({
      limit: String(state.limit + 1),
      offset: String(state.offset),
    });
    const response = await apiFetch(`/api/posts?${params.toString()}`);
    const posts = response.posts ?? [];
    state.hasMore = posts.length > state.limit;
    const visiblePosts = posts.slice(0, state.limit);
    state.posts = reset ? visiblePosts : [...state.posts, ...visiblePosts];
    state.offset += visiblePosts.length;
    renderPosts();
  } catch (error) {
    state.hasMore = false;
    if (reset || state.posts.length === 0) {
      els.postsList.innerHTML = `<div class="error-state">${escapeHTML(error.message)}</div>`;
    } else {
      setAuthStatus(error.message, "error");
    }
  } finally {
    state.loadingPosts = false;
    renderScrollStatus();
    queueMicrotask(maybeLoadMorePosts);
  }
}

function maybeLoadMorePosts() {
  if (state.loadingPosts || !state.hasMore) return;
  const scrollPosition = window.scrollY + window.innerHeight;
  const loadThreshold = document.documentElement.scrollHeight - 360;
  if (scrollPosition >= loadThreshold) {
    loadPosts();
  }
}

function renderSession() {
  const user = state.session?.user;
  const loggedIn = renderSessionChrome(els, user);
  els.postForm.classList.toggle("is-hidden", !loggedIn);
  els.postSignInPrompt.hidden = loggedIn;
  els.timelineMeta.textContent = loggedIn
    ? "Post and reply from your household timeline"
    : "Read-only until you login";
  renderPosts();
}

function renderPosts() {
  if (state.loadingPosts && state.posts.length === 0) {
    els.postsList.innerHTML = `<div class="loading-state" aria-label="Loading timeline"></div>`;
  } else if (state.posts.length === 0) {
    els.postsList.innerHTML = `<div class="empty-state">${state.session?.user ? "No posts yet. Start the first thread." : "No posts yet. Login to add the first update."}</div>`;
  } else {
    els.postsList.innerHTML = state.posts.map((post) => renderPost(post)).join("");
  }
  renderScrollStatus();
}

function renderScrollStatus() {
  els.scrollStatus.classList.remove("is-loading", "is-end");
  if (state.loadingPosts && state.posts.length > 0) {
    els.scrollStatus.classList.add("is-loading");
    els.scrollStatus.textContent = "Loading more posts";
    return;
  }
  if (!state.hasMore && state.posts.length > 0) {
    els.scrollStatus.classList.add("is-end");
    els.scrollStatus.textContent = "End of timeline";
    return;
  }
  els.scrollStatus.textContent = "";
}

function renderPost(post, depth = 0) {
  const author = authorName(post);
  const initials = initialsFor(author);
  const timestamp = formatDate(post.created_at);
  const replies = replyState(post.id);
  const expanded = replies.expanded;
  const canReply = Boolean(state.session?.user);
  const className = depth === 0 ? "post" : "reply";
  const repliesHTML = expanded ? renderReplies(post.id, depth + 1) : "";
  const replyFormHTML = canReply ? renderReplyForm(post.id) : "";
  const replyCount = post.reply_count ?? 0;
  const repliesDisabled = replyCount === 0;
  const repliesLabel = expanded ? "Hide replies" : `Replies${replyCount > 0 ? ` (${replyCount})` : ""}`;

  return `
    <article class="${className}" data-post-id="${escapeHTML(post.id)}">
      <div class="post-header">
        <span class="post-user">
          <span class="avatar">${escapeHTML(initials)}</span>
          <span>${escapeHTML(author)}</span>
        </span>
        <time datetime="${escapeHTML(post.created_at)}">${escapeHTML(timestamp)}</time>
      </div>
      <p class="post-body">${escapeHTML(post.body)}</p>
      <div class="post-actions">
        ${canReply ? `<button class="text-action" type="button" data-action="toggle-reply-form" data-post-id="${escapeHTML(post.id)}">Reply</button>` : ""}
        <button class="text-action" type="button" data-action="toggle-replies" data-post-id="${escapeHTML(post.id)}" ${repliesDisabled ? "disabled" : ""}>
          ${escapeHTML(repliesLabel)}
        </button>
      </div>
      ${replyFormHTML}
      ${repliesHTML}
    </article>
  `;
}

function renderReplies(postID, depth) {
  const replies = replyState(postID);
  if (replies.loading) {
    return `<div class="reply-list"><div class="reply-note">Loading replies...</div></div>`;
  }
  if (replies.error) {
    return `<div class="reply-list"><div class="reply-note error">${escapeHTML(replies.error)}</div></div>`;
  }
  if (!replies.items.length) {
    return `<div class="reply-list"><div class="reply-note">No replies yet. Keep the thread moving.</div></div>`;
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
        <div class="action-group">
          ${renderEmojiControl()}
          <button class="primary-action" type="submit">Reply</button>
        </div>
      </div>
    </form>
  `;
}

function handleTimelineClick(event) {
  const emojiButton = event.target.closest(".reply-form [data-emoji]");
  if (emojiButton) {
    const form = emojiButton.closest(".reply-form");
    const textarea = form.querySelector("textarea");
    insertEmoji(textarea, emojiButton.dataset.emoji);
    const counter = form.querySelector(".composer-actions span");
    counter.textContent = `${textarea.value.length}/280`;
    closeEmojiPopovers();
    return;
  }

  const emojiToggle = event.target.closest(".reply-form [data-action='toggle-emoji']");
  if (emojiToggle) {
    event.stopPropagation();
    toggleEmojiPopover(emojiToggle);
    return;
  }

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
    await apiFetch(`/api/posts/${encodeURIComponent(postID)}/replies`, {
      method: "POST",
      auth: true,
      body: { body },
    });
    const replies = replyState(postID);
    incrementReplyCount(postID);
    replies.expanded = true;
    replies.formOpen = false;
    replies.hasLoaded = false;
    await loadReplies(postID);
    setAuthStatus("Reply posted.", "success");
  } catch (error) {
    setAuthStatus(error.message, "error");
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
    const response = await apiFetch(`/api/posts/${encodeURIComponent(postID)}/replies?${params.toString()}`);
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

function incrementReplyCount(postID) {
  const post = findPostByID(postID);
  if (!post) return;
  post.reply_count = (post.reply_count ?? 0) + 1;
}

function findPostByID(postID) {
  const topLevelPost = state.posts.find((post) => post.id === postID);
  if (topLevelPost) return topLevelPost;

  for (const replies of Object.values(state.replies)) {
    const reply = replies.items.find((item) => item.id === postID);
    if (reply) return reply;
  }
  return null;
}

function updatePostCounter() {
  els.postCounter.textContent = `${els.postBody.value.length}/280`;
}

function insertEmoji(textarea, emoji) {
  if (!emoji) return;
  const start = textarea.selectionStart ?? textarea.value.length;
  const end = textarea.selectionEnd ?? textarea.value.length;
  const next = `${textarea.value.slice(0, start)}${emoji}${textarea.value.slice(end)}`;
  if (next.length > textarea.maxLength) return;
  textarea.value = next;
  const cursor = start + emoji.length;
  textarea.focus();
  textarea.setSelectionRange(cursor, cursor);
}

function renderEmojiControl() {
  return `
    <div class="emoji-control" data-emoji-control>
      <button class="icon-action" type="button" data-action="toggle-emoji" aria-label="Open emoji selector" aria-expanded="false">${icon("smile")}</button>
      <div class="emoji-popover is-hidden" data-emoji-popover>${renderEmojiPalette()}</div>
    </div>
  `;
}

function renderEmojiPalette() {
  return EMOJI_GROUPS.map((group) => `
    <div class="emoji-group">
      <div class="emoji-group-title">${escapeHTML(group.label)}</div>
      <div class="emoji-grid">
        ${group.items.map((emoji) => `<button type="button" data-emoji="${escapeHTML(emoji)}">${escapeHTML(emoji)}</button>`).join("")}
      </div>
    </div>
  `).join("");
}

function toggleEmojiPopover(toggle) {
  const control = toggle.closest("[data-emoji-control]");
  const popover = control.querySelector("[data-emoji-popover]");
  const willOpen = popover.classList.contains("is-hidden");
  closeEmojiPopovers();
  popover.classList.toggle("is-hidden", !willOpen);
  toggle.setAttribute("aria-expanded", String(willOpen));
}

function closeEmojiPopovers(event) {
  if (event?.target.closest("[data-emoji-control]")) return;
  document.querySelectorAll("[data-emoji-popover]").forEach((popover) => {
    popover.classList.add("is-hidden");
  });
  document.querySelectorAll("[data-action='toggle-emoji']").forEach((button) => {
    button.setAttribute("aria-expanded", "false");
  });
}

function setAuthStatus(message, tone = "") {
  setStatus(els.authStatus, message, tone);
}

function authorName(post) {
  return post.display_name || post.handle || "Unknown user";
}

function initialsFor(name) {
  const words = name.trim().split(/\s+/).filter(Boolean);
  if (!words.length) return "?";
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase();
  return `${words[0][0]}${words[1][0]}`.toUpperCase();
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
