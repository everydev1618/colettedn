import { dom } from '../dom.js';
import { activeTabId, authToken, currentUser, tabs, userFavorites, userOwnedDomains, userMonitoring, comSiteChecks } from '../state.js';
import { apiFetch } from '../api.js';
import { escapeHtml, formatExpiryBadge, formatRelativeTime, extractBaseName } from '../utils.js';
import { renderDomainCard } from './domain-card.js';
import { checkComSite, getComStatusHtml } from './com-check.js';
import { saveTabsToStorage } from '../tabs/persistence.js';
import { renderTabBar } from '../tabs/render.js';
import { getActiveTab } from '../tabs/index.js';
import { showRegistrationView } from '../views/registration.js';
import { openOwnedModal, removeOwnedDomain } from '../modals/owned.js';
import { openLoginModal } from '../auth/login-modal.js';
import { openUpgradeModal } from '../modals/upgrade.js';
import { addToMonitoring, removeFromMonitoring } from '../views/monitoring.js';
import { toggleFavorite } from '../views/favorites.js';

export function showErrorForTab(tab) {
    if (activeTabId !== tab.id) return;
    dom.resultsEl.innerHTML = `<p class="error-message">${escapeHtml(tab.error)}</p>`;
    dom.welcomeContent.hidden = true;
    dom.resultsEl.hidden = false;
}

