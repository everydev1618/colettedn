import {
    authToken, currentUser, userFavorites, userOwnedDomains, userMonitoring,
    setAuthToken, setCurrentUser, activeTabId, tabs
} from '../state.js';
import { apiFetch, getAuthHeaders } from '../api.js';
import { escapeHtml } from '../utils.js';
import { dom } from '../dom.js';
import { ADMIN_EMAIL } from '../config.js';
import { applyUserTheme } from '../theme.js';
import { showWelcomeContent } from '../views/welcome.js';
import { loadUserPreferences } from '../views/registration.js';
import { renderResultsForTab } from '../search/results.js';

export function checkAuthFromHash() {
    const hash = window.location.hash;
    if (hash.startsWith('#token=')) {
        const token = hash.substring(7);
        setAuthToken(token);
        // Clear the hash
        history.replaceState(null, '', window.location.pathname);
        fetchCurrentUser(true); // true = just signed in
    }
}

export async function fetchCurrentUser(justSignedIn = false) {
    try {
        const response = await apiFetch('/api/user/me');
        if (response.ok) {
            const data = await response.json();
            setCurrentUser(data.user);
            updateAuthUI();
            await fetchFavorites();
            await fetchOwnedDomains();
            await fetchMonitoredDomains();
        } else {
            // Invalid token
            logout();
        }
    } catch (err) {
        console.error('Failed to fetch user:', err);
        logout();
    }
}

export async function fetchFavorites() {
    if (!authToken) return;
    try {
        const response = await apiFetch('/api/favorites');
        if (response.ok) {
            const data = await response.json();
            userFavorites.clear();
            data.favorites.forEach(f => userFavorites.add(f.domain.toLowerCase()));
        }
    } catch (err) {
        console.error('Failed to fetch favorites:', err);
    }
}

export async function fetchOwnedDomains() {
    if (!authToken) return;
    try {
        const response = await apiFetch('/api/owned');
        if (response.ok) {
            const data = await response.json();
            userOwnedDomains.clear();
            for (const d of data.owned) {
                userOwnedDomains.set(d.domain.toLowerCase(), {
                    acquisitionType: d.acquisitionType,
                    createdAt: d.createdAt
                });
            }
        }
    } catch (err) {
        console.error('Failed to fetch owned domains:', err);
    }
}

export async function fetchMonitoredDomains() {
    if (!authToken) return;
    const isPro = currentUser && currentUser.subscriptionTier === 'pro';
    if (!isPro) return;
    try {
        const response = await apiFetch('/api/monitoring');
        if (response.ok) {
            const data = await response.json();
            userMonitoring.clear();
            for (const d of data.monitoring) {
                userMonitoring.set(d.domain.toLowerCase(), {
                    expirationDate: d.expirationDate,
                    daysUntilExpiry: d.daysUntilExpiry,
                    registrar: d.registrar,
                    createdAt: d.createdAt,
                    lastCheckedAt: d.lastCheckedAt
                });
            }
        }
    } catch (err) {
        console.error('Failed to fetch monitored domains:', err);
    }
}

export function updateAuthUI() {
    const isPro = currentUser && currentUser.subscriptionTier === 'pro';

    if (currentUser) {
        // Header (app layout)
        dom.signInBtn.hidden = true;
        dom.userDropdown.hidden = false;
        dom.userEmailEl.innerHTML = isPro
            ? `<span class="user-email-text">${escapeHtml(currentUser.email)}</span><span class="pro-badge">Pro</span>`
            : `<span class="user-email-text">${escapeHtml(currentUser.email)}</span>`;

        // Update plan info in dropdown
        if (isPro) {
            dom.planName.textContent = 'Pro';
            dom.planName.classList.add('plan-pro');
            dom.planDetail.textContent = 'Unlimited searches';
        } else {
            dom.planName.textContent = 'Free';
            dom.planName.classList.remove('plan-pro');
            dom.planDetail.textContent = '3 searches/day';
        }

        // Show upgrade or manage button based on tier
        dom.upgradeMenuBtn.hidden = isPro;
        dom.manageBtn.hidden = !isPro;

        // Show monitoring button only for Pro users
        dom.monitoringBtn.hidden = !isPro;

        // Show admin button only for admin email
        const isAdmin = currentUser.email === ADMIN_EMAIL;
        dom.adminBtn.hidden = !isAdmin;

        // Load user preferences (for registrar preference, theme, etc.)
        loadUserPreferences();

        // Re-render active tab to update PRO-only features (e.g., monitor buttons)
        const activeTab = tabs.find(t => t.id === activeTabId);
        if (activeTab && activeTab.categories && Object.keys(activeTab.categories).length > 0) {
            renderResultsForTab(activeTab);
        }
    } else {
        // Header (app layout)
        dom.signInBtn.hidden = false;
        dom.userDropdown.hidden = true;
    }
}

export function logout() {
    setAuthToken(null);
    setCurrentUser(null);
    userFavorites.clear();
    userOwnedDomains.clear();
    userMonitoring.clear();
    updateAuthUI();
    // POST to logout endpoint (fire and forget)
    fetch('/api/auth/logout', { method: 'POST', headers: getAuthHeaders() }).catch(() => {});
    // Show welcome content
    showWelcomeContent();
}

export function initAuth() {
    // Check for auth token in URL hash (after magic link redirect)
    checkAuthFromHash();

    // Initialize auth state
    if (authToken) {
        fetchCurrentUser();
    }
}
