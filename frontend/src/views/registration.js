import { dom } from '../dom.js';
import { authToken, currentUser, setCurrentRegistrationDomain, setUserPreferredRegistrar, setUserPreferredOtherRegistrar, userPreferredRegistrar, userPreferredOtherRegistrar } from '../state.js';
import { apiFetch } from '../api.js';
import { escapeHtml } from '../utils.js';
import { REGISTRARS, TLD_INFO, OTHER_REGISTRARS } from '../config.js';
import { openLoginModal } from '../auth/login-modal.js';
import { getActiveTab, switchToTab } from '../tabs/index.js';
import { applyUserTheme } from '../theme.js';

export async function loadUserPreferences() {
    if (!authToken) return;
    try {
        const response = await apiFetch('/api/user/preferences');
        if (response.ok) {
            const data = await response.json();
            setUserPreferredRegistrar(data.preferredRegistrar || null);
            setUserPreferredOtherRegistrar(data.preferredOtherRegistrar || null);
            // Apply user's saved theme preference
            if (data.theme) {
                applyUserTheme(data.theme);
            }
        }
    } catch (err) {
        console.error('Failed to load preferences:', err);
    }
}

async function savePreferredOtherRegistrar(registrarId) {
    if (!authToken) return;
    try {
        await apiFetch('/api/user/preferences', {
            method: 'PUT',
            body: JSON.stringify({ preferredOtherRegistrar: registrarId })
        });
        setUserPreferredOtherRegistrar(registrarId);
    } catch (err) {
        console.error('Failed to save preference:', err);
    }
}

export function showRegistrationView(domain) {
    setCurrentRegistrationDomain(domain);

    // Hide other views
    dom.welcomeContent.hidden = true;
    dom.resultsEl.hidden = true;
    dom.favoritesView.hidden = true;
    dom.historyView.hidden = true;
    dom.monitoringView.hidden = true;

    // Show registration view
    dom.registrationView.hidden = false;

    // Set domain name
    dom.registrationDomain.textContent = domain;

    // Compute and display stats
    const stats = getDomainStats(domain);
    dom.registrationStats.innerHTML = `
        <span class="stat-item">${stats.length} characters</span>
        <span class="stat-separator">&middot;</span>
        <span class="stat-item">.${stats.tld} TLD</span>
        <span class="stat-separator">&middot;</span>
        <span class="stat-item">${stats.tldInfo}</span>
        ${stats.memorabilityLabel ? `<span class="stat-separator">&middot;</span><span class="stat-item">${stats.memorabilityLabel}</span>` : ''}
    `;

    // Render registrar cards
    renderRegistrarCards(domain);

    // Hide the old preference checkbox area - we handle this differently now
    if (dom.registrationPreference) {
        dom.registrationPreference.hidden = true;
    }
    if (dom.loginForPreference) {
        dom.loginForPreference.hidden = true;
    }
}

export function hideRegistrationView() {
    dom.registrationView.hidden = true;
    setCurrentRegistrationDomain(null);

    // Show results if we have them, otherwise welcome
    const activeTab = getActiveTab();
    if (activeTab && Object.keys(activeTab.categories).length > 0) {
        switchToTab(activeTab.id);
    } else {
        dom.welcomeContent.hidden = false;
    }
}

function getDomainStats(domain) {
    const parts = domain.split('.');
    const name = parts[0];
    const tld = parts.slice(1).join('.');

    const stats = {
        length: name.length,
        tld: tld,
        tldInfo: TLD_INFO[tld] || 'Domain extension',
        hasHyphens: name.includes('-'),
        hasNumbers: /\d/.test(name)
    };

    // Calculate memorability score
    let score = 100;
    if (name.length > 12) score -= 20;
    if (name.length > 8) score -= 10;
    if (stats.hasHyphens) score -= 15;
    if (stats.hasNumbers) score -= 10;
    // Check for common letter patterns (vowels help pronunciation)
    const vowelRatio = (name.match(/[aeiou]/gi) || []).length / name.length;
    if (vowelRatio < 0.2) score -= 15;

    if (score >= 80) {
        stats.memorabilityLabel = 'Easy to remember';
    } else if (score >= 60) {
        stats.memorabilityLabel = 'Fairly memorable';
    } else {
        stats.memorabilityLabel = null;
    }

    return stats;
}

function getRegistrarUrl(domain, registrar) {
    const directUrl = registrar.getUrl(domain);
    if (registrar.affiliateUrl) {
        return `${registrar.affiliateUrl}?u=${encodeURIComponent(directUrl)}`;
    }
    return directUrl;
}

