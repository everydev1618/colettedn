import { dom } from '../dom.js';
import { currentUser, userMonitoring } from '../state.js';
import { apiFetch } from '../api.js';
import { escapeHtml, formatExpiryBadge } from '../utils.js';
import { openLoginModal } from '../auth/login-modal.js';
import { openUpgradeModal } from '../modals/upgrade.js';
import { getActiveTab, switchToTab } from '../tabs/index.js';
import { fetchMonitoredDomains } from '../auth/index.js';

export async function showMonitoringView() {
    // Check if user is logged in
    if (!currentUser) {
        openLoginModal();
        return;
    }
    // PRO only feature
    if (currentUser.subscriptionTier !== 'pro') {
        openUpgradeModal();
        return;
    }
    dom.welcomeContent.hidden = true;
    dom.resultsEl.hidden = true;
    dom.favoritesView.hidden = true;
    dom.historyView.hidden = true;
    dom.monitoringView.hidden = false;
    // Hide registration view if showing
    if (dom.registrationView) dom.registrationView.hidden = true;
    await renderMonitoringView();
}

export function hideMonitoringView() {
    dom.monitoringView.hidden = true;
    dom.favoritesView.hidden = true;
    dom.historyView.hidden = true;
    const activeTab = getActiveTab();
    if (activeTab && Object.keys(activeTab.categories).length > 0) {
        switchToTab(activeTab.id);
    } else {
        dom.welcomeContent.hidden = false;
    }
}

export async function renderMonitoringView() {
    // Refresh monitored domains from server
    await fetchMonitoredDomains();

    // Load notification preference
    const notificationsCheckbox = document.getElementById('monitoring-notifications-checkbox');
    if (notificationsCheckbox) {
        try {
            const response = await apiFetch('/api/user/preferences');
            if (response.ok) {
                const data = await response.json();
                notificationsCheckbox.checked = data.monitoringNotifications !== false;
            }
        } catch (err) {
            console.error('Failed to load notification preference:', err);
        }

        // Add change handler (remove old one first to avoid duplicates)
        notificationsCheckbox.onchange = async () => {
            try {
                await apiFetch('/api/user/monitoring-notifications', {
                    method: 'PUT',
                    body: JSON.stringify({ enabled: notificationsCheckbox.checked })
                });
            } catch (err) {
                console.error('Failed to update notification preference:', err);
                // Revert on error
                notificationsCheckbox.checked = !notificationsCheckbox.checked;
            }
        };
    }

    if (userMonitoring.size === 0) {
        dom.monitoringList.innerHTML = `
            <div class="monitoring-empty">
                <p>No domains being monitored</p>
                <p class="monitoring-empty-hint">Click the monitor button on unavailable domains to track them</p>
            </div>
        `;
        return;
    }

    // Sort by days until expiry (soonest first, null values at end)
    const sortedEntries = Array.from(userMonitoring.entries()).sort((a, b) => {
        const aDays = a[1].daysUntilExpiry;
        const bDays = b[1].daysUntilExpiry;

        if (aDays === null && bDays === null) return 0;
        if (aDays === null) return 1;  // a goes after b
        if (bDays === null) return -1; // a goes before b
        return aDays - bDays; // ascending (soonest first)
    });

    const items = sortedEntries.map(([domain, info], i) => {
        const expiryBadge = formatExpiryBadge(info.daysUntilExpiry, info.expirationDate);
        const registrarText = info.registrar ? `<span class="registrar-info">${escapeHtml(info.registrar)}</span>` : '';

        return `
            <div class="monitoring-item" style="animation-delay: ${i * 0.05}s">
                <div class="monitoring-item-info">
                    <div class="monitoring-item-domain">${escapeHtml(domain)}</div>
                    <div class="monitoring-item-meta">
                        ${expiryBadge}
                        ${registrarText}
                    </div>
                </div>
                <div class="monitoring-item-actions">
                    <button class="monitoring-refresh-btn" data-domain="${escapeHtml(domain)}">Refresh</button>
                    <button class="monitoring-remove-btn" data-domain="${escapeHtml(domain)}" title="Remove">&times;</button>
                </div>
            </div>
        `;
    });

    dom.monitoringList.innerHTML = items.join('');

    // Add refresh handlers
    dom.monitoringList.querySelectorAll('.monitoring-refresh-btn').forEach(btn => {
        btn.addEventListener('click', async () => {
            const domain = btn.dataset.domain;
            btn.disabled = true;
            btn.textContent = '...';
            try {
                const response = await apiFetch(`/api/monitoring/${encodeURIComponent(domain)}/refresh`, {
                    method: 'POST'
                });
                if (response.ok) {
                    const data = await response.json();
                    if (data.monitoring) {
                        userMonitoring.set(domain.toLowerCase(), {
                            expirationDate: data.monitoring.expirationDate,
                            daysUntilExpiry: data.monitoring.daysUntilExpiry,
                            registrar: data.monitoring.registrar,
                            createdAt: data.monitoring.createdAt,
                            lastCheckedAt: data.monitoring.lastCheckedAt
                        });
                    }
                    await renderMonitoringView();
                }
            } catch (err) {
                console.error('Failed to refresh domain:', err);
            } finally {
                btn.disabled = false;
                btn.textContent = 'Refresh';
            }
        });
    });

    // Add remove handlers
    dom.monitoringList.querySelectorAll('.monitoring-remove-btn').forEach(btn => {
        btn.addEventListener('click', async () => {
            const domain = btn.dataset.domain;
            try {
                const response = await apiFetch(`/api/monitoring/${encodeURIComponent(domain)}`, {
                    method: 'DELETE'
                });
                if (response.ok) {
                    userMonitoring.delete(domain.toLowerCase());
                    await renderMonitoringView();
                }
            } catch (err) {
                console.error('Failed to remove from monitoring:', err);
            }
        });
    });
}

export async function addToMonitoring(domain, expirationDate, daysUntilExpiry, registrar) {
    try {
        const response = await apiFetch('/api/monitoring', {
            method: 'POST',
            body: JSON.stringify({ domain, expirationDate, daysUntilExpiry, registrar })
        });
        if (response.ok) {
            const data = await response.json();
            if (data.monitoring) {
                userMonitoring.set(domain.toLowerCase(), {
                    expirationDate: data.monitoring.expirationDate,
                    daysUntilExpiry: data.monitoring.daysUntilExpiry,
                    registrar: data.monitoring.registrar,
                    createdAt: data.monitoring.createdAt,
                    lastCheckedAt: data.monitoring.lastCheckedAt
                });
            }
            return true;
        }
        return false;
    } catch (err) {
        console.error('Failed to add to monitoring:', err);
        return false;
    }
}

export async function removeFromMonitoring(domain) {
    try {
        const response = await apiFetch(`/api/monitoring/${encodeURIComponent(domain)}`, {
            method: 'DELETE'
        });
        if (response.ok) {
            userMonitoring.delete(domain.toLowerCase());
            return true;
        }
        return false;
    } catch (err) {
        console.error('Failed to remove from monitoring:', err);
        return false;
    }
}

export function initMonitoring() {
    dom.monitoringClose.addEventListener('click', hideMonitoringView);
}
