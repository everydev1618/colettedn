import { dom } from '../dom.js';
import { authToken, currentUser, comSiteChecks, setUsageInfo, setComSiteChecks } from '../state.js';
import { FUNCTION_URL } from '../config.js';
import { shakeElement } from '../utils.js';
import { openLoginModal } from '../auth/login-modal.js';
import { openUpgradeModal } from '../modals/upgrade.js';
import { showTabLimitModal } from '../modals/tab-limit.js';
import { createTab, getActiveTab, canCreateNewTab } from '../tabs/index.js';
import { renderTabBar } from '../tabs/render.js';
import { saveTabsToStorage } from '../tabs/persistence.js';
import { renderResultsForTab, showErrorForTab, saveSearchHistory, generateTabTitle } from './results.js';
import { showMaintenanceMode } from './maintenance.js';

// Track custom TLD mode
let isCustomTldMode = false;

// Get selected custom TLDs from chips
function getSelectedTlds() {
    const chips = document.querySelectorAll('.tld-chip.active');
    return Array.from(chips).map(c => c.dataset.tld);
}

// Update chip selection based on a preset
function setChipsFromPreset(preset) {
    const presets = {
        traditional: ['com', 'co', 'net', 'org'],
        creative: ['com', 'io', 'ai', 'app', 'dev', 'co'],
        global: ['co.uk', 'de', 'eu']
    };
    const tlds = presets[preset] || presets.traditional;
    document.querySelectorAll('.tld-chip').forEach(chip => {
        chip.classList.toggle('active', tlds.includes(chip.dataset.tld));
    });
}

// Exit custom mode and select a preset
function exitCustomMode(preset = 'traditional') {
    isCustomTldMode = false;
    dom.tldCustomizeBtn.classList.remove('active');
    dom.tldCustomPanel.hidden = true;

    // Activate the preset button
    document.querySelectorAll('.tld-toggle').forEach(b => {
        b.classList.toggle('active', b.dataset.value === preset);
    });
    dom.tldStyleInput.value = preset;
}

export function initSearchForm() {
    // TLD toggle handlers
    document.querySelectorAll('.tld-toggle').forEach(btn => {
        btn.addEventListener('click', () => {
            // Exit custom mode when clicking a preset
            exitCustomMode(btn.dataset.value);
            setChipsFromPreset(btn.dataset.value);
        });
    });

    // Customize button handler
    dom.tldCustomizeBtn.addEventListener('click', () => {
        isCustomTldMode = !isCustomTldMode;
        dom.tldCustomizeBtn.classList.toggle('active', isCustomTldMode);
        dom.tldCustomPanel.hidden = !isCustomTldMode;

        if (isCustomTldMode) {
            // Deselect preset buttons
            document.querySelectorAll('.tld-toggle').forEach(b => b.classList.remove('active'));
            dom.tldStyleInput.value = 'custom';
        }
    });

    // TLD chip toggle handlers
    document.querySelectorAll('.tld-chip').forEach(chip => {
        chip.addEventListener('click', () => {
            chip.classList.toggle('active');
            // Ensure at least one TLD is selected
            if (getSelectedTlds().length === 0) {
                chip.classList.add('active');
            }
        });
    });

    // Done button handler
    dom.tldCustomDone.addEventListener('click', () => {
        dom.tldCustomPanel.hidden = true;
        // Keep customize button active to show we're in custom mode
    });

    // New tab button handler
    dom.tabNewBtn.addEventListener('click', () => {
        // Check if user can create a new tab (free limit check)
        if (!canCreateNewTab()) {
            showTabLimitModal();
            return;
        }
        createTab();
        dom.descriptionInput.value = '';
        dom.descriptionInput.focus();
        // Show welcome content for the new empty tab
        dom.resultsEl.hidden = true;
        dom.welcomeContent.hidden = false;
        dom.favoritesView.hidden = true;
        dom.historyView.hidden = true;
        if (dom.registrationView) dom.registrationView.hidden = true;
    });

    dom.form.addEventListener('submit', handleFormSubmit);
}