export function renderResultsForTab(tab) {
    if (activeTabId !== tab.id) return; // Don't render if not active

    const categories = tab.categories;
    const rounds = tab.rounds || 1;
    const categoryOrder = ['Professional', 'Playful', 'Creative', 'Minimal'];
    const totalDomains = Object.values(categories).flat().length;
    const isPro = currentUser && currentUser.subscriptionTier === 'pro';

    // Small inline badge for multi-round searches
    const roundsBadge = rounds > 1
        ? `<span class="rounds-badge">${rounds} rounds · ${totalDomains} found</span>`
        : '';

    // Search phrase header with copy-to-search functionality
    const searchPhraseHtml = tab.description ? `
        <div class="search-phrase-header">
            <span class="search-phrase-label">Search:</span>
            <span class="search-phrase-text">"${escapeHtml(tab.description)}"</span>
            <button class="search-phrase-copy" title="Copy to search field">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"></path>
                    <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"></path>
                </svg>
                Edit search
            </button>
        </div>
    ` : '';

    const sectionsHtml = categoryOrder
        .map((cat, idx) => {
            const domains = categories[cat] || [];
            // Put the rounds badge after the first category title
            const badge = idx === 0 ? roundsBadge : '';
            const gridContent = domains.length > 0
                ? domains.map((d, i) => renderDomainCard(d, i, categories)).join('')
                : '<div class="empty-category">No available domains found</div>';
            return `
                <section class="category${domains.length === 0 ? ' category-empty' : ''}">
                    <div class="category-header">
                        <h2 class="category-title">${cat}</h2>
                        <span class="category-count">${domains.length}</span>
                        ${badge}
                        <div class="category-line"></div>
                    </div>
                    <div class="domain-grid">
                        ${gridContent}
                    </div>
                </section>
            `;
        }).join('');

    // Render searched domain section (for domain mode searches)
    const searchedDomainHtml = renderSearchedDomainSection(tab.searchedDomain, tab.description);

    // Render unavailable domains section
    const unavailableHtml = renderUnavailableSection(tab.unavailable);

    dom.resultsEl.innerHTML = searchPhraseHtml + searchedDomainHtml + sectionsHtml + unavailableHtml;
    dom.resultsEl.hidden = false;
    dom.welcomeContent.hidden = true;

    // Add click handler for "Edit search" button
    const copyBtn = dom.resultsEl.querySelector('.search-phrase-copy');
    if (copyBtn) {
        copyBtn.addEventListener('click', () => {
            if (dom.descriptionInput && tab.description) {
                dom.descriptionInput.value = tab.description;
                dom.descriptionInput.focus();
                dom.descriptionInput.select();
                // Scroll to top if needed
                window.scrollTo({ top: 0, behavior: 'smooth' });
            }
        });
    }

    // Add refresh button handlers
    dom.resultsEl.querySelectorAll('.cache-refresh').forEach(btn => {
        btn.addEventListener('click', (e) => {
            e.preventDefault();
            refreshDomain(btn.dataset.domain);
        });
    });

    // Add favorite button handlers
    dom.resultsEl.querySelectorAll('.favorite-btn').forEach(btn => {
        btn.addEventListener('click', async (e) => {
            e.preventDefault();
            await toggleFavorite(btn.dataset.domain);
            // Update button state
            const domain = btn.dataset.domain.toLowerCase();
            btn.classList.toggle('favorited', userFavorites.has(domain));
            btn.textContent = userFavorites.has(domain) ? '♥' : '♡';
        });
    });

    // Register button click handlers - open registration view
    dom.resultsEl.querySelectorAll('.domain-register-btn').forEach(btn => {
        btn.addEventListener('click', () => {
            const domain = btn.dataset.domain;
            if (domain) {
                showRegistrationView(domain);
            }
        });
    });

    // Searched domain register button handlers
    dom.resultsEl.querySelectorAll('.searched-register-btn').forEach(btn => {
        btn.addEventListener('click', () => {
            const domain = btn.dataset.domain;
            if (domain) {
                showRegistrationView(domain);
            }
        });
    });

    // Add monitor button handlers (for searched domain section)
    dom.resultsEl.querySelectorAll('.searched-domain-section .monitor-btn').forEach(btn => {
        btn.addEventListener('click', async (e) => {
            e.preventDefault();

            // If PRO required button, open upgrade modal
            if (btn.classList.contains('pro-required')) {
                if (!currentUser) {
                    openLoginModal();
                } else {
                    openUpgradeModal();
                }
                return;
            }

            const domain = btn.dataset.domain;
            const isMonitored = userMonitoring.has(domain.toLowerCase());

            if (isMonitored) {
                // Remove from monitoring
                const success = await removeFromMonitoring(domain);
                if (success) {
                    btn.classList.remove('monitored');
                    btn.textContent = '◯';
                    btn.title = 'Monitor this domain';
                }
            } else {
                // Add to monitoring
                const expirationDate = btn.dataset.expiration || null;
                const daysUntilExpiry = btn.dataset.days ? parseInt(btn.dataset.days) : null;
                const registrar = btn.dataset.registrar || '';

                const success = await addToMonitoring(domain, expirationDate, daysUntilExpiry, registrar);
                if (success) {
                    btn.classList.add('monitored');
                    btn.textContent = '◉';
                    btn.title = 'Remove from monitoring';
                }
            }
        });
    });

    // Add own button handlers
    dom.resultsEl.querySelectorAll('.own-btn').forEach(btn => {
        btn.addEventListener('click', (e) => {
            e.preventDefault();
            openOwnedModal(btn.dataset.domain);
        });
    });

    // Add unown button handlers
    dom.resultsEl.querySelectorAll('.unown-btn').forEach(btn => {
        btn.addEventListener('click', async (e) => {
            e.preventDefault();
            await removeOwnedDomain(btn.dataset.domain);
            renderResultsForTab(tab);
        });
    });

    // Add monitor button handlers (for unavailable section)
    dom.resultsEl.querySelectorAll('.unavailable-section .monitor-btn').forEach(btn => {
        btn.addEventListener('click', async (e) => {
            e.preventDefault();

            // If PRO required button, open upgrade modal
            if (btn.classList.contains('pro-required')) {
                if (!currentUser) {
                    openLoginModal();
                } else {
                    openUpgradeModal();
                }
                return;
            }

            const domain = btn.dataset.domain;
            const isMonitored = userMonitoring.has(domain.toLowerCase());

            if (isMonitored) {
                // Remove from monitoring
                const success = await removeFromMonitoring(domain);
                if (success) {
                    btn.classList.remove('monitored');
                    btn.textContent = '◯';
                    btn.title = 'Monitor this domain';
                }
            } else {
                // Add to monitoring
                const expirationDate = btn.dataset.expiration || null;
                const daysUntilExpiry = btn.dataset.days ? parseInt(btn.dataset.days) : null;
                const registrar = btn.dataset.registrar || '';

                const success = await addToMonitoring(domain, expirationDate, daysUntilExpiry, registrar);
                if (success) {
                    btn.classList.add('monitored');
                    btn.textContent = '◉';
                    btn.title = 'Remove from monitoring';
                }
            }
        });
    });

    // Add check .com button handlers
    dom.resultsEl.querySelectorAll('.check-com-btn').forEach(btn => {
        btn.addEventListener('click', async (e) => {
            e.preventDefault();
            const domain = btn.dataset.domain;
            btn.disabled = true;
            btn.textContent = '...';

            const result = await checkComSite(domain);
            if (result) {
                // Update in place - replace button with status
                const statusHtml = getComStatusHtml(result.status, result.domain, result.expirationDate, result.daysUntilExpiry);
                btn.outerHTML = statusHtml;
            } else {
                btn.disabled = false;
                btn.textContent = 'check .com';
            }
        });
    });
}