function renderRegistrarCards(domain) {
    // Always show Namecheap first
    const namecheap = REGISTRARS.namecheap;
    const namecheapUrl = getRegistrarUrl(domain, namecheap);

    let cardsHtml = `
        <div class="registrar-card" style="animation-delay: 0s">
            <div class="registrar-info">
                <div class="registrar-name">${namecheap.name}</div>
                <div class="registrar-tagline">${namecheap.tagline}</div>
            </div>
            <div class="registrar-right">
                <div class="registrar-price">${namecheap.price}</div>
                <a href="${namecheapUrl}" target="_blank" rel="noopener" class="registrar-btn" data-registrar="namecheap" data-domain="${escapeHtml(domain)}">
                    Register &rarr;
                </a>
            </div>
        </div>
    `;

    // If user is logged in and has a preferred "other" registrar, show it as second card
    const savedOther = currentUser && userPreferredOtherRegistrar ? OTHER_REGISTRARS[userPreferredOtherRegistrar] : null;
    if (savedOther) {
        const savedOtherUrl = savedOther.getUrl(domain);
        cardsHtml += `
            <div class="registrar-card registrar-card-saved" style="animation-delay: 0.05s">
                <div class="registrar-info">
                    <div class="registrar-name">${escapeHtml(savedOther.name)}</div>
                    <div class="registrar-tagline">Your preferred registrar</div>
                </div>
                <div class="registrar-right">
                    <a href="${savedOtherUrl}" target="_blank" rel="noopener" class="registrar-btn" data-registrar="${savedOther.id}" data-domain="${escapeHtml(domain)}">
                        Register &rarr;
                    </a>
                </div>
                <span class="preferred-badge">Preferred</span>
            </div>
        `;
    }

    // Build dropdown options from OTHER_REGISTRARS
    const dropdownOptions = Object.values(OTHER_REGISTRARS).map(reg =>
        `<li class="autocomplete-item" data-value="${escapeHtml(reg.id)}" data-name="${escapeHtml(reg.name)}">${escapeHtml(reg.name)}</li>`
    ).join('');

    // "Other" card with dropdown
    const otherCardDelay = savedOther ? 0.1 : 0.05;
    cardsHtml += `
        <div class="registrar-card registrar-card-other" style="animation-delay: ${otherCardDelay}s" data-domain="${escapeHtml(domain)}">
            <div class="other-collapsed">
                <div class="registrar-info">
                    <div class="registrar-name">Other</div>
                    <div class="registrar-tagline">Choose a different registrar</div>
                </div>
                <div class="registrar-right">
                    <button type="button" class="registrar-btn other-expand-btn">
                        Choose &rarr;
                    </button>
                </div>
            </div>
            <div class="other-expanded" hidden>
                <div class="other-input-row">
                    <label class="other-label">Select your registrar</label>
                    <div class="autocomplete-wrapper">
                        <input type="text" class="other-input" placeholder="Type to search..." autocomplete="off">
                        <ul class="autocomplete-dropdown" hidden>
                            ${dropdownOptions}
                        </ul>
                    </div>
                </div>
                <div class="other-selected" hidden>
                    <div class="other-selected-name"></div>
                    <a href="#" target="_blank" rel="noopener" class="registrar-btn other-register-btn" data-domain="${escapeHtml(domain)}">
                        Register &rarr;
                    </a>
                </div>
                <div class="other-save-prompt" hidden>
                    ${currentUser
                        ? `<label class="other-save-label">
                               <input type="checkbox" class="other-save-checkbox">
                               <span>Remember this choice</span>
                           </label>`
                        : `<div class="other-login-prompt">
                               <a href="#" class="other-login-link">Log in</a> to save your preference
                           </div>`
                    }
                </div>
                <div class="other-actions">
                    <button type="button" class="other-cancel-btn">Cancel</button>
                </div>
            </div>
        </div>
    `;

    dom.registrarCards.innerHTML = cardsHtml;

    // Track clicks on main registrar buttons
    dom.registrarCards.querySelectorAll('.registrar-btn[data-registrar]').forEach(btn => {
        btn.addEventListener('click', () => {
            const registrarId = btn.dataset.registrar;
            const clickedDomain = btn.dataset.domain;

            fetch('/api/track/affiliate', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ domain: clickedDomain, registrar: registrarId })
            }).catch(() => {});
        });
    });

    // "Other" card handlers
    setupOtherCardHandlers(domain);
}

