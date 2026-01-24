import { dom } from '../dom.js';
import { authToken, currentUser, userOwnedDomains, pendingOwnedDomain, setPendingOwnedDomain } from '../state.js';
import { apiFetch } from '../api.js';
import { openLoginModal } from '../auth/login-modal.js';
import { renderFavoritesView } from '../views/favorites.js';
import { getActiveTab } from '../tabs/index.js';
import { renderResultsForTab } from '../search/results.js';

export function openOwnedModal(domain) {
    if (!authToken) {
        openLoginModal();
        return;
    }
    setPendingOwnedDomain(domain);
    dom.ownedDomainName.textContent = domain;
    dom.ownedError.hidden = true;
    dom.ownedModal.hidden = false;
}

export async function removeOwnedDomain(domain) {
    try {
        const response = await apiFetch(`/api/owned/${encodeURIComponent(domain)}`, {
            method: 'DELETE'
        });
        if (response.ok) {
            userOwnedDomains.delete(domain.toLowerCase());
            return true;
        }
        return false;
    } catch (err) {
        console.error('Failed to remove owned domain:', err);
        return false;
    }
}

export function initOwnedModal() {
    dom.ownedClose.addEventListener('click', () => {
        dom.ownedModal.hidden = true;
        setPendingOwnedDomain(null);
    });

    dom.ownedModal.addEventListener('click', (e) => {
        if (e.target === dom.ownedModal) {
            dom.ownedModal.hidden = true;
            setPendingOwnedDomain(null);
        }
    });

    // Owned option click handlers
    dom.ownedModal.querySelectorAll('.owned-option').forEach(btn => {
        btn.addEventListener('click', async () => {
            if (!pendingOwnedDomain) return;

            const acquisitionType = btn.dataset.type;
            btn.disabled = true;

            try {
                const response = await apiFetch('/api/owned', {
                    method: 'POST',
                    body: JSON.stringify({
                        domain: pendingOwnedDomain,
                        acquisitionType: acquisitionType
                    })
                });

                if (response.ok) {
                    userOwnedDomains.set(pendingOwnedDomain.toLowerCase(), {
                        acquisitionType: acquisitionType,
                        createdAt: Date.now() / 1000
                    });
                    dom.ownedModal.hidden = true;
                    const domain = pendingOwnedDomain;
                    setPendingOwnedDomain(null);
                    // Re-render to show owned badge
                    if (!dom.favoritesView.hidden) {
                        renderFavoritesView();
                    } else {
                        const activeTab = getActiveTab();
                        if (activeTab && Object.keys(activeTab.categories).length > 0) {
                            renderResultsForTab(activeTab);
                        }
                    }
                } else {
                    const data = await response.json();
                    dom.ownedError.textContent = data.error || 'Failed to mark domain as owned';
                    dom.ownedError.hidden = false;
                }
            } catch (err) {
                dom.ownedError.textContent = 'Failed to mark domain as owned. Please try again.';
                dom.ownedError.hidden = false;
            } finally {
                btn.disabled = false;
            }
        });
    });
}
