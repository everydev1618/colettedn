import { dom } from '../dom.js';
import { currentUser } from '../state.js';
import { apiFetch } from '../api.js';
import { escapeHtml, formatHistoryDate } from '../utils.js';
import { openLoginModal } from '../auth/login-modal.js';
import { openUpgradeModal } from '../modals/upgrade.js';
import { getActiveTab, switchToTab } from '../tabs/index.js';
import { openDomainDetailModal } from '../modals/domain-detail.js';

export async function showHistoryView() {
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
    dom.monitoringView.hidden = true;
    dom.historyView.hidden = false;
    // Hide registration view if showing
    if (dom.registrationView) dom.registrationView.hidden = true;
    await renderHistoryView();
}

export function hideHistoryView() {
    dom.historyView.hidden = true;
    dom.favoritesView.hidden = true;
    dom.monitoringView.hidden = true;
    const activeTab = getActiveTab();
    if (activeTab && Object.keys(activeTab.categories).length > 0) {
        switchToTab(activeTab.id);
    } else {
        dom.welcomeContent.hidden = false;
    }
}

export async function renderHistoryView() {
    try {
        const response = await apiFetch('/api/history');
        if (!response.ok) {
            dom.historyList.innerHTML = `
                <div class="history-empty">
                    <p>Failed to load history</p>
                </div>
            `;
            return;
        }

        const data = await response.json();
        const histories = data.histories || [];

        if (histories.length === 0) {
            dom.historyList.innerHTML = `
                <div class="history-empty">
                    <p>No searches yet</p>
                    <p class="history-empty-hint">Your searches will appear here</p>
                </div>
            `;
            return;
        }

        dom.historyList.innerHTML = histories.map((h, i) => {
            const date = new Date(h.searchedAt);
            const dateStr = formatHistoryDate(date);
            const tldLabel = h.tldStyle === 'creative' ? '.io .ai' :
                            h.tldStyle === 'global' ? '.co.uk .de' :
                            h.tldStyle === 'custom' ? 'Custom' : '.com .co';

            // Get all domains across all categories
            const allDomains = [];
            const categoryOrder = ['Professional', 'Playful', 'Creative', 'Minimal'];
            for (const cat of categoryOrder) {
                if (h.categories && h.categories[cat]) {
                    allDomains.push(...h.categories[cat].map(d => d.name));
                }
            }

            return `
                <div class="history-item" style="animation-delay: ${i * 0.05}s" data-searched-at="${h.searchedAt}">
                    <div class="history-item-header">
                        <div class="history-item-info">
                            <div class="history-item-description">${escapeHtml(h.description)}</div>
                            <div class="history-item-meta">
                                <span class="history-item-date">${dateStr}</span>
                                <span class="history-item-tld">${tldLabel}</span>
                                <span class="history-item-count">${allDomains.length} domains</span>
                            </div>
                        </div>
                        <div class="history-item-actions">
                            <button class="history-search-btn" data-description="${escapeHtml(h.description)}" data-tld="${h.tldStyle}">
                                Search again
                            </button>
                            <button class="history-delete-btn" data-searched-at="${h.searchedAt}" title="Delete">
                                ✕
                            </button>
                        </div>
                    </div>
                    <div class="history-domains-preview">
                        ${allDomains.map(d => `<span class="history-domain-tag" title="Click for domain details">${escapeHtml(d)}</span>`).join('')}
                    </div>
                </div>
            `;
        }).join('');

        // Add domain tag click handlers for detail modal
        dom.historyList.querySelectorAll('.history-domain-tag').forEach(el => {
            el.addEventListener('click', (e) => {
                e.preventDefault();
                e.stopPropagation();
                const domainName = el.textContent;
                if (domainName) {
                    openDomainDetailModal(domainName);
                }
            });
        });

        // Add search again handlers
        dom.historyList.querySelectorAll('.history-search-btn').forEach(btn => {
            btn.addEventListener('click', () => {
                const description = btn.dataset.description;
                const tldStyle = btn.dataset.tld || 'traditional';

                // Set values in the main form
                dom.descriptionInput.value = description;
                dom.tldStyleInput.value = tldStyle;
                document.querySelectorAll('.tld-toggle').forEach(b => {
                    b.classList.toggle('active', b.dataset.value === tldStyle);
                });

                // Hide history view and trigger search
                dom.historyView.hidden = true;
                dom.form.dispatchEvent(new Event('submit'));
            });
        });

        // Add delete handlers
        dom.historyList.querySelectorAll('.history-delete-btn').forEach(btn => {
            btn.addEventListener('click', async () => {
                const searchedAt = btn.dataset.searchedAt;
                try {
                    const response = await apiFetch(`/api/history/${searchedAt}`, {
                        method: 'DELETE'
                    });
                    if (response.ok) {
                        await renderHistoryView();
                    }
                } catch (err) {
                    console.error('Failed to delete history:', err);
                }
            });
        });
    } catch (err) {
        console.error('Failed to load history:', err);
        dom.historyList.innerHTML = `
            <div class="history-empty">
                <p>Failed to load history</p>
            </div>
        `;
    }
}

export function initHistory() {
    dom.historyClose.addEventListener('click', hideHistoryView);
}
