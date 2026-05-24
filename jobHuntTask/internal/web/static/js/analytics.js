/*
 * analytics.js — Chart.js initialisation for the analytics page.
 * Re-inits charts after HTMX partial swaps.
 */
(function () {
    'use strict';

    var instances = {};

    function themeColors() {
        var dark = document.documentElement.getAttribute('data-theme') !== 'light';
        return {
            text: dark ? '#8a929c' : '#6b7280',
            grid: dark ? 'rgba(35,41,50,0.6)' : 'rgba(226,230,235,0.8)',
        };
    }

    function destroyChart(id) {
        if (instances[id]) {
            instances[id].destroy();
            delete instances[id];
        }
    }

    function initCharts(root) {
        if (typeof Chart === 'undefined') return;
        root = root || document;
        root.querySelectorAll('.analytics-chart-config').forEach(function (el) {
            var id = el.getAttribute('data-chart-id');
            var type = el.getAttribute('data-chart-type') || 'line';
            var canvas = document.getElementById(id);
            if (!canvas) return;
            destroyChart(id);
            var data;
            try {
                data = JSON.parse(el.textContent);
            } catch (_) {
                return;
            }
            var colors = themeColors();
            instances[id] = new Chart(canvas, {
                type: type,
                data: data,
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: {
                        legend: { display: type !== 'bar' || (data.datasets && data.datasets.length > 1), labels: { color: colors.text } },
                    },
                    scales: type === 'bar' || type === 'line' ? {
                        x: { ticks: { color: colors.text, maxRotation: 45 }, grid: { color: colors.grid } },
                        y: { beginAtZero: true, ticks: { color: colors.text }, grid: { color: colors.grid } },
                    } : {},
                },
            });
        });
    }

    document.addEventListener('DOMContentLoaded', function () {
        initCharts(document.getElementById('analytics-panels') || document);
    });

    document.body.addEventListener('htmx:afterSwap', function (evt) {
        var target = evt.detail && evt.detail.target;
        if (!target) return;
        if (target.id === 'analytics-panels' || target.closest && target.closest('#analytics-panels')) {
            initCharts(document.getElementById('analytics-panels'));
        } else if (target.classList && target.classList.contains('analytics-chart-card')) {
            initCharts(target);
        }
    });
})();
