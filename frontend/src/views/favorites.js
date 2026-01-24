import { dom } from '../dom.js';
import { authToken, currentUser, userFavorites, userOwnedDomains } from '../state.js';
import { apiFetch } from '../api.js';
import { escapeHtml } from '../utils.js';
import { openLoginModal } from '../auth/login-modal.js';
import { openUpgradeModal } from '../modals/upgrade.js';
import { openOwnedModal, removeOwnedDomain } from '../modals/owned.js';
import { getActiveTab, switchToTab } from '../tabs/index.js';
import { showRegistrationView } from './registration.js';

export async function showFavoritesView() {
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
    dom.historyView.hidden = true;
    dom.monitoringView.hidden = true;
    dom.favoritesView.hidden = false;
    // Hide registration view if showing
    if (dom.registrationView) dom.registrationView.hidden = true;
    await renderFavoritesView();
}

export function hideFavoritesView() {
    dom.favoritesView.hidden = true;
    dom.historyView.hidden = true;
    dom.monitoringView.hidden = true;
    const activeTab = getActiveTab();
    if (activeTab && Object.keys(activeTab.categories).length > 0) {
        switchToTab(activeTab.id);
    } else {
        dom.welcomeContent.hidden = false;
    }
}

export async function renderFavoritesView() {
    if (userFavorites.size === 0) {
        dom.favoritesList.innerHTML = `
            <div class="favorites-empty">
                <p>No favorites yet</p>
                <p class="favorites-empty-hint">Heart domains to save them here</p>
            </div>
        `;
        return;
    }

    const favArray = Array.from(userFavorites);
    dom.favoritesList.innerHTML = favArray.map((domain, i) => {
        const ownedInfo = userOwnedDomains.get(domain.toLowerCase());
        const isOwned = !!ownedInfo;
        const ownedBadgeHtml = isOwned
            ? `<span class="owned-badge" title="${ownedInfo.acquisitionType === 'found_via_colette' ? 'Found on Colette' : 'Previously owned'}">✓ Owned</span>`
            : '';
        const actionHtml = isOwned
            ? `<button class="unown-btn" data-domain="${escapeHtml(domain)}" title="Remove ownership">✕</button>`
            : `<button class="domain-register-btn" data-domain="${escapeHtml(domain)}">Register &rarr;</button>`;

        return `
            <div class="domain-card${isOwned ? ' owned' : ''}" style="animation-delay: ${i * 0.03}s">
                <div class="domain-name-row">
                    <span class="domain-name">${escapeHtml(domain)}</span>
                    ${ownedBadgeHtml}
                </div>
                <div class="domain-row">
                    <button class="favorite-btn favorited" data-domain="${escapeHtml(domain)}" title="Remove from favorites">
                        ♥
                    </button>
                    <button class="own-btn${isOwned ? ' hidden' : ''}" data-domain="${escapeHtml(domain)}" title="I own this domain">
                        ✓
                    </button>
                    ${actionHtml}
                </div>
            </div>
        `;
    }).join('');

    // Add remove handlers
    dom.favoritesList.querySelectorAll('.favorite-btn').forEach(btn => {
        btn.addEventListener('click', async () => {
            await toggleFavorite(btn.dataset.domain);
            await renderFavoritesView();
        });
    });

    // Register button click handlers - open registration view
    dom.favoritesList.querySelectorAll('.domain-register-btn').forEach(btn => {
        btn.addEventListener('click', () => {
            const domain = btn.dataset.domain;
            if (domain) {
                showRegistrationView(domain);
            }
        });
    });

    // Add own button handlers
    dom.favoritesList.querySelectorAll('.own-btn').forEach(btn => {
        btn.addEventListener('click', (e) => {
            e.preventDefault();
            openOwnedModal(btn.dataset.domain);
        });
    });

    // Add unown button handlers
    dom.favoritesList.querySelectorAll('.unown-btn').forEach(btn => {
        btn.addEventListener('click', async (e) => {
            e.preventDefault();
            await removeOwnedDomain(btn.dataset.domain);
            await renderFavoritesView();
        });
    });
}

export async function toggleFavorite(domain) {
    if (!authToken) {
        openLoginModal();
        return;
    }
    // PRO only feature
    if (!currentUser || currentUser.subscriptionTier !== 'pro') {
        openUpgradeModal();
        return;
    }

    domain = domain.toLowerCase();
    const isFavorite = userFavorites.has(domain);

    try {
        if (isFavorite) {
            const response = await apiFetch(`/api/favorites/${encodeURIComponent(domain)}`, {
                method: 'DELETE'
            });
            if (response.ok) {
                userFavorites.delete(domain);
            }
        } else {
            const response = await apiFetch('/api/favorites', {
                method: 'POST',
                body: JSON.stringify({ domain })
            });
            if (response.ok) {
                userFavorites.add(domain);
            }
        }
    } catch (err) {
        console.error('Failed to toggle favorite:', err);
    }
}

export function initFavorites() {
    dom.favoritesClose.addEventListener('click', hideFavoritesView);
}