function renderSearchedDomainSection(searchedDomain, description) {
    if (!searchedDomain || searchedDomain.length === 0) {
        return '';
    }

    // Extract base name from the first domain for the header
    const baseName = searchedDomain[0].name.split('.')[0];
    const isPro = currentUser && currentUser.subscriptionTier === 'pro';

    const cards = searchedDomain.map(d => {
        const isAvailable = d.available;

        if (isAvailable) {
            // Available domain - show register button
            return `
                <div class="searched-card searched-card-available">
                    <div class="searched-info">
                        <div class="searched-name">${escapeHtml(d.name)}</div>
                        <div class="searched-meta">
                            <span class="searched-available-badge">Available</span>
                        </div>
                    </div>
                    <button class="searched-register-btn" data-domain="${escapeHtml(d.name)}">
                        Register
                    </button>
                </div>
            `;
        } else {
            // Taken domain - show expiry info and monitor button (like unavailable section)
            const isMonitored = userMonitoring.has(d.name.toLowerCase());
            const expiryBadge = formatExpiryBadge(d.daysUntilExpiry, d.expirationDate);
            const registrarText = d.registrar ? `<span class="registrar-info">${escapeHtml(d.registrar)}</span>` : '';

            let monitorBtn = '';
            if (isPro) {
                monitorBtn = `
                    <button class="monitor-btn ${isMonitored ? 'monitored' : ''}"
                            data-domain="${escapeHtml(d.name)}"
                            data-expiration="${d.expirationDate || ''}"
                            data-days="${d.daysUntilExpiry !== null ? d.daysUntilExpiry : ''}"
                            data-registrar="${escapeHtml(d.registrar || '')}"
                            title="${isMonitored ? 'Remove from monitoring' : 'Monitor this domain'}">
                        ${isMonitored ? '◉' : '◯'}
                    </button>
                `;
            } else {
                monitorBtn = `
                    <button class="monitor-btn pro-required" title="PRO required">
                        ◯
                        <span class="pro-label">PRO</span>
                    </button>
                `;
            }

            return `
                <div class="searched-card">
                    <div class="searched-info">
                        <div class="searched-name searched-name-taken">${escapeHtml(d.name)}</div>
                        <div class="searched-meta">
                            ${expiryBadge}
                            ${registrarText}
                        </div>
                    </div>
                    ${monitorBtn}
                </div>
            `;
        }
    }).join('');

    return `
        <div class="searched-domain-section">
            <div class="searched-domain-header">
                <h3 class="searched-domain-title">Your Search: ${escapeHtml(baseName)}</h3>
                <span class="searched-domain-subtitle">TLD variations</span>
            </div>
            <div class="searched-domain-grid">
                ${cards}
            </div>
        </div>
    `;
}

