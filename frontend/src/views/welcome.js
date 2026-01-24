import { dom } from '../dom.js';
import { animateCounter } from '../utils.js';

let currentTourStep = 1;
const tourTargets = [
    { el: () => document.getElementById('description'), padding: 8 },
    { el: () => document.querySelector('.search-options'), padding: 8 },
    { el: () => document.getElementById('submit-btn'), padding: 8 }
];

export function showWelcomeContent() {
    dom.resultsEl.hidden = true;
    dom.favoritesView.hidden = true;
    dom.historyView.hidden = true;
    dom.monitoringView.hidden = true;
    dom.welcomeContent.hidden = false;
    // Hide registration view if showing
    if (dom.registrationView) dom.registrationView.hidden = true;
}

export function startTour() {
    currentTourStep = 1;
    dom.tourOverlay.hidden = false;
    showTourStep(currentTourStep);
}

function showTourStep(step) {
    // Hide all steps
    dom.tourTooltip.querySelectorAll('.tour-step').forEach(s => s.hidden = true);
    // Show current step
    const stepEl = dom.tourTooltip.querySelector(`[data-step="${step}"]`);
    if (stepEl) stepEl.hidden = false;

    // Position spotlight and tooltip
    const target = tourTargets[step - 1];
    if (target && target.el()) {
        const el = target.el();
        const rect = el.getBoundingClientRect();
        const padding = target.padding || 4;

        // Position spotlight
        dom.tourSpotlight.style.top = (rect.top - padding) + 'px';
        dom.tourSpotlight.style.left = (rect.left - padding) + 'px';
        dom.tourSpotlight.style.width = (rect.width + padding * 2) + 'px';
        dom.tourSpotlight.style.height = (rect.height + padding * 2) + 'px';

        // Position tooltip below the spotlight
        dom.tourTooltip.style.top = (rect.bottom + padding + 12) + 'px';
        dom.tourTooltip.style.left = Math.max(16, Math.min(rect.left, window.innerWidth - 320)) + 'px';
    }
}

function nextTourStep() {
    currentTourStep++;
    if (currentTourStep > tourTargets.length) {
        endTour();
    } else {
        showTourStep(currentTourStep);
    }
}

function endTour() {
    dom.tourOverlay.hidden = true;
    // Focus the search input
    if (dom.descriptionInput) {
        dom.descriptionInput.focus();
    }
}

export async function fetchStats() {
    try {
        const response = await fetch('/api/stats');
        if (response.ok) {
            const data = await response.json();
            if (dom.statDomainsEl && data.domainsFound) {
                // Animate the counter
                animateCounter(dom.statDomainsEl, data.domainsFound);
            }
        }
    } catch (err) {
        console.error('Failed to fetch stats:', err);
    }
}

export function initWelcome() {
    // Get Started button - start onboarding tour
    if (dom.getStartedBtn) {
        dom.getStartedBtn.addEventListener('click', startTour);
    }

    // Tour button handlers
    dom.tourTooltip.querySelectorAll('.tour-next').forEach(btn => {
        btn.addEventListener('click', nextTourStep);
    });

    dom.tourTooltip.querySelectorAll('.tour-done').forEach(btn => {
        btn.addEventListener('click', endTour);
    });

    // Close tour on backdrop click
    dom.tourOverlay.querySelector('.tour-backdrop').addEventListener('click', endTour);

    // Logo click - show welcome content
    dom.logoHome.addEventListener('click', (e) => {
        e.preventDefault();
        showWelcomeContent();
    });

    // Initialize stats on page load
    fetchStats();
}