function setupOtherCardHandlers(domain) {
    const otherCardEl = dom.registrarCards.querySelector('.registrar-card-other');
    if (!otherCardEl) return;

    const collapsedEl = otherCardEl.querySelector('.other-collapsed');
    const expandedEl = otherCardEl.querySelector('.other-expanded');
    const expandBtn = otherCardEl.querySelector('.other-expand-btn');
    const cancelBtn = otherCardEl.querySelector('.other-cancel-btn');
    const input = otherCardEl.querySelector('.other-input');
    const dropdown = otherCardEl.querySelector('.autocomplete-dropdown');
    const selectedEl = otherCardEl.querySelector('.other-selected');
    const selectedNameEl = otherCardEl.querySelector('.other-selected-name');
    const registerBtn = otherCardEl.querySelector('.other-register-btn');
    const savePromptEl = otherCardEl.querySelector('.other-save-prompt');
    const saveCheckbox = otherCardEl.querySelector('.other-save-checkbox');
    const loginLink = otherCardEl.querySelector('.other-login-link');

    let highlightedIndex = -1;
    let selectedRegistrar = null;

    function showDropdown() {
        dropdown.hidden = false;
    }

    function hideDropdown() {
        dropdown.hidden = true;
        highlightedIndex = -1;
        clearHighlight();
    }

    function clearHighlight() {
        dropdown.querySelectorAll('.autocomplete-item').forEach(item => {
            item.classList.remove('highlighted');
        });
    }

    function highlightItem(index) {
        const visibleItems = getVisibleItems();
        clearHighlight();
        if (index >= 0 && index < visibleItems.length) {
            visibleItems[index].classList.add('highlighted');
            visibleItems[index].scrollIntoView({ block: 'nearest' });
        }
    }

    function getVisibleItems() {
        return [...dropdown.querySelectorAll('.autocomplete-item:not([hidden])')];
    }

    function filterSuggestions(query) {
        const lowerQuery = query.toLowerCase().trim();
        dropdown.querySelectorAll('.autocomplete-item').forEach(item => {
            const name = item.dataset.name.toLowerCase();
            const matches = !lowerQuery || name.includes(lowerQuery);
            item.hidden = !matches;
        });
        highlightedIndex = -1;
        clearHighlight();
    }

    function selectRegistrar(registrarId) {
        const registrar = OTHER_REGISTRARS[registrarId];
        if (!registrar) return;

        selectedRegistrar = registrar;
        input.value = registrar.name;
        hideDropdown();

        // Show selected state with register button
        selectedNameEl.textContent = registrar.name;
        registerBtn.href = registrar.getUrl(domain);
        selectedEl.hidden = false;
        savePromptEl.hidden = false;

        // Track selection
        fetch('/api/track/affiliate', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ domain, registrar: 'other', otherRegistrar: registrarId })
        }).catch(() => {});
    }

    function resetSelection() {
        selectedRegistrar = null;
        input.value = '';
        selectedEl.hidden = true;
        savePromptEl.hidden = true;
        if (saveCheckbox) saveCheckbox.checked = false;
    }

    // Expand card
    expandBtn.addEventListener('click', () => {
        collapsedEl.hidden = true;
        expandedEl.hidden = false;
        otherCardEl.classList.add('expanded');
        input.focus();
    });

    // Cancel / collapse
    cancelBtn.addEventListener('click', () => {
        collapsedEl.hidden = false;
        expandedEl.hidden = true;
        otherCardEl.classList.remove('expanded');
        resetSelection();
        hideDropdown();
    });

    // Input focus shows dropdown
    input.addEventListener('focus', () => {
        filterSuggestions(input.value);
        showDropdown();
    });

    // Input typing filters dropdown
    input.addEventListener('input', () => {
        filterSuggestions(input.value);
        showDropdown();
        // Reset selection if user is typing again
        if (selectedRegistrar && input.value !== selectedRegistrar.name) {
            resetSelection();
        }
    });

    // Keyboard navigation
    input.addEventListener('keydown', (e) => {
        const visibleItems = getVisibleItems();

        if (e.key === 'ArrowDown') {
            e.preventDefault();
            if (dropdown.hidden) {
                showDropdown();
            } else {
                highlightedIndex = Math.min(highlightedIndex + 1, visibleItems.length - 1);
                highlightItem(highlightedIndex);
            }
        } else if (e.key === 'ArrowUp') {
            e.preventDefault();
            highlightedIndex = Math.max(highlightedIndex - 1, 0);
            highlightItem(highlightedIndex);
        } else if (e.key === 'Enter') {
            e.preventDefault();
            if (highlightedIndex >= 0 && highlightedIndex < visibleItems.length) {
                const item = visibleItems[highlightedIndex];
                selectRegistrar(item.dataset.value);
            }
        } else if (e.key === 'Escape') {
            hideDropdown();
        } else if (e.key === 'Tab') {
            hideDropdown();
        }
    });

    // Click on dropdown item
    dropdown.addEventListener('click', (e) => {
        const item = e.target.closest('.autocomplete-item');
        if (item) {
            selectRegistrar(item.dataset.value);
        }
    });

    // Close dropdown when clicking outside
    document.addEventListener('click', (e) => {
        if (!otherCardEl.contains(e.target)) {
            hideDropdown();
        }
    });

    // Register button click - save preference if checkbox checked
    registerBtn.addEventListener('click', async () => {
        if (saveCheckbox && saveCheckbox.checked && selectedRegistrar && currentUser) {
            await savePreferredOtherRegistrar(selectedRegistrar.id);
        }
    });

    // Login link in save prompt
    if (loginLink) {
        loginLink.addEventListener('click', (e) => {
            e.preventDefault();
            openLoginModal();
        });
    }
}

export function initRegistration() {
    // Back button handler
    dom.registrationBack.addEventListener('click', hideRegistrationView);

    // Login link in preference area (legacy, keeping for compatibility)
    if (dom.loginForPrefLink) {
        dom.loginForPrefLink.addEventListener('click', (e) => {
            e.preventDefault();
            openLoginModal();
        });
    }
}

// Make showRegistrationView available globally for legacy code
window.showRegistrationView = showRegistrationView;
