/*
 * Job Hunt — minimal client glue.
 *
 * Responsibilities:
 *   1. Theme toggle (dark/light) — persisted in localStorage.
 *   2. Mobile sidebar open/close with focus management + ESC + backdrop.
 *   3. Global HTMX UX — loading, confirm, modals, optimistic UI, errors.
 *   4. Toast renderer driven by the `toast` HX-Trigger event.
 *   5. Keyboard shortcuts for common actions.
 *
 * No frameworks. No build step.
 */

(function () {
    'use strict';

    const root = document.documentElement;

    // ------------------------------------------------------------------
    // 1. Theme toggle
    // ------------------------------------------------------------------

    function applyTheme(theme) {
        root.setAttribute('data-theme', theme);
        const dark = theme === 'dark';
        document.querySelectorAll('[data-theme-icon]').forEach((el) => {
            el.hidden = (el.dataset.themeIcon === 'dark') ? !dark : dark;
        });
    }

    function currentTheme() {
        return root.getAttribute('data-theme') || 'dark';
    }

    function toggleTheme() {
        const next = currentTheme() === 'dark' ? 'light' : 'dark';
        try { localStorage.setItem('theme', next); } catch (_) {}
        applyTheme(next);
    }

    applyTheme(currentTheme());

    // ------------------------------------------------------------------
    // 2. Mobile sidebar
    // ------------------------------------------------------------------

    const sidebar = document.getElementById('sidebar');
    let backdrop = null;
    let lastFocused = null;

    function ensureBackdrop() {
        if (backdrop) return backdrop;
        backdrop = document.createElement('div');
        backdrop.className = 'sidebar-backdrop';
        backdrop.addEventListener('click', closeSidebar);
        document.body.appendChild(backdrop);
        return backdrop;
    }

    function openSidebar() {
        if (!sidebar) return;
        lastFocused = document.activeElement;
        sidebar.classList.add('is-open');
        ensureBackdrop().classList.add('is-visible');
        const btn = document.querySelector('[data-action="toggle-sidebar"]');
        if (btn) btn.setAttribute('aria-expanded', 'true');
        const firstLink = sidebar.querySelector('a, button');
        if (firstLink) firstLink.focus();
    }

    function closeSidebar() {
        if (!sidebar) return;
        sidebar.classList.remove('is-open');
        if (backdrop) backdrop.classList.remove('is-visible');
        const btn = document.querySelector('[data-action="toggle-sidebar"]');
        if (btn) btn.setAttribute('aria-expanded', 'false');
        if (lastFocused && typeof lastFocused.focus === 'function') {
            lastFocused.focus();
            lastFocused = null;
        }
    }

    function isSidebarOpen() {
        return sidebar && sidebar.classList.contains('is-open');
    }

    // ------------------------------------------------------------------
    // 3. Toast
    // ------------------------------------------------------------------

    function showToast(opts) {
        const region = document.getElementById('toast-region');
        if (!region) return;
        const tone = (opts && opts.tone) || 'info';
        const message = (opts && opts.message) || '';
        const title = opts && opts.title;
        const duration = (opts && opts.duration) || 3500;

        const el = document.createElement('div');
        el.className = 'toast toast--' + tone;
        el.setAttribute('role', 'alert');
        if (title) {
            const t = document.createElement('strong');
            t.className = 'toast-title';
            t.textContent = title;
            el.appendChild(t);
        }
        const m = document.createElement('span');
        m.className = 'toast-message';
        m.textContent = message;
        el.appendChild(m);
        region.appendChild(el);

        setTimeout(function () {
            el.classList.add('toast--leaving');
            setTimeout(function () { el.remove(); }, 200);
        }, duration);
    }

    document.body.addEventListener('toast', function (e) {
        showToast(e.detail || {});
    });
    window.JobHuntToast = showToast;

    // ------------------------------------------------------------------
    // 3b. Task timers (in-progress session tracking)
    // ------------------------------------------------------------------

    let taskTimerInterval = null;

    function formatElapsedSeconds(totalSec) {
        if (totalSec < 0) totalSec = 0;
        const h = Math.floor(totalSec / 3600);
        const m = Math.floor((totalSec % 3600) / 60);
        const s = totalSec % 60;
        if (h > 0) return h + 'h ' + String(m).padStart(2, '0') + 'm';
        if (m > 0) return m + 'm ' + String(s).padStart(2, '0') + 's';
        return s + 's';
    }

    function elapsedFromTimerEl(el) {
        const startedMs = parseInt(el.dataset.startedAt, 10) * 1000;
        if (!startedMs) return 0;
        const pausedSec = parseInt(el.dataset.pausedSeconds || '0', 10) || 0;
        const nowMs = Date.now();
        let elapsed = Math.floor((nowMs - startedMs) / 1000) - pausedSec;
        if (el.dataset.paused === 'true' && el.dataset.pausedAt) {
            const pausedAtMs = parseInt(el.dataset.pausedAt, 10) * 1000;
            if (pausedAtMs) {
                elapsed -= Math.floor((nowMs - pausedAtMs) / 1000);
            }
        }
        return elapsed;
    }

    function tickTaskTimers() {
        document.querySelectorAll('[data-task-timer]').forEach(function (el) {
            el.textContent = formatElapsedSeconds(elapsedFromTimerEl(el));
        });
    }

    function syncTaskTimerInterval() {
        const hasTimers = document.querySelector('[data-task-timer]');
        if (hasTimers && !taskTimerInterval) {
            taskTimerInterval = setInterval(tickTaskTimers, 1000);
            tickTaskTimers();
        } else if (!hasTimers && taskTimerInterval) {
            clearInterval(taskTimerInterval);
            taskTimerInterval = null;
        }
    }

    // ------------------------------------------------------------------
    // 4. Global loading indicator
    // ------------------------------------------------------------------

    const globalIndicator = document.getElementById('htmx-global-indicator');
    let pendingRequests = 0;

    function setGlobalLoading(active) {
        if (!globalIndicator) return;
        globalIndicator.classList.toggle('is-active', active);
        globalIndicator.setAttribute('aria-hidden', active ? 'false' : 'true');
    }

    // ------------------------------------------------------------------
    // 5. Styled confirm dialog (replaces native hx-confirm)
    // ------------------------------------------------------------------

    const confirmModal = document.getElementById('confirm-modal');
    const confirmMessage = document.getElementById('confirm-modal-message');
    const confirmOk = document.getElementById('confirm-modal-ok');
    let confirmCallback = null;
    let pendingConfirmRequest = null;

    function openConfirm(opts) {
        if (!confirmModal) {
            if (opts.onConfirm) opts.onConfirm();
            return;
        }
        confirmCallback = opts;
        if (confirmMessage) confirmMessage.textContent = opts.message || 'Are you sure?';
        if (confirmOk) {
            confirmOk.textContent = opts.confirmLabel || 'Confirm';
            confirmOk.classList.toggle('btn-danger', !!opts.danger);
            confirmOk.classList.toggle('btn-primary', !opts.danger);
        }
        confirmModal.hidden = false;
        confirmModal.classList.add('is-open');
        confirmModal.setAttribute('aria-hidden', 'false');
        document.body.classList.add('modal-open');
        if (confirmOk) confirmOk.focus();
    }

    function closeConfirm(confirmed) {
        if (!confirmModal || !confirmModal.classList.contains('is-open')) return false;
        const cb = confirmCallback;
        confirmCallback = null;
        pendingConfirmRequest = null;
        confirmModal.classList.remove('is-open');
        confirmModal.hidden = true;
        confirmModal.setAttribute('aria-hidden', 'true');
        if (!document.getElementById('task-modal') || !document.getElementById('task-modal').innerHTML.trim()) {
            document.body.classList.remove('modal-open');
        }
        if (cb) {
            if (confirmed && cb.onConfirm) cb.onConfirm();
            else if (!confirmed && cb.onCancel) cb.onCancel();
        }
        return true;
    }

    function isConfirmOpen() {
        return !!(confirmModal && confirmModal.classList.contains('is-open'));
    }

    function isTasksListNav(elt) {
        return !!(elt && (elt.dataset.listNav === 'true' || elt.closest('[data-list-nav="true"]')));
    }

    // Used by hx-vals on #tasks-list-host to preserve the active tab/filters
    // when tasks-changed triggers a list refresh.
    window.tasksQueryParams = function () {
        const p = new URLSearchParams(window.location.search);
        return {
            view: p.get('view') || 'today',
            sort: p.get('sort') || 'due_date',
            dir: p.get('dir') || 'asc',
            status: p.get('status') || '',
            priority: p.get('priority') || '',
            category: p.get('category') || '',
            q: p.get('q') || '',
        };
    };

    document.body.addEventListener('htmx:confirm', function (e) {
        const elt = e.detail.elt;
        if (!elt || !elt.getAttribute('hx-confirm')) {
            return;
        }
        e.preventDefault();
        if (isConfirmOpen()) {
            e.detail.issueRequest(false);
            return;
        }
        pendingConfirmRequest = e.detail;
        const isDanger = !!(elt.getAttribute('hx-delete') ||
            elt.dataset.confirmDanger === 'true' ||
            elt.classList.contains('btn-danger-ghost') ||
            elt.classList.contains('icon-button--danger'));
        openConfirm({
            message: e.detail.question || 'Are you sure?',
            danger: isDanger,
            confirmLabel: isDanger ? 'Delete' : 'Confirm',
            onConfirm: function () {
                if (pendingConfirmRequest) pendingConfirmRequest.issueRequest(true);
                pendingConfirmRequest = null;
            },
            onCancel: function () {
                if (pendingConfirmRequest) pendingConfirmRequest.issueRequest(false);
                pendingConfirmRequest = null;
            },
        });
    });

    // ------------------------------------------------------------------
    // 6. Modal manager (#task-modal and similar slots)
    // ------------------------------------------------------------------

    function closeModalSlot() {
        const slot = document.getElementById('task-modal');
        if (slot) slot.innerHTML = '';
        document.body.classList.remove('modal-open');
    }

    function bindModalSlot(slot) {
        if (!slot || !slot.innerHTML.trim()) return;
        const dialog = slot.querySelector('.modal');
        if (!dialog) return;
        document.body.classList.add('modal-open');
        dialog.classList.add('is-entering');
        setTimeout(function () { dialog.classList.remove('is-entering'); }, 220);
        dialog.focus();
        if (window.htmx && typeof htmx.process === 'function') {
            htmx.process(slot);
        }
        const firstInput = slot.querySelector('input, textarea, select');
        if (firstInput) firstInput.focus();
    }

    function openTaskForm(url) {
        const slot = document.getElementById('task-modal');
        if (!slot || !url) return;

        pendingRequests++;
        setGlobalLoading(true);

        fetch(url, {
            method: 'GET',
            credentials: 'same-origin',
            headers: {
                'HX-Request': 'true',
                'Accept': 'text/html',
            },
        }).then(function (resp) {
            return resp.text().then(function (html) {
                return { ok: resp.ok, html: html };
            });
        }).then(function (result) {
            if (!result.ok) {
                showToast({ tone: 'danger', message: 'Could not open the task form.' });
                return;
            }
            slot.innerHTML = result.html;
            bindModalSlot(slot);
        }).catch(function () {
            showToast({ tone: 'danger', message: 'Network error — check your connection.' });
        }).finally(function () {
            pendingRequests = Math.max(0, pendingRequests - 1);
            if (pendingRequests === 0) setGlobalLoading(false);
        });
    }

    function submitTaskForm(form) {
        const url = form.dataset.action;
        const method = (form.dataset.method || 'post').toUpperCase();
        if (!url) return;

        const submitBtn = form.querySelector('button[type="submit"]');
        if (submitBtn) submitBtn.classList.add('htmx-request');
        pendingRequests++;
        setGlobalLoading(true);

        fetch(url, {
            method: method,
            credentials: 'same-origin',
            headers: {
                'HX-Request': 'true',
                'Accept': 'text/html',
            },
            body: new FormData(form),
        }).then(function (resp) {
            return resp.text().then(function (html) {
                return {
                    ok: resp.ok,
                    status: resp.status,
                    html: html,
                    trigger: resp.headers.get('HX-Trigger'),
                };
            });
        }).then(function (result) {
            if (result.status === 422) {
                const slot = document.getElementById('task-modal');
                if (slot) {
                    slot.innerHTML = result.html;
                    bindModalSlot(slot);
                }
                return;
            }
            if (!result.ok) {
                dispatchHXTrigger(result.trigger);
                if (!result.trigger) {
                    showToast({ tone: 'danger', message: 'Could not save the task.' });
                }
                return;
            }
            dispatchHXTrigger(result.trigger);
            applyOOBSwaps(result.html);
            closeModalSlot();
        }).catch(function () {
            showToast({ tone: 'danger', message: 'Network error — check your connection.' });
        }).finally(function () {
            if (submitBtn) submitBtn.classList.remove('htmx-request');
            pendingRequests = Math.max(0, pendingRequests - 1);
            if (pendingRequests === 0) setGlobalLoading(false);
            syncTaskTimerInterval();
        });
    }

    // ------------------------------------------------------------------
    // 7. Optimistic UI
    // ------------------------------------------------------------------

    const optimisticSnapshots = new WeakMap();

    function applyOptimistic(elt) {
        const kind = elt && elt.dataset.optimistic;
        if (!kind) return;
        const targetSel = elt.getAttribute('hx-target');
        let target = targetSel ? document.querySelector(targetSel) : null;
        if (!target) target = elt.closest('tr, .task-card');
        if (!target) return;
        optimisticSnapshots.set(elt, {
            html: target.outerHTML,
            target: target,
        });
        target.classList.add('htmx-optimistic', 'htmx-optimistic--' + kind);
        const id = target.id && target.id.split('-').pop();
        let peer = null;
        if (id) {
            peer = document.getElementById(
                target.id.startsWith('task-row-') ? 'task-card-' + id : 'task-row-' + id
            );
        }
        if (kind === 'delete') {
            target.classList.add('htmx-optimistic--removing');
            if (peer) peer.classList.add('htmx-optimistic', 'htmx-optimistic--delete', 'htmx-optimistic--removing');
        }
        if (kind === 'complete') {
            target.classList.add('task-row--status-completed', 'task-card--status-completed');
            if (peer) peer.classList.add('htmx-optimistic', 'htmx-optimistic--complete', 'task-row--status-completed', 'task-card--status-completed');
        }
        if (kind === 'in_progress') {
            target.classList.add('task-row--status-in_progress', 'task-card--status-in_progress');
            if (peer) peer.classList.add('htmx-optimistic', 'htmx-optimistic--in_progress', 'task-row--status-in_progress', 'task-card--status-in_progress');
        }
        if (kind === 'missed') {
            target.classList.add('task-row--status-missed', 'task-card--status-missed');
            if (peer) peer.classList.add('htmx-optimistic', 'htmx-optimistic--missed', 'task-row--status-missed', 'task-card--status-missed');
        }
        if (kind === 'pending') {
            target.classList.remove('task-row--status-in_progress', 'task-card--status-in_progress');
            target.classList.add('task-row--status-pending', 'task-card--status-pending');
            if (peer) {
                peer.classList.remove('task-row--status-in_progress', 'task-card--status-in_progress');
                peer.classList.add('htmx-optimistic', 'htmx-optimistic--pending', 'task-row--status-pending', 'task-card--status-pending');
            }
        }
    }

    function revertOptimistic(elt) {
        const snap = optimisticSnapshots.get(elt);
        if (!snap || !snap.target || !snap.target.isConnected) {
            optimisticSnapshots.delete(elt);
            return;
        }
        snap.target.outerHTML = snap.html;
        optimisticSnapshots.delete(elt);
    }

    // ------------------------------------------------------------------
    // 8. HTMX hooks
    // ------------------------------------------------------------------

    function refreshActiveNav() {
        const path = window.location.pathname;
        document.querySelectorAll('[data-nav]').forEach((link) => {
            const isActive = link.dataset.nav === path;
            link.classList.toggle('active', isActive);
            if (isActive) link.setAttribute('aria-current', 'page');
            else link.removeAttribute('aria-current');
        });
    }

    function updateNavbarTitle() {
        const active = document.querySelector('[data-nav].active span');
        const navTitle = document.querySelector('.navbar-title');
        if (active && navTitle) {
            navTitle.textContent = active.textContent.trim();
            document.title = navTitle.textContent + ' · Job Hunt';
        }
    }

    function friendlyError(evt) {
        const xhr = evt.detail && evt.detail.xhr;
        if (!xhr) return 'Network error — check your connection.';
        if (xhr.status === 0) return 'Request failed — you may be offline.';
        if (xhr.status >= 500) return 'Something went wrong on the server.';
        if (xhr.status === 404) return 'That resource was not found.';
        if (xhr.status === 422) return 'Please fix the highlighted errors.';
        return 'Request failed (' + xhr.status + ').';
    }

    function maybeInjectRetry(evt) {
        const target = evt.detail && evt.detail.target;
        const elt = evt.detail && evt.detail.elt;
        if (!target || !elt || target.id !== 'tasks-list') return;
        if (isTasksListNav(elt)) return;
        if (target.querySelector('.retry-state')) return;
        const url = elt.getAttribute('hx-get') || elt.getAttribute('hx-post') || '';
        if (!url) return;
        const retry = document.createElement('div');
        retry.className = 'retry-state';
        retry.setAttribute('role', 'alert');
        retry.innerHTML =
            '<div class="retry-state-copy"><span>Could not load tasks.</span></div>' +
            '<button type="button" class="btn btn-ghost btn-sm" ' +
            'hx-get="' + url + '" hx-target="#tasks-list" hx-swap="outerHTML">' +
            'Retry</button>';
        target.prepend(retry);
        if (window.htmx) htmx.process(retry);
    }

    document.body.addEventListener('htmx:beforeRequest', function (e) {
        const elt = e.detail.elt;
        if (isTasksListNav(elt) && isConfirmOpen()) {
            e.preventDefault();
            return;
        }
        pendingRequests++;
        setGlobalLoading(true);
        if (elt) {
            elt.classList.add('htmx-request');
            applyOptimistic(elt);
        }
        const target = e.detail.target;
        if (target) target.classList.add('htmx-loading');
    });

    document.body.addEventListener('htmx:afterRequest', function (e) {
        pendingRequests = Math.max(0, pendingRequests - 1);
        if (pendingRequests === 0) setGlobalLoading(false);
        const elt = e.detail.elt;
        if (elt) elt.classList.remove('htmx-request');
        const target = e.detail.target;
        if (target) target.classList.remove('htmx-loading');
        if (!e.detail.successful) {
            revertOptimistic(elt);
            if (!isTasksListNav(elt)) {
                showToast({ tone: 'danger', message: friendlyError(e) });
                maybeInjectRetry(e);
            }
        } else {
            optimisticSnapshots.delete(elt);
        }
    });

    document.body.addEventListener('htmx:beforeSwap', function (e) {
        if (e.detail.target) e.detail.target.classList.add('htmx-swapping');
    });

    function syncTasksFilterFromURL() {
        const p = new URLSearchParams(window.location.search);
        const form = document.querySelector('.filter-bar');
        if (!form) return;
        const viewInput = form.querySelector('input[name="view"]');
        const sortInput = form.querySelector('input[name="sort"]');
        const dirInput = form.querySelector('input[name="dir"]');
        if (viewInput) viewInput.value = p.get('view') || 'today';
        if (sortInput) sortInput.value = p.get('sort') || 'due_date';
        if (dirInput) dirInput.value = p.get('dir') || 'asc';
    }

    function syncTasksTabActive() {
        if (!window.location.pathname.startsWith('/tasks')) return;
        const view = new URLSearchParams(window.location.search).get('view') || 'today';
        document.querySelectorAll('.tasks-tabs .tab').forEach(function (tab) {
            const active = tab.dataset.view === view;
            tab.classList.toggle('tab--active', active);
            if (active) tab.setAttribute('aria-current', 'page');
            else tab.removeAttribute('aria-current');
        });
        syncTasksFilterFromURL();
    }

    function finishTasksListSwap(html, pageUrl) {
        const list = document.getElementById('tasks-list');
        if (!list) return false;
        list.outerHTML = html;
        if (pageUrl && window.history && history.pushState) {
            history.pushState({}, '', pageUrl);
        }
        syncTasksTabActive();
        const newList = document.getElementById('tasks-list');
        if (newList && window.htmx) htmx.process(newList);
        syncTaskTimerInterval();
        return true;
    }

    function loadTasksListNav(link) {
        const listUrl = link.getAttribute('hx-get');
        if (!listUrl || link.getAttribute('hx-target') !== '#tasks-list') return;
        if (link.classList.contains('htmx-request')) return;

        const pageUrl = link.getAttribute('hx-push-url') || link.getAttribute('href') || listUrl;
        link.classList.add('htmx-request');
        pendingRequests++;
        setGlobalLoading(true);
        const list = document.getElementById('tasks-list');
        if (list) list.classList.add('htmx-loading');

        fetch(listUrl, {
            method: 'GET',
            credentials: 'same-origin',
            headers: {
                'HX-Request': 'true',
                'Accept': 'text/html',
            },
        }).then(function (resp) {
            return resp.text().then(function (html) {
                return { ok: resp.ok, html: html, status: resp.status };
            });
        }).then(function (result) {
            if (!result.ok) {
                showToast({
                    tone: 'danger',
                    message: result.status >= 500
                        ? 'Something went wrong on the server.'
                        : 'Could not load tasks.',
                });
                return;
            }
            finishTasksListSwap(result.html, pageUrl);
        }).catch(function () {
            showToast({ tone: 'danger', message: 'Network error — check your connection.' });
        }).finally(function () {
            link.classList.remove('htmx-request');
            pendingRequests = Math.max(0, pendingRequests - 1);
            if (pendingRequests === 0) setGlobalLoading(false);
            const newList = document.getElementById('tasks-list');
            if (newList) newList.classList.remove('htmx-loading');
        });
    }

    document.body.addEventListener('htmx:afterSettle', function (e) {
        if (e.detail.target) e.detail.target.classList.remove('htmx-swapping');
        if (e.detail.target && e.detail.target.id === 'tasks-list') {
            syncTasksTabActive();
            if (window.htmx) htmx.process(e.detail.target);
            syncTaskTimerInterval();
        }
        if (e.detail.target && e.detail.target.id === 'main-content') {
            refreshActiveNav();
            updateNavbarTitle();
            const main = document.getElementById('main-content');
            if (main) main.focus({ preventScroll: true });
        }
        if (e.detail.target && e.detail.target.id === 'task-modal') {
            bindModalSlot(e.detail.target);
        }
        syncTaskTimerInterval();
    });

    // ------------------------------------------------------------------
    // 9. Delegated actions
    // ------------------------------------------------------------------

    function dispatchHXTrigger(header) {
        if (!header) return;
        try {
            var events = JSON.parse(header);
            Object.keys(events).forEach(function (name) {
                document.body.dispatchEvent(new CustomEvent(name, {
                    detail: events[name],
                    bubbles: true,
                }));
            });
        } catch (_) { /* ignore malformed trigger JSON */ }
    }

    function applyOOBSwaps(html) {
        if (!html || !html.trim()) return;
        var tpl = document.createElement('template');
        tpl.innerHTML = html.trim();
        tpl.content.querySelectorAll('[hx-swap-oob]').forEach(function (el) {
            var mode = el.getAttribute('hx-swap-oob');
            var id = el.id;
            if (!id) return;
            var target = document.getElementById(id);
            if (!target) return;
            var clean = el.cloneNode(true);
            clean.removeAttribute('hx-swap-oob');
            if (mode === 'delete') {
                target.remove();
            } else if (mode === 'outerHTML') {
                target.outerHTML = clean.outerHTML;
            } else if (mode === 'innerHTML') {
                target.innerHTML = clean.innerHTML;
            }
        });
        if (window.htmx && typeof htmx.process === 'function') {
            htmx.process(document.body);
        }
    }

    function runTaskAction(btn) {
        var id = btn.dataset.taskId;
        var action = btn.dataset.taskAction;
        if (!id || !action) return;

        btn.classList.add('htmx-request');
        applyOptimistic(btn);
        pendingRequests++;
        setGlobalLoading(true);

        fetch('/tasks/' + encodeURIComponent(id) + '/' + action, {
            method: 'POST',
            credentials: 'same-origin',
            headers: {
                'HX-Request': 'true',
                'Accept': 'text/html',
            },
        }).then(function (resp) {
            return resp.text().then(function (html) {
                return {
                    ok: resp.ok,
                    status: resp.status,
                    html: html,
                    trigger: resp.headers.get('HX-Trigger'),
                };
            });
        }).then(function (result) {
            if (!result.ok) {
                revertOptimistic(btn);
                dispatchHXTrigger(result.trigger);
                if (!result.trigger) {
                    showToast({
                        tone: 'danger',
                        message: result.status >= 500
                            ? 'Something went wrong on the server.'
                            : 'Could not update the task.',
                    });
                }
                return;
            }
            optimisticSnapshots.delete(btn);
            dispatchHXTrigger(result.trigger);
            applyOOBSwaps(result.html);
        }).catch(function () {
            revertOptimistic(btn);
            showToast({ tone: 'danger', message: 'Network error — check your connection.' });
        }).finally(function () {
            btn.classList.remove('htmx-request');
            pendingRequests = Math.max(0, pendingRequests - 1);
            if (pendingRequests === 0) setGlobalLoading(false);
            syncTaskTimerInterval();
        });
    }

    // Tab and column sort links use fetch (same reliability as task actions).
    document.body.addEventListener('click', function (e) {
        const listLink = e.target.closest('a[hx-get][hx-target="#tasks-list"]');
        if (listLink && !listLink.hasAttribute('hx-confirm')) {
            e.preventDefault();
            e.stopPropagation();
            e.stopImmediatePropagation();
            loadTasksListNav(listLink);
            return;
        }
    }, true);

    // Instant task transitions (complete / in progress / pending) use fetch
    // so they work even when HTMX CDN is blocked or not yet loaded.
    document.body.addEventListener('click', function (e) {
        const opener = e.target.closest('[data-load-task-form]');
        if (opener) {
            e.preventDefault();
            e.stopPropagation();
            openTaskForm(opener.dataset.loadTaskForm);
            return;
        }

        const btn = e.target.closest('[data-task-action]');
        if (!btn || btn.hasAttribute('hx-confirm') || btn.disabled) return;
        if (btn.classList.contains('htmx-request')) return;

        const id = btn.dataset.taskId;
        const action = btn.dataset.taskAction;
        if (!id || !action) return;

        e.preventDefault();
        e.stopPropagation();
        runTaskAction(btn);
    });

    document.body.addEventListener('submit', function (e) {
        const form = e.target.closest('.task-form');
        if (!form) return;
        e.preventDefault();
        submitTaskForm(form);
    });

    document.addEventListener('click', function (e) {
        const trigger = e.target.closest('[data-action]');
        if (!trigger) return;

        switch (trigger.dataset.action) {
            case 'toggle-theme':
                toggleTheme();
                break;
            case 'toggle-sidebar':
                isSidebarOpen() ? closeSidebar() : openSidebar();
                break;
            case 'close-modal':
                e.preventDefault();
                closeModalSlot();
                break;
            case 'confirm-ok':
                e.preventDefault();
                e.stopPropagation();
                closeConfirm(true);
                break;
            case 'confirm-cancel':
                e.preventDefault();
                e.stopPropagation();
                closeConfirm(false);
                break;
        }
    });

    document.addEventListener('click', function (e) {
        if (!isSidebarOpen()) return;
        if (e.target.closest('.sidebar-nav a')) {
            setTimeout(closeSidebar, 0);
        }
    });

    // ------------------------------------------------------------------
    // 10. Keyboard shortcuts
    // ------------------------------------------------------------------

    function isTypingTarget(el) {
        if (!el) return false;
        const tag = el.tagName;
        return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || el.isContentEditable;
    }

    function focusTaskSearch() {
        const input = document.querySelector('.filter--search input[type="search"]');
        if (input) input.focus();
    }

    function openNewTaskModal() {
        const btn = document.querySelector('[data-shortcut="new-task"]');
        if (btn && btn.dataset.loadTaskForm) {
            openTaskForm(btn.dataset.loadTaskForm);
            return;
        }
        if (btn) btn.click();
    }

    function toggleShortcutsHint() {
        const hint = document.getElementById('shortcuts-hint');
        if (!hint) return;
        const show = hint.hasAttribute('hidden');
        hint.toggleAttribute('hidden', !show);
        hint.setAttribute('aria-hidden', show ? 'false' : 'true');
        if (show) {
            setTimeout(function () {
                hint.setAttribute('hidden', '');
                hint.setAttribute('aria-hidden', 'true');
            }, 4000);
        }
    }

    document.addEventListener('keydown', function (e) {
        if (e.key === 'Escape') {
            if (closeConfirm(false)) { e.preventDefault(); return; }
            const slot = document.getElementById('task-modal');
            if (slot && slot.innerHTML.trim()) {
                closeModalSlot();
                e.preventDefault();
                return;
            }
            if (isSidebarOpen()) {
                closeSidebar();
                e.preventDefault();
            }
            return;
        }

        if (isTypingTarget(document.activeElement)) return;

        if (e.key === '?' && !e.metaKey && !e.ctrlKey && !e.altKey) {
            e.preventDefault();
            toggleShortcutsHint();
            return;
        }

        if (e.key === '/' && !e.metaKey && !e.ctrlKey) {
            if (window.location.pathname.startsWith('/tasks')) {
                e.preventDefault();
                focusTaskSearch();
            }
            return;
        }

        if ((e.key === 'n' || e.key === 'N') && !e.metaKey && !e.ctrlKey && !e.altKey) {
            if (window.location.pathname === '/tasks') {
                e.preventDefault();
                openNewTaskModal();
            }
        }
    });

    refreshActiveNav();
    syncTaskTimerInterval();
})();