async function handleFormSubmit(e) {
    e.preventDefault();

    const description = dom.descriptionInput.value.trim();
    const tldStyle = dom.tldStyleInput.value;

    // Get custom TLDs if in custom mode
    const customTlds = isCustomTldMode ? getSelectedTlds() : null;

    if (!description) {
        shakeElement(dom.descriptionInput);
        return;
    }

    // Validate at least one TLD is selected in custom mode
    if (isCustomTldMode && customTlds.length === 0) {
        shakeElement(dom.tldCustomizeBtn);
        return;
    }

    // Hide favorites/history/welcome/registration view if showing
    dom.favoritesView.hidden = true;
    dom.historyView.hidden = true;
    dom.welcomeContent.hidden = true;
    if (dom.registrationView) dom.registrationView.hidden = true;

    // Determine if we should create a new tab or reuse active
    let tab = getActiveTab();
    const shouldCreateNewTab = !tab || Object.keys(tab.categories).length > 0 || tab.error;

    if (shouldCreateNewTab) {
        // Check if user can create a new tab (free limit check)
        if (!canCreateNewTab()) {
            showTabLimitModal();
            return;
        }
        tab = createTab(description, tldStyle);
    } else {
        // Reuse empty tab
        tab.description = description;
        tab.tldStyle = tldStyle;
        renderTabBar();
    }

    // Show loading state
    dom.submitBtn.disabled = true;
    dom.btnText.hidden = true;
    dom.btnLoading.hidden = false;
    tab.categories = {};
    tab.error = null;
    tab.isLoading = true;
    setComSiteChecks(tab.comSiteChecks);
    renderTabBar();

    dom.resultsEl.innerHTML = `
        <div class="searching-state">
            <div class="search-animation">
                <div class="orbit">
                    <div class="orbit-dot"></div>
                    <div class="orbit-dot"></div>
                    <div class="orbit-dot"></div>
                </div>
                <div class="orbit orbit-reverse">
                    <div class="orbit-dot"></div>
                    <div class="orbit-dot"></div>
                </div>
                <div class="search-icon">◇</div>
            </div>
            <p class="search-text">Searching for available domains<span class="search-dots"></span></p>
            <p class="search-subtext">May run up to 5 rounds to find the best options</p>
            <div class="tld-parade">
                <span>.com</span><span>.io</span><span>.co</span><span>.dev</span><span>.app</span><span>.ai</span>
            </div>
        </div>`;
    dom.resultsEl.hidden = false;

    try {
        // Build request body - use custom TLDs if in custom mode, otherwise use style
        const requestBody = { description };
        if (customTlds) {
            requestBody.tlds = customTlds;
        } else {
            requestBody.tldStyle = tldStyle;
        }

        // Use Function URL if configured (no timeout), with fallback to API Gateway
        // Some ad blockers may block lambda-url domains, so we fall back to relative path
        let response;
        const functionUrl = FUNCTION_URL ? `${FUNCTION_URL}api/generate` : null;
        const apiGatewayUrl = '/api/generate';

        if (functionUrl) {
            try {
                response = await fetch(functionUrl, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(requestBody),
                });
            } catch (fetchErr) {
                // Function URL failed (likely blocked), fall back to API Gateway
                console.warn('Function URL failed, falling back to API Gateway:', fetchErr.message);
                response = await fetch(apiGatewayUrl, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(requestBody),
                });
            }
        } else {
            response = await fetch(apiGatewayUrl, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(requestBody),
            });
        }

        // Handle service unavailable (kill switch active)
        if (response.status === 503) {
            tab.isLoading = false;
            renderTabBar();
            showMaintenanceMode();
            return;
        }

        const data = await response.json();

        // Handle rate limiting - show upgrade modal if available
        if (response.status === 429) {
            tab.isLoading = false;
            tab.error = data.error || 'Too many requests. Please wait a moment.';
            renderTabBar();
            if (data.upgradeAvailable && authToken) {
                openUpgradeModal();
            } else if (data.upgradeAvailable && !authToken) {
                // Show login first, then upgrade
                openLoginModal();
            } else {
                showErrorForTab(tab);
            }
            return;
        }

        if (data.error) {
            tab.isLoading = false;
            tab.error = data.error;
            renderTabBar();
            showErrorForTab(tab);
            return;
        }

        // Results come back with availability already checked
        tab.categories = data.categories || {};
        tab.unavailable = data.unavailable || [];
        tab.searchedDomain = data.searchedDomain || [];
        tab.rounds = data.rounds || 1;
        tab.isLoading = false;
        tab.error = null;

        // Capture usage info
        if (data.usage) {
            setUsageInfo(data.usage);
        }

        renderTabBar();
        renderResultsForTab(tab);
        saveTabsToStorage();

        // Save to history if logged in
        saveSearchHistory(description, tldStyle, tab.categories);

        // Generate a title for the tab (async, don't block)
        generateTabTitle(tab.id, description);

    } catch (err) {
        tab.isLoading = false;
        // Provide more specific error messages
        let errorMessage = 'Failed to generate domains. Please try again.';
        if (err.name === 'TypeError' && err.message.includes('fetch')) {
            errorMessage = 'Network error. Please check your connection and try again.';
        } else if (err.name === 'AbortError') {
            errorMessage = 'Request timed out. Please try again.';
        }
        tab.error = errorMessage;
        renderTabBar();
        showErrorForTab(tab);
        saveTabsToStorage();
        console.error('Generate error:', err.name, err.message, err);
    } finally {
        dom.submitBtn.disabled = false;
        dom.btnText.hidden = false;
        dom.btnLoading.hidden = true;
    }
}