function renderUnavailableSection(unavailable) {
    if (!unavailable || unavailable.length === 0) {
        return '';
    }

    const isPro = currentUser && currentUser.subscriptionTier === 'pro';

    const cards = unavailable.map(d => {
        const isMonitored = userMonitoring.has(d.name.toLowerCase());
        const expiryBadge = formatExpiryBadge(d.daysUntilExpiry, d.expirationDate);
        const registrarText = d.registrar ? `<span class="registrar-info">${escapeHtml(d.registrar)}</span>` : '';

        let monitorBtn = '';
        if (isPro) {
            monitorBtn = `
                <button class="monitor-btn ${isMonitored ? 'monitored' : ''}"
                        data-domain="${escapeHtml(d.name)}"
                        data-expiration="${d.expirationDate || ''}"
                        data-days="${d.daysUntilExpiry !== null ? d.daysUntilExpiry : ''}"
                        data-registrar="${escapeHtml(d.registrar || '')}"
                        title="${isMonitored ? 'Remove from monitoring' : 'Monitor this domain'}">
                    ${isMonitored ? '◉' : '◯'}
                </button>
            `;
        } else {
            monitorBtn = `
                <button class="monitor-btn pro-required" title="PRO required">
                    ◯
                    <span class="pro-label">PRO</span>
                </button>
            `;
        }

        return `
            <div class="unavailable-card">
                <div class="unavailable-info">
                    <div class="unavailable-name">${escapeHtml(d.name)}</div>
                    <div class="unavailable-meta">
                        ${expiryBadge}
                        ${registrarText}
                    </div>
                </div>
                ${monitorBtn}
            </div>
        `;
    }).join('');

    return `
        <div class="unavailable-section">
            <div class="unavailable-header">
                <h3 class="unavailable-title">Unavailable</h3>
                <span class="unavailable-subtitle">Monitor these domains for expiration</span>
            </div>
            <div class="unavailable-grid">
                ${cards}
            </div>
        </div>
    `;
}

// Save search to history (PRO only)
export async function saveSearchHistory(description, tldStyle, categoriesData) {
    if (!authToken) return;
    // PRO only feature
    if (!currentUser || currentUser.subscriptionTier !== 'pro') return;

    try {
        await apiFetch('/api/history', {
            method: 'POST',
            body: JSON.stringify({
                description,
                tldStyle,
                categories: categoriesData
            })
        });
    } catch (err) {
        console.error('Failed to save history:', err);
    }
}

// Generate a title for a tab using AI
export async function generateTabTitle(tabId, searchPhrase) {
    try {
        const response = await fetch('/api/generate-tab-title', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ searchPhrase })
        });

        if (response.ok) {
            const data = await response.json();
            if (data.title) {
                // Find the tab and update its title
                const tab = tabs.find(t => t.id === tabId);
                if (tab) {
                    tab.title = data.title;
                    renderTabBar();
                    // Re-render results if this is the active tab (to show the title)
                    if (activeTabId === tabId) {
                        renderResultsForTab(tab);
                    }
                    saveTabsToStorage();
                }
            }
        }
    } catch (err) {
        console.error('Failed to generate tab title:', err);
        // Silent failure - tab will just use description as title
    }
}

async function refreshDomain(domainName) {
    const tab = getActiveTab();
    if (!tab) return;

    const categories = tab.categories;

    // Find and mark as checking
    Object.keys(categories).forEach(cat => {
        const domain = categories[cat].find(d => d.name.toLowerCase() === domainName.toLowerCase());
        if (domain) {
            domain.checking = true;
            domain.fromCache = false;
        }
    });
    renderResultsForTab(tab);

    try {
        const response = await fetch('/api/check', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                domains: [domainName],
                forceRefresh: [domainName]
            }),
        });

        const data = await response.json();
        if (data.results && data.results.length > 0) {
            const result = data.results[0];
            Object.keys(categories).forEach(cat => {
                const domain = categories[cat].find(d => d.name.toLowerCase() === result.name.toLowerCase());
                if (domain) {
                    domain.checking = false;
                    domain.available = result.available !== null ? result.available : true;
                    domain.isPremium = result.isPremium;
                    domain.price = result.price;
                    domain.fromCache = false;
                    domain.checkedAt = result.checkedAt;

                    // Remove if taken
                    if (!domain.available) {
                        categories[cat] = categories[cat].filter(d => d.name !== domainName);
                    }
                }
            });
        }
        renderResultsForTab(tab);
        saveTabsToStorage();
    } catch (err) {
        console.error('Refresh error:', err);
    }
}
