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

    document.body.addEventListener('htmx:confirm', function (e) {
        e.preventDefault();
        const elt = e.detail.elt;
        const isDanger = !!(elt && (
            elt.getAttribute('hx-delete') ||
            elt.dataset.confirmDanger === 'true' ||
            elt.classList.contains('btn-danger-ghost') ||
            elt.classList.contains('icon-button--danger')
        ));
        openConfirm({
            message: e.detail.question || 'Are you sure?',
            danger: isDanger,
            confirmLabel: isDanger ? 'Delete' : 'Confirm',
            onConfirm: function () { e.detail.issueRequest(true); },
            onCancel: function () { e.detail.issueRequest(false); },
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
        if (kind === 'delete') {
            target.classList.add('htmx-optimistic--removing');
            const id = target.id && target.id.split('-').pop();
            if (id) {
                const peer = document.getElementById(
                    target.id.startsWith('task-row-') ? 'task-card-' + id : 'task-row-' + id
                );
                if (peer) peer.classList.add('htmx-optimistic', 'htmx-optimistic--delete', 'htmx-optimistic--removing');
            }
        }
        if (kind === 'complete') target.classList.add('task-row--status-completed', 'task-card--status-completed');
        if (kind === 'in_progress') target.classList.add('task-row--status-in_progress', 'task-card--status-in_progress');
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
        pendingRequests++;
        setGlobalLoading(true);
        const elt = e.detail.elt;
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
            showToast({ tone: 'danger', message: friendlyError(e) });
            maybeInjectRetry(e);
        } else {
            optimisticSnapshots.delete(elt);
        }
    });

    document.body.addEventListener('htmx:beforeSwap', function (e) {
        if (e.detail.target) e.detail.target.classList.add('htmx-swapping');
    });

    document.body.addEventListener('htmx:afterSettle', function (e) {
        if (e.detail.target) e.detail.target.classList.remove('htmx-swapping');
        if (e.detail.target && e.detail.target.id === 'main-content') {
            refreshActiveNav();
            updateNavbarTitle();
            const main = document.getElementById('main-content');
            if (main) main.focus({ preventScroll: true });
        }
        if (e.detail.target && e.detail.target.id === 'task-modal') {
            bindModalSlot(e.detail.target);
        }
    });

    // ------------------------------------------------------------------
    // 9. Delegated actions
    // ------------------------------------------------------------------

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
                closeModalSlot();
                break;
            case 'confirm-ok':
                closeConfirm(true);
                break;
            case 'confirm-cancel':
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
})();
