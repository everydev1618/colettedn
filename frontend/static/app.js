// Lambda Function URL for generate endpoint (no timeout limit)
// Falls back to relative path if not set
const FUNCTION_URL = 'https://4tpzgbt5zo7kade7egg5uu75jy0inpuj.lambda-url.us-east-1.on.aws/';

// Theme handling - initialize early to prevent flash
(function() {
    const savedTheme = localStorage.getItem('theme') || 'dark';
    document.documentElement.setAttribute('data-theme', savedTheme);
})();

// Auth state
let authToken = localStorage.getItem('authToken');
let currentUser = null;
let userFavorites = new Set();
let userOwnedDomains = new Map(); // domain -> { acquisitionType, createdAt }
let usageInfo = null; // { used, limit, unlimited }

// Tab state
let tabs = [];          // Array of tab objects
let activeTabId = null; // Currently active tab
let tabCounter = 0;     // For unique IDs

document.addEventListener('DOMContentLoaded', () => {
    const form = document.getElementById('generate-form');
    const submitBtn = document.getElementById('submit-btn');
    const btnText = submitBtn.querySelector('.btn-text');
    const btnLoading = submitBtn.querySelector('.btn-loading');
    const resultsEl = document.getElementById('results');
    const welcomeContent = document.getElementById('welcome-content');
    const tldStyleInput = document.getElementById('tld-style');
    const maintenanceOverlay = document.getElementById('maintenance-overlay');
    const maintenanceCountdown = document.getElementById('maintenance-countdown');

    // Tour elements
    const tourOverlay = document.getElementById('onboarding-tour');
    const tourSpotlight = document.getElementById('tour-spotlight');
    const tourTooltip = document.getElementById('tour-tooltip');
    const getStartedBtn = document.getElementById('get-started-btn');
    const statDomainsEl = document.getElementById('stat-domains');

    // Auth elements
    const loginModal = document.getElementById('login-modal');
    const loginForm = document.getElementById('login-form');
    const loginEmail = document.getElementById('login-email');
    const loginSubmitBtn = document.getElementById('login-submit-btn');
    const loginBtnText = loginSubmitBtn.querySelector('.login-btn-text');
    const loginBtnLoading = loginSubmitBtn.querySelector('.login-btn-loading');
    const loginSent = document.getElementById('login-sent');
    const sentEmail = document.getElementById('sent-email');
    const loginError = document.getElementById('login-error');
    const loginClose = document.getElementById('login-close');
    const loginModalText = loginModal.querySelector('.modal-text');

    // User menu elements
    const signInBtn = document.getElementById('sign-in-btn');
    const userDropdown = document.getElementById('user-dropdown');
    const userBtn = document.getElementById('user-btn');
    const userEmailEl = document.getElementById('user-email');
    const dropdownMenu = document.getElementById('dropdown-menu');
    const searchConsoleBtn = document.getElementById('search-console-btn');
    const favoritesBtn = document.getElementById('favorites-btn');
    const logoutBtn = document.getElementById('logout-btn');

    // Favorites view elements
    const favoritesView = document.getElementById('favorites-view');
    const favoritesList = document.getElementById('favorites-list');
    const favoritesClose = document.getElementById('favorites-close');

    // History view elements
    const historyView = document.getElementById('history-view');
    const historyList = document.getElementById('history-list');
    const historyClose = document.getElementById('history-close');
    const historyBtn = document.getElementById('history-btn');

    // Upgrade modal elements
    const upgradeModal = document.getElementById('upgrade-modal');
    const upgradeBtn = document.getElementById('upgrade-btn');
    const upgradeBtnText = upgradeBtn.querySelector('.upgrade-btn-text');
    const upgradeBtnLoading = upgradeBtn.querySelector('.upgrade-btn-loading');
    const upgradeClose = document.getElementById('upgrade-close');
    const upgradeError = document.getElementById('upgrade-error');

    // Upgrade/manage buttons in dropdowns
    const upgradeMenuBtn = document.getElementById('upgrade-menu-btn');
    const manageBtn = document.getElementById('manage-btn');

    // Admin buttons
    const adminBtn = document.getElementById('admin-btn');
    const ADMIN_EMAIL = 'etdebruin@gmail.com';

    // Plan info elements
    const planName = document.getElementById('plan-name');
    const planDetail = document.getElementById('plan-detail');

    // Owned modal elements
    const ownedModal = document.getElementById('owned-modal');
    const ownedClose = document.getElementById('owned-close');
    const ownedDomainName = document.getElementById('owned-domain-name');
    const ownedError = document.getElementById('owned-error');
    let pendingOwnedDomain = null;

    // Theme toggle elements
    const themeToggleBtn = document.getElementById('theme-toggle-btn');
    const themeToggleDropdown = document.getElementById('theme-toggle-dropdown');

    // Tab bar elements
    const tabBar = document.getElementById('tab-bar');
    const tabList = document.getElementById('tab-list');
    const tabNewBtn = document.getElementById('tab-new-btn');

    // Per-session .com site checks (moved from global since it's per-tab now)
    let comSiteChecks = new Map(); // baseName -> { status, domain }

    // Theme toggle functionality
    function toggleTheme() {
        const currentTheme = document.documentElement.getAttribute('data-theme');
        const newTheme = currentTheme === 'dark' ? 'light' : 'dark';
        document.documentElement.setAttribute('data-theme', newTheme);
        localStorage.setItem('theme', newTheme);

        // Sync with server if logged in
        if (authToken) {
            apiFetch('/api/user/preferences', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ theme: newTheme })
            }).catch(err => console.error('Failed to save theme preference:', err));
        }
    }

    // Apply theme from user account (called after login)
    function applyUserTheme(theme) {
        if (theme && (theme === 'light' || theme === 'dark')) {
            document.documentElement.setAttribute('data-theme', theme);
            localStorage.setItem('theme', theme);
        }
    }

    themeToggleBtn.addEventListener('click', toggleTheme);
    themeToggleDropdown.addEventListener('click', toggleTheme);

    // Maintenance mode handling
    let maintenanceTimer = null;

    // Check for auth token in URL hash (after magic link redirect)
    checkAuthFromHash();

    // Initialize auth state
    if (authToken) {
        fetchCurrentUser();
    }

    function checkAuthFromHash() {
        const hash = window.location.hash;
        if (hash.startsWith('#token=')) {
            const token = hash.substring(7);
            authToken = token;
            localStorage.setItem('authToken', token);
            // Clear the hash
            history.replaceState(null, '', window.location.pathname);
            fetchCurrentUser(true); // true = just signed in
        }
    }

    async function fetchCurrentUser(justSignedIn = false) {
        try {
            const response = await apiFetch('/api/user/me');
            if (response.ok) {
                const data = await response.json();
                currentUser = data.user;
                updateAuthUI();
                await fetchFavorites();
                await fetchOwnedDomains();
            } else {
                // Invalid token
                logout();
            }
        } catch (err) {
            console.error('Failed to fetch user:', err);
            logout();
        }
    }

    async function fetchFavorites() {
        if (!authToken) return;
        try {
            const response = await apiFetch('/api/favorites');
            if (response.ok) {
                const data = await response.json();
                userFavorites = new Set(data.favorites.map(f => f.domain.toLowerCase()));
            }
        } catch (err) {
            console.error('Failed to fetch favorites:', err);
        }
    }

    async function fetchOwnedDomains() {
        if (!authToken) return;
        try {
            const response = await apiFetch('/api/owned');
            if (response.ok) {
                const data = await response.json();
                userOwnedDomains = new Map();
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

    function updateAuthUI() {
        const isPro = currentUser && currentUser.subscriptionTier === 'pro';

        if (currentUser) {
            // Header (app layout)
            signInBtn.hidden = true;
            userDropdown.hidden = false;
            userEmailEl.innerHTML = isPro
                ? `<span class="user-email-text">${escapeHtml(currentUser.email)}</span><span class="pro-badge">Pro</span>`
                : `<span class="user-email-text">${escapeHtml(currentUser.email)}</span>`;

            // Update plan info in dropdown
            if (isPro) {
                planName.textContent = 'Pro';
                planName.classList.add('plan-pro');
                planDetail.textContent = 'Unlimited searches';
            } else {
                planName.textContent = 'Free';
                planName.classList.remove('plan-pro');
                planDetail.textContent = '3 searches/day';
            }

            // Show upgrade or manage button based on tier
            upgradeMenuBtn.hidden = isPro;
            manageBtn.hidden = !isPro;

            // Show admin button only for admin email
            const isAdmin = currentUser.email === ADMIN_EMAIL;
            adminBtn.hidden = !isAdmin;
        } else {
            // Header (app layout)
            signInBtn.hidden = false;
            userDropdown.hidden = true;
        }
    }

    function logout() {
        authToken = null;
        currentUser = null;
        userFavorites.clear();
        userOwnedDomains.clear();
        localStorage.removeItem('authToken');
        updateAuthUI();
        // POST to logout endpoint (fire and forget)
        fetch('/api/auth/logout', { method: 'POST', headers: getAuthHeaders() }).catch(() => {});
        // Show welcome content
        showWelcomeContent();
    }

    function getAuthHeaders() {
        const headers = { 'Content-Type': 'application/json' };
        if (authToken) {
            headers['Authorization'] = `Bearer ${authToken}`;
        }
        return headers;
    }

    async function apiFetch(url, options = {}) {
        const headers = { ...getAuthHeaders(), ...options.headers };
        return fetch(url, { ...options, headers });
    }

    // Login modal handlers
    function openLoginModal() {
        loginModal.hidden = false;
        loginForm.hidden = false;
        loginSent.hidden = true;
        loginError.hidden = true;
        loginModalText.hidden = false;
        loginEmail.value = '';
        loginEmail.focus();
    }

    signInBtn.addEventListener('click', openLoginModal);

    loginClose.addEventListener('click', () => {
        loginModal.hidden = true;
    });

    loginModal.addEventListener('click', (e) => {
        if (e.target === loginModal) {
            loginModal.hidden = true;
        }
    });

    loginForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        const email = loginEmail.value.trim();
        if (!email) {
            shakeElement(loginEmail);
            return;
        }

        loginSubmitBtn.disabled = true;
        loginBtnText.hidden = true;
        loginBtnLoading.hidden = false;
        loginError.hidden = true;

        try {
            const response = await fetch('/api/auth/login', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ email })
            });

            const data = await response.json();

            if (response.ok && data.success) {
                loginForm.hidden = true;
                loginModalText.hidden = true;
                loginSent.hidden = false;
                sentEmail.textContent = email;
            } else {
                loginError.textContent = data.error || 'Failed to send login email';
                loginError.hidden = false;
            }
        } catch (err) {
            loginError.textContent = 'Failed to send login email. Please try again.';
            loginError.hidden = false;
        } finally {
            loginSubmitBtn.disabled = false;
            loginBtnText.hidden = false;
            loginBtnLoading.hidden = true;
        }
    });

    // Upgrade modal handlers
    function openUpgradeModal() {
        upgradeModal.hidden = false;
        upgradeError.hidden = true;
    }

    upgradeClose.addEventListener('click', () => {
        upgradeModal.hidden = true;
    });

    upgradeModal.addEventListener('click', (e) => {
        if (e.target === upgradeModal) {
            upgradeModal.hidden = true;
        }
    });

    upgradeBtn.addEventListener('click', async () => {
        upgradeBtn.disabled = true;
        upgradeBtnText.hidden = true;
        upgradeBtnLoading.hidden = false;
        upgradeError.hidden = true;

        try {
            const response = await apiFetch('/api/billing/checkout', {
                method: 'POST'
            });

            const data = await response.json();

            if (response.ok && data.url) {
                // Redirect to Stripe Checkout
                window.location.href = data.url;
            } else {
                upgradeError.textContent = data.error || 'Failed to start checkout';
                upgradeError.hidden = false;
            }
        } catch (err) {
            upgradeError.textContent = 'Failed to start checkout. Please try again.';
            upgradeError.hidden = false;
        } finally {
            upgradeBtn.disabled = false;
            upgradeBtnText.hidden = false;
            upgradeBtnLoading.hidden = true;
        }
    });

    // Upgrade button in dropdown
    upgradeMenuBtn.addEventListener('click', () => {
        dropdownMenu.hidden = true;
        userDropdown.classList.remove('open');
        openUpgradeModal();
    });

    // Manage subscription handlers
    async function openManageSubscription() {
        try {
            const response = await apiFetch('/api/billing/portal', {
                method: 'POST'
            });

            const data = await response.json();

            if (response.ok && data.url) {
                window.location.href = data.url;
            } else {
                alert(data.error || 'Failed to open subscription management');
            }
        } catch (err) {
            alert('Failed to open subscription management. Please try again.');
        }
    }

    manageBtn.addEventListener('click', () => {
        dropdownMenu.hidden = true;
        userDropdown.classList.remove('open');
        openManageSubscription();
    });

    // Admin button handler
    adminBtn.addEventListener('click', () => {
        dropdownMenu.hidden = true;
        userDropdown.classList.remove('open');
        window.location.href = '/admin';
    });

    // Owned domain modal handlers
    function openOwnedModal(domain) {
        if (!authToken) {
            openLoginModal();
            return;
        }
        pendingOwnedDomain = domain;
        ownedDomainName.textContent = domain;
        ownedError.hidden = true;
        ownedModal.hidden = false;
    }

    ownedClose.addEventListener('click', () => {
        ownedModal.hidden = true;
        pendingOwnedDomain = null;
    });

    ownedModal.addEventListener('click', (e) => {
        if (e.target === ownedModal) {
            ownedModal.hidden = true;
            pendingOwnedDomain = null;
        }
    });

    // Owned option click handlers
    ownedModal.querySelectorAll('.owned-option').forEach(btn => {
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
                    ownedModal.hidden = true;
                    pendingOwnedDomain = null;
                    // Re-render to show owned badge
                    if (!favoritesView.hidden) {
                        renderFavoritesView();
                    } else {
                        const activeTab = getActiveTab();
                        if (activeTab && Object.keys(activeTab.categories).length > 0) {
                            renderResultsForTab(activeTab);
                        }
                    }
                } else {
                    const data = await response.json();
                    ownedError.textContent = data.error || 'Failed to mark domain as owned';
                    ownedError.hidden = false;
                }
            } catch (err) {
                ownedError.textContent = 'Failed to mark domain as owned. Please try again.';
                ownedError.hidden = false;
            } finally {
                btn.disabled = false;
            }
        });
    });

    async function removeOwnedDomain(domain) {
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

    // User dropdown handlers
    userBtn.addEventListener('click', () => {
        const isOpen = !dropdownMenu.hidden;
        dropdownMenu.hidden = isOpen;
        userDropdown.classList.toggle('open', !isOpen);
    });

    // Close dropdown when clicking outside
    document.addEventListener('click', (e) => {
        if (!userDropdown.contains(e.target)) {
            dropdownMenu.hidden = true;
            userDropdown.classList.remove('open');
        }
    });

    searchConsoleBtn.addEventListener('click', () => {
        dropdownMenu.hidden = true;
        userDropdown.classList.remove('open');
        favoritesView.hidden = true;
        historyView.hidden = true;
        const activeTab = getActiveTab();
        if (activeTab && Object.keys(activeTab.categories).length > 0) {
            switchToTab(activeTab.id);
        } else {
            welcomeContent.hidden = false;
        }
    });

    favoritesBtn.addEventListener('click', () => {
        dropdownMenu.hidden = true;
        userDropdown.classList.remove('open');
        showFavoritesView();
    });

    historyBtn.addEventListener('click', () => {
        dropdownMenu.hidden = true;
        userDropdown.classList.remove('open');
        showHistoryView();
    });

    logoutBtn.addEventListener('click', () => {
        dropdownMenu.hidden = true;
        userDropdown.classList.remove('open');
        logout();
    });

    // Favorites view handlers
    async function showFavoritesView() {
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
        welcomeContent.hidden = true;
        resultsEl.hidden = true;
        historyView.hidden = true;
        favoritesView.hidden = false;
        // Hide registration view if showing
        const regView = document.getElementById('registration-view');
        if (regView) regView.hidden = true;
        await renderFavoritesView();
    }

    function hideFavoritesView() {
        favoritesView.hidden = true;
        historyView.hidden = true;
        const activeTab = getActiveTab();
        if (activeTab && Object.keys(activeTab.categories).length > 0) {
            switchToTab(activeTab.id);
        } else {
            welcomeContent.hidden = false;
        }
    }

    favoritesClose.addEventListener('click', hideFavoritesView);

    // History view handlers
    async function showHistoryView() {
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
        welcomeContent.hidden = true;
        resultsEl.hidden = true;
        favoritesView.hidden = true;
        historyView.hidden = false;
        // Hide registration view if showing
        const regView = document.getElementById('registration-view');
        if (regView) regView.hidden = true;
        await renderHistoryView();
    }

    function hideHistoryView() {
        historyView.hidden = true;
        favoritesView.hidden = true;
        const activeTab = getActiveTab();
        if (activeTab && Object.keys(activeTab.categories).length > 0) {
            switchToTab(activeTab.id);
        } else {
            welcomeContent.hidden = false;
        }
    }

    historyClose.addEventListener('click', hideHistoryView);

    async function renderHistoryView() {
        try {
            const response = await apiFetch('/api/history');
            if (!response.ok) {
                historyList.innerHTML = `
                    <div class="history-empty">
                        <p>Failed to load history</p>
                    </div>
                `;
                return;
            }

            const data = await response.json();
            const histories = data.histories || [];

            if (histories.length === 0) {
                historyList.innerHTML = `
                    <div class="history-empty">
                        <p>No searches yet</p>
                        <p class="history-empty-hint">Your searches will appear here</p>
                    </div>
                `;
                return;
            }

            historyList.innerHTML = histories.map((h, i) => {
                const date = new Date(h.searchedAt);
                const dateStr = formatHistoryDate(date);
                const tldLabel = h.tldStyle === 'creative' ? '.io .ai' : '.com .co';

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
                            ${allDomains.map(d => `<span class="history-domain-tag">${escapeHtml(d)}</span>`).join('')}
                        </div>
                    </div>
                `;
            }).join('');

            // Add search again handlers
            historyList.querySelectorAll('.history-search-btn').forEach(btn => {
                btn.addEventListener('click', () => {
                    const description = btn.dataset.description;
                    const tldStyle = btn.dataset.tld || 'traditional';

                    // Set values in the main form
                    document.getElementById('description').value = description;
                    tldStyleInput.value = tldStyle;
                    document.querySelectorAll('.tld-toggle').forEach(b => {
                        b.classList.toggle('active', b.dataset.value === tldStyle);
                    });

                    // Hide history view and trigger search
                    historyView.hidden = true;
                    form.dispatchEvent(new Event('submit'));
                });
            });

            // Add delete handlers
            historyList.querySelectorAll('.history-delete-btn').forEach(btn => {
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
            historyList.innerHTML = `
                <div class="history-empty">
                    <p>Failed to load history</p>
                </div>
            `;
        }
    }

    function formatHistoryDate(date) {
        const now = new Date();
        const diff = now - date;
        const minutes = Math.floor(diff / 60000);
        const hours = Math.floor(diff / 3600000);
        const days = Math.floor(diff / 86400000);

        if (minutes < 1) return 'Just now';
        if (minutes < 60) return `${minutes}m ago`;
        if (hours < 24) return `${hours}h ago`;
        if (days < 7) return `${days}d ago`;
        return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
    }

    // Save search to history (PRO only)
    async function saveSearchHistory(description, tldStyle, categoriesData) {
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
    async function generateTabTitle(tabId, searchPhrase) {
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

    async function renderFavoritesView() {
        if (userFavorites.size === 0) {
            favoritesList.innerHTML = `
                <div class="favorites-empty">
                    <p>No favorites yet</p>
                    <p class="favorites-empty-hint">Heart domains to save them here</p>
                </div>
            `;
            return;
        }

        const favArray = Array.from(userFavorites);
        favoritesList.innerHTML = favArray.map((domain, i) => {
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
        favoritesList.querySelectorAll('.favorite-btn').forEach(btn => {
            btn.addEventListener('click', async () => {
                await toggleFavorite(btn.dataset.domain);
                await renderFavoritesView();
            });
        });

        // Register button click handlers - open registration view
        favoritesList.querySelectorAll('.domain-register-btn').forEach(btn => {
            btn.addEventListener('click', () => {
                const domain = btn.dataset.domain;
                if (domain && typeof showRegistrationView === 'function') {
                    showRegistrationView(domain);
                }
            });
        });

        // Add own button handlers
        favoritesList.querySelectorAll('.own-btn').forEach(btn => {
            btn.addEventListener('click', (e) => {
                e.preventDefault();
                openOwnedModal(btn.dataset.domain);
            });
        });

        // Add unown button handlers
        favoritesList.querySelectorAll('.unown-btn').forEach(btn => {
            btn.addEventListener('click', async (e) => {
                e.preventDefault();
                await removeOwnedDomain(btn.dataset.domain);
                await renderFavoritesView();
            });
        });
    }

    async function toggleFavorite(domain) {
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

    function showMaintenanceMode() {
        maintenanceOverlay.hidden = false;
        document.body.style.overflow = 'hidden';

        // Start 15-minute countdown
        let remaining = 15 * 60; // 15 minutes in seconds

        function updateCountdown() {
            const mins = Math.floor(remaining / 60);
            const secs = remaining % 60;
            maintenanceCountdown.textContent = `${mins}:${secs.toString().padStart(2, '0')}`;

            if (remaining > 0) {
                remaining--;
            }
        }

        updateCountdown();
        if (maintenanceTimer) clearInterval(maintenanceTimer);
        maintenanceTimer = setInterval(updateCountdown, 1000);
    }

    function hideMaintenanceMode() {
        maintenanceOverlay.hidden = true;
        document.body.style.overflow = '';
        if (maintenanceTimer) {
            clearInterval(maintenanceTimer);
            maintenanceTimer = null;
        }
    }

    // Logo click - show welcome content
    document.getElementById('logo-home').addEventListener('click', (e) => {
        e.preventDefault();
        showWelcomeContent();
    });

    // TLD toggle handlers
    document.querySelectorAll('.tld-toggle').forEach(btn => {
        btn.addEventListener('click', () => {
            document.querySelectorAll('.tld-toggle').forEach(b => b.classList.remove('active'));
            btn.classList.add('active');
            tldStyleInput.value = btn.dataset.value;
        });
    });

    // =========================================================================
    // Tab Management
    // =========================================================================

    function createTab(description = '', tldStyle = 'traditional') {
        tabCounter++;
        const tab = {
            id: `tab-${tabCounter}`,
            description: description,
            title: '', // AI-generated title
            tldStyle: tldStyle,
            categories: {},
            isLoading: false,
            error: null,
            rounds: 1,
            comSiteChecks: new Map()
        };

        // Max 10 tabs - remove oldest if exceeded
        if (tabs.length >= 10) {
            const oldestTab = tabs.shift();
            if (activeTabId === oldestTab.id) {
                activeTabId = null;
            }
        }

        tabs.push(tab);
        activeTabId = tab.id;
        renderTabBar();
        saveTabsToStorage();
        return tab;
    }

    function getActiveTab() {
        return tabs.find(t => t.id === activeTabId) || null;
    }

    function switchToTab(tabId) {
        const tab = tabs.find(t => t.id === tabId);
        if (!tab) return;

        activeTabId = tabId;
        comSiteChecks = tab.comSiteChecks;
        renderTabBar();
        saveTabsToStorage();

        // Hide other views
        favoritesView.hidden = true;
        historyView.hidden = true;
        welcomeContent.hidden = true;
        const regView = document.getElementById('registration-view');
        if (regView) regView.hidden = true;

        // Show tab content
        if (tab.isLoading) {
            resultsEl.innerHTML = `
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
            resultsEl.hidden = false;
        } else if (tab.error) {
            resultsEl.innerHTML = `<p class="error-message">${escapeHtml(tab.error)}</p>`;
            resultsEl.hidden = false;
        } else if (Object.keys(tab.categories).length > 0) {
            renderResultsForTab(tab);
            resultsEl.hidden = false;
        } else {
            // Empty tab - show welcome but keep tab bar visible
            resultsEl.hidden = true;
            welcomeContent.hidden = false;
        }
    }

    function closeTab(tabId) {
        const index = tabs.findIndex(t => t.id === tabId);
        if (index === -1) return;

        tabs.splice(index, 1);
        saveTabsToStorage();

        if (tabs.length === 0) {
            // No tabs left - show welcome
            activeTabId = null;
            tabBar.hidden = true;
            showWelcomeContent();
        } else if (activeTabId === tabId) {
            // Closed active tab - switch to nearest
            const newIndex = Math.min(index, tabs.length - 1);
            switchToTab(tabs[newIndex].id);
        } else {
            renderTabBar();
        }
    }

    function renderTabBar() {
        if (tabs.length === 0) {
            tabBar.hidden = true;
            return;
        }

        tabBar.hidden = false;

        tabList.innerHTML = tabs.map(tab => {
            const isActive = tab.id === activeTabId;
            // Use AI-generated title if available, otherwise fall back to description
            let title;
            if (tab.title) {
                title = tab.title.length > 20 ? tab.title.substring(0, 20) + '...' : tab.title;
            } else if (tab.description) {
                title = tab.description.length > 20 ? tab.description.substring(0, 20) + '...' : tab.description;
            } else {
                title = 'New Search';
            }

            return `
                <button class="tab${isActive ? ' active' : ''}" data-tab-id="${tab.id}">
                    ${tab.isLoading ? '<span class="tab-spinner"></span>' : ''}
                    <span class="tab-title">${escapeHtml(title)}</span>
                    <span class="tab-close" data-tab-id="${tab.id}">&times;</span>
                </button>
            `;
        }).join('');

        // Add tab click handlers
        tabList.querySelectorAll('.tab').forEach(tabEl => {
            tabEl.addEventListener('click', (e) => {
                // Don't switch if clicking close button
                if (e.target.classList.contains('tab-close')) return;
                switchToTab(tabEl.dataset.tabId);
            });
        });

        // Add close button handlers
        tabList.querySelectorAll('.tab-close').forEach(closeBtn => {
            closeBtn.addEventListener('click', (e) => {
                e.stopPropagation();
                closeTab(closeBtn.dataset.tabId);
            });
        });
    }

    // =========================================================================
    // Tab Persistence (localStorage)
    // =========================================================================

    function saveTabsToStorage() {
        try {
            // Convert tabs to serializable format (Maps become arrays)
            const serializableTabs = tabs.map(tab => ({
                ...tab,
                comSiteChecks: Array.from(tab.comSiteChecks.entries())
            }));
            localStorage.setItem('colette_tabs', JSON.stringify({
                tabs: serializableTabs,
                activeTabId: activeTabId,
                tabCounter: tabCounter
            }));
        } catch (err) {
            console.error('Failed to save tabs:', err);
        }
    }

    function loadTabsFromStorage() {
        try {
            const stored = localStorage.getItem('colette_tabs');
            if (!stored) return false;

            const data = JSON.parse(stored);
            if (!data.tabs || !Array.isArray(data.tabs)) return false;

            // Restore tabs with Maps
            tabs = data.tabs.map(tab => ({
                ...tab,
                comSiteChecks: new Map(tab.comSiteChecks || []),
                isLoading: false // Reset loading state on page load
            }));

            tabCounter = data.tabCounter || tabs.length;
            activeTabId = data.activeTabId;

            // Verify active tab still exists
            if (activeTabId && !tabs.find(t => t.id === activeTabId)) {
                activeTabId = tabs.length > 0 ? tabs[0].id : null;
            }

            if (tabs.length > 0) {
                renderTabBar();
                if (activeTabId) {
                    const activeTab = getActiveTab();
                    if (activeTab) {
                        comSiteChecks = activeTab.comSiteChecks;
                        if (Object.keys(activeTab.categories).length > 0) {
                            renderResultsForTab(activeTab);
                        }
                    }
                }
                return true;
            }
        } catch (err) {
            console.error('Failed to load tabs:', err);
            localStorage.removeItem('colette_tabs');
        }
        return false;
    }

    // Load tabs on page load
    const hadStoredTabs = loadTabsFromStorage();

    // New tab button handler
    tabNewBtn.addEventListener('click', () => {
        createTab();
        document.getElementById('description').value = '';
        document.getElementById('description').focus();
        // Show welcome content for the new empty tab
        resultsEl.hidden = true;
        welcomeContent.hidden = false;
        favoritesView.hidden = true;
        historyView.hidden = true;
        const regView = document.getElementById('registration-view');
        if (regView) regView.hidden = true;
    });

    form.addEventListener('submit', async (e) => {
        e.preventDefault();

        const description = document.getElementById('description').value.trim();
        const tldStyle = tldStyleInput.value;

        if (!description) {
            shakeElement(document.getElementById('description'));
            return;
        }

        // Hide favorites/history/welcome/registration view if showing
        favoritesView.hidden = true;
        historyView.hidden = true;
        welcomeContent.hidden = true;
        const regView = document.getElementById('registration-view');
        if (regView) regView.hidden = true;

        // Determine if we should create a new tab or reuse active
        let tab = getActiveTab();
        const shouldCreateNewTab = !tab || Object.keys(tab.categories).length > 0 || tab.error;

        if (shouldCreateNewTab) {
            tab = createTab(description, tldStyle);
        } else {
            // Reuse empty tab
            tab.description = description;
            tab.tldStyle = tldStyle;
            renderTabBar();
        }

        // Show loading state
        submitBtn.disabled = true;
        btnText.hidden = true;
        btnLoading.hidden = false;
        tab.categories = {};
        tab.error = null;
        tab.isLoading = true;
        comSiteChecks = tab.comSiteChecks;
        renderTabBar();

        resultsEl.innerHTML = `
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
        resultsEl.hidden = false;

        try {
            // Use Function URL if configured (no timeout), otherwise fall back to API Gateway
            const generateUrl = FUNCTION_URL ? `${FUNCTION_URL}api/generate` : '/api/generate';
            const response = await fetch(generateUrl, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ description, tldStyle }),
            });

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
            tab.rounds = data.rounds || 1;
            tab.isLoading = false;
            tab.error = null;

            // Capture usage info
            if (data.usage) {
                usageInfo = data.usage;
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
            tab.error = 'Failed to generate domains. Please try again.';
            renderTabBar();
            showErrorForTab(tab);
            saveTabsToStorage();
            console.error(err);
        } finally {
            submitBtn.disabled = false;
            btnText.hidden = false;
            btnLoading.hidden = true;
        }
    });

    function showErrorForTab(tab) {
        if (activeTabId !== tab.id) return;
        resultsEl.innerHTML = `<p class="error-message">${escapeHtml(tab.error)}</p>`;
        welcomeContent.hidden = true;
        resultsEl.hidden = false;
    }

    // Convenience wrapper for old code that calls renderResults
    function renderResults(rounds) {
        const tab = getActiveTab();
        if (tab) {
            renderResultsForTab(tab);
        }
    }

    function renderResultsForTab(tab) {
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

        // Usage banner removed - let users enjoy the free tier without constant reminders

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

        // Results CTA removed - let users enjoy results without being pushed to upgrade

        resultsEl.innerHTML = searchPhraseHtml + sectionsHtml;
        resultsEl.hidden = false;
        welcomeContent.hidden = true;

        // Add click handler for "Edit search" button
        const copyBtn = resultsEl.querySelector('.search-phrase-copy');
        if (copyBtn) {
            copyBtn.addEventListener('click', () => {
                const descInput = document.getElementById('description');
                if (descInput && tab.description) {
                    descInput.value = tab.description;
                    descInput.focus();
                    descInput.select();
                    // Scroll to top if needed
                    window.scrollTo({ top: 0, behavior: 'smooth' });
                }
            });
        }

        // Add refresh button handlers
        resultsEl.querySelectorAll('.cache-refresh').forEach(btn => {
            btn.addEventListener('click', (e) => {
                e.preventDefault();
                refreshDomain(btn.dataset.domain);
            });
        });

        // Add favorite button handlers
        resultsEl.querySelectorAll('.favorite-btn').forEach(btn => {
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
        resultsEl.querySelectorAll('.domain-register-btn').forEach(btn => {
            btn.addEventListener('click', () => {
                const domain = btn.dataset.domain;
                if (domain && typeof showRegistrationView === 'function') {
                    showRegistrationView(domain);
                }
            });
        });

        // Add own button handlers
        resultsEl.querySelectorAll('.own-btn').forEach(btn => {
            btn.addEventListener('click', (e) => {
                e.preventDefault();
                openOwnedModal(btn.dataset.domain);
            });
        });

        // Add unown button handlers
        resultsEl.querySelectorAll('.unown-btn').forEach(btn => {
            btn.addEventListener('click', async (e) => {
                e.preventDefault();
                await removeOwnedDomain(btn.dataset.domain);
                renderResultsForTab(tab);
            });
        });


        // Add check .com button handlers
        resultsEl.querySelectorAll('.check-com-btn').forEach(btn => {
            btn.addEventListener('click', async (e) => {
                e.preventDefault();
                const domain = btn.dataset.domain;
                btn.disabled = true;
                btn.textContent = '...';

                const result = await checkComSite(domain);
                if (result) {
                    // Update in place - replace button with status
                    const statusHtml = getComStatusHtml(result.status, result.domain);
                    btn.outerHTML = statusHtml;
                } else {
                    btn.disabled = false;
                    btn.textContent = 'check .com';
                }
            });
        });
    }

    function getComStatusHtml(status, comDomain) {
        if (status === 'active') {
            return `<span class="com-status com-active" title="${comDomain} has an active website">⚠ .com active</span>`;
        } else if (status === 'parked') {
            return `<span class="com-status com-parked" title="${comDomain} is parked/for sale">◐ .com parked</span>`;
        } else if (status === 'available') {
            return `<span class="com-status com-available" title="${comDomain} is available!">✓ .com free</span>`;
        } else {
            return `<span class="com-status com-inactive" title="${comDomain} has no active site">✓ .com clear</span>`;
        }
    }

    function isComInResults(baseName, categories) {
        // Check if {baseName}.com is already in the results
        const comDomain = baseName + '.com';
        for (const cat of Object.values(categories)) {
            if (cat.some(d => d.name.toLowerCase() === comDomain)) {
                return true;
            }
        }
        return false;
    }

    function renderDomainCard(domain, index, categories) {
        let metaHtml = '';

        const statusClass = domain.available === false ? 'taken' : 'available';
        const statusText = domain.available === null ? 'Verify' : 'Available';

        metaHtml = `<span class="domain-status ${statusClass}">${statusText}</span>`;

        // Build score bar HTML if score available
        let scoreBarHtml = '';
        if (domain.score) {
            const scoreClass = domain.score >= 80 ? 'score-great' : domain.score >= 65 ? 'score-good' : 'score-fair';
            scoreBarHtml = `
                <div class="domain-score-bar ${scoreClass}" title="Quality score: ${domain.score}/100">
                    <div class="score-fill" style="width: ${domain.score}%"></div>
                </div>`;
        }

        if (domain.isPremium) {
            metaHtml += '<span class="domain-premium">Premium</span>';
        }
        if (domain.price) {
            metaHtml += `<span class="domain-price">$${domain.price.toFixed(0)}</span>`;
        }
        if (domain.fromCache && domain.checkedAt) {
            metaHtml += `<button class="cache-refresh" data-domain="${escapeHtml(domain.name)}">
                <span class="cache-time">${formatRelativeTime(domain.checkedAt)}</span>
                <span class="refresh-icon">↻</span>
            </button>`;
        }

        // Check .com status for non-.com domains
        const isComDomain = domain.name.toLowerCase().endsWith('.com');
        let comCheckHtml = '';
        if (!isComDomain) {
            const baseName = extractBaseName(domain.name);
            // Don't show check .com if the .com is already in results (it's available)
            if (isComInResults(baseName, categories)) {
                // .com is in results, no need to check
                comCheckHtml = '';
            } else {
                const comCheck = comSiteChecks.get(baseName);
                if (comCheck) {
                    // Already checked - show result
                    comCheckHtml = getComStatusHtml(comCheck.status, comCheck.domain);
                } else {
                    // Not checked yet - show link
                    comCheckHtml = `<button class="check-com-btn" data-domain="${escapeHtml(domain.name)}" title="Check if ${baseName}.com has a website">check .com</button>`;
                }
            }
        }

        const isFavorited = userFavorites.has(domain.name.toLowerCase());
        const heartIcon = isFavorited ? '♥' : '♡';
        const heartClass = isFavorited ? 'favorited' : '';

        // Check if domain is owned
        const ownedInfo = userOwnedDomains.get(domain.name.toLowerCase());
        const isOwned = !!ownedInfo;
        const ownedBadgeHtml = isOwned
            ? `<span class="owned-badge" title="${ownedInfo.acquisitionType === 'found_via_colette' ? 'Found on Colette' : 'Previously owned'}">✓ Owned</span>`
            : '';

        // Show "I own this" button or "Register" button based on ownership
        const actionHtml = isOwned
            ? `<button class="unown-btn" data-domain="${escapeHtml(domain.name)}" title="Remove ownership">✕</button>`
            : `<button class="domain-register-btn" data-domain="${escapeHtml(domain.name)}">Register &rarr;</button>`;

        return `
            <div class="domain-card${isOwned ? ' owned' : ''}" style="animation-delay: ${index * 0.03}s">
                <div class="domain-name-row">
                    <span class="domain-name">${escapeHtml(domain.name)}</span>
                    ${ownedBadgeHtml}
                    ${comCheckHtml}
                </div>
                ${scoreBarHtml}
                <div class="domain-row">
                    <div class="domain-meta">${metaHtml}</div>
                    <div class="domain-actions">
                        <button class="favorite-btn ${heartClass}" data-domain="${escapeHtml(domain.name)}" title="${isFavorited ? 'Remove from favorites' : 'Add to favorites'}">
                            ${heartIcon}
                        </button>
                        <button class="own-btn${isOwned ? ' hidden' : ''}" data-domain="${escapeHtml(domain.name)}" title="I own this domain">
                            ✓
                        </button>
                        ${actionHtml}
                    </div>
                </div>
            </div>
        `;
    }

    function extractBaseName(domain) {
        const tlds = ['.com', '.io', '.co', '.net', '.org', '.ai', '.app', '.dev', '.me', '.xyz', '.tech', '.site', '.online'];
        const lowerDomain = domain.toLowerCase();
        for (const tld of tlds) {
            if (lowerDomain.endsWith(tld)) {
                return lowerDomain.slice(0, -tld.length);
            }
        }
        const lastDot = lowerDomain.lastIndexOf('.');
        return lastDot > 0 ? lowerDomain.slice(0, lastDot) : lowerDomain;
    }

    async function checkComSite(domain) {
        const baseName = extractBaseName(domain);

        try {
            const response = await fetch('/api/check-com', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ domain })
            });

            if (response.ok) {
                const data = await response.json();
                comSiteChecks.set(baseName, {
                    status: data.status,
                    domain: data.domain
                });
                saveTabsToStorage();
                return data;
            }
        } catch (err) {
            console.error('Failed to check .com site:', err);
        }
        return null;
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

    function formatRelativeTime(timestamp) {
        const diff = Date.now() - timestamp * 1000;
        const minutes = Math.floor(diff / 60000);
        const hours = Math.floor(diff / 3600000);
        if (minutes < 1) return 'now';
        if (minutes < 60) return `${minutes}m`;
        if (hours < 24) return `${hours}h`;
        return `${Math.floor(hours / 24)}d`;
    }

    function getAffiliateUrl(domain) {
        const namecheapUrl = `https://www.namecheap.com/domains/registration/results/?domain=${encodeURIComponent(domain)}`;
        return `https://namecheap.pxf.io/c/6878241/1632743/5618?u=${encodeURIComponent(namecheapUrl)}`;
    }

    function showError(message) {
        resultsEl.innerHTML = `<p class="error-message">${escapeHtml(message)}</p>`;
        welcomeContent.hidden = true;
        resultsEl.hidden = false;
    }

    function escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    function shakeElement(el) {
        el.style.animation = 'none';
        el.offsetHeight;
        el.style.animation = 'shake 0.4s ease';
        el.focus();
    }

    // Add shake animation
    const style = document.createElement('style');
    style.textContent = `@keyframes shake {
        0%, 100% { transform: translateX(0); }
        20%, 60% { transform: translateX(-4px); }
        40%, 80% { transform: translateX(4px); }
    }`;
    document.head.appendChild(style);

    // =========================================================================
    // Welcome Content & Onboarding Tour
    // =========================================================================

    function showWelcomeContent() {
        resultsEl.hidden = true;
        favoritesView.hidden = true;
        historyView.hidden = true;
        welcomeContent.hidden = false;
        // Hide registration view if showing
        const regView = document.getElementById('registration-view');
        if (regView) regView.hidden = true;
    }

    // Get Started button - start onboarding tour
    if (getStartedBtn) {
        getStartedBtn.addEventListener('click', startTour);
    }

    let currentTourStep = 1;
    const tourTargets = [
        { el: () => document.getElementById('description'), padding: 8 },
        { el: () => document.querySelector('.search-options'), padding: 8 },
        { el: () => document.getElementById('submit-btn'), padding: 8 }
    ];

    function startTour() {
        currentTourStep = 1;
        tourOverlay.hidden = false;
        showTourStep(currentTourStep);
    }

    function showTourStep(step) {
        // Hide all steps
        tourTooltip.querySelectorAll('.tour-step').forEach(s => s.hidden = true);
        // Show current step
        const stepEl = tourTooltip.querySelector(`[data-step="${step}"]`);
        if (stepEl) stepEl.hidden = false;

        // Position spotlight and tooltip
        const target = tourTargets[step - 1];
        if (target && target.el()) {
            const el = target.el();
            const rect = el.getBoundingClientRect();
            const padding = target.padding || 4;

            // Position spotlight
            tourSpotlight.style.top = (rect.top - padding) + 'px';
            tourSpotlight.style.left = (rect.left - padding) + 'px';
            tourSpotlight.style.width = (rect.width + padding * 2) + 'px';
            tourSpotlight.style.height = (rect.height + padding * 2) + 'px';

            // Position tooltip below the spotlight
            tourTooltip.style.top = (rect.bottom + padding + 12) + 'px';
            tourTooltip.style.left = Math.max(16, Math.min(rect.left, window.innerWidth - 320)) + 'px';
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
        tourOverlay.hidden = true;
        // Focus the search input
        const descInput = document.getElementById('description');
        if (descInput) {
            descInput.focus();
        }
    }

    // Tour button handlers
    tourTooltip.querySelectorAll('.tour-next').forEach(btn => {
        btn.addEventListener('click', nextTourStep);
    });

    tourTooltip.querySelectorAll('.tour-done').forEach(btn => {
        btn.addEventListener('click', endTour);
    });

    // Close tour on backdrop click
    tourOverlay.querySelector('.tour-backdrop').addEventListener('click', endTour);

    // Fetch public stats on page load
    async function fetchStats() {
        try {
            const response = await fetch('/api/stats');
            if (response.ok) {
                const data = await response.json();
                if (statDomainsEl && data.domainsFound) {
                    // Animate the counter
                    animateCounter(statDomainsEl, data.domainsFound);
                }
            }
        } catch (err) {
            console.error('Failed to fetch stats:', err);
        }
    }

    function animateCounter(el, target) {
        const duration = 1000;
        const start = 0;
        const startTime = performance.now();

        function update(currentTime) {
            const elapsed = currentTime - startTime;
            const progress = Math.min(elapsed / duration, 1);
            // Ease out
            const eased = 1 - Math.pow(1 - progress, 3);
            const current = Math.round(start + (target - start) * eased);
            el.textContent = current.toLocaleString();

            if (progress < 1) {
                requestAnimationFrame(update);
            }
        }

        requestAnimationFrame(update);
    }

    // Initialize stats on page load
    fetchStats();

    // =========================================================================
    // Registration View
    // =========================================================================

    const REGISTRARS = {
        namecheap: {
            id: 'namecheap',
            name: 'Namecheap',
            affiliateUrl: 'https://namecheap.pxf.io/c/6878241/1632743/5618',
            getUrl: (d) => `https://www.namecheap.com/domains/registration/results/?domain=${encodeURIComponent(d)}`,
            tagline: 'Best value',
            price: '~$8.88/yr'
        },
        godaddy: {
            id: 'godaddy',
            name: 'GoDaddy',
            affiliateUrl: null, // Will be added when available
            getUrl: (d) => `https://www.godaddy.com/domainsearch/find?domainToCheck=${encodeURIComponent(d)}`,
            tagline: 'Most popular',
            price: '~$12.99/yr'
        },
        porkbun: {
            id: 'porkbun',
            name: 'Porkbun',
            affiliateUrl: null, // Will be added when available
            getUrl: (d) => `https://porkbun.com/checkout/search?q=${encodeURIComponent(d)}`,
            tagline: 'Great prices',
            price: '~$9.73/yr'
        }
    };

    const TLD_INFO = {
        'com': 'Most recognized TLD',
        'io': 'Tech & startup favorite',
        'co': 'Company & commercial',
        'net': 'Network & infrastructure',
        'org': 'Organizations & nonprofits',
        'ai': 'AI & machine learning',
        'app': 'Mobile & web apps',
        'dev': 'Developer tools',
        'me': 'Personal branding',
        'xyz': 'Modern & creative',
        'tech': 'Technology focused',
        'site': 'General websites',
        'online': 'Online presence'
    };

    // Registration view elements
    const registrationView = document.getElementById('registration-view');
    const registrationBack = document.getElementById('registration-back');
    const registrationDomain = document.getElementById('registration-domain');
    const registrationStats = document.getElementById('registration-stats');
    const registrarCards = document.getElementById('registrar-cards');
    const registrationPreference = document.getElementById('registration-preference');
    const rememberRegistrar = document.getElementById('remember-registrar');
    const loginForPreference = document.getElementById('login-for-preference');
    const loginForPrefLink = document.getElementById('login-for-pref-link');

    let currentRegistrationDomain = null;
    let userPreferredRegistrar = null;

    // Load user's preferences from server when logged in
    async function loadUserPreferences() {
        if (!authToken) return;
        try {
            const response = await apiFetch('/api/user/preferences');
            if (response.ok) {
                const data = await response.json();
                userPreferredRegistrar = data.preferredRegistrar || null;
                // Apply user's saved theme preference
                if (data.theme) {
                    applyUserTheme(data.theme);
                }
            }
        } catch (err) {
            console.error('Failed to load preferences:', err);
        }
    }

    // Save user's preferred registrar
    async function savePreferredRegistrar(registrarId) {
        if (!authToken) return;
        try {
            await apiFetch('/api/user/preferences', {
                method: 'PUT',
                body: JSON.stringify({ preferredRegistrar: registrarId })
            });
            userPreferredRegistrar = registrarId;
        } catch (err) {
            console.error('Failed to save preference:', err);
        }
    }

    function showRegistrationView(domain) {
        currentRegistrationDomain = domain;

        // Hide other views
        welcomeContent.hidden = true;
        resultsEl.hidden = true;
        favoritesView.hidden = true;
        historyView.hidden = true;

        // Show registration view
        registrationView.hidden = false;

        // Set domain name
        registrationDomain.textContent = domain;

        // Compute and display stats
        const stats = getDomainStats(domain);
        registrationStats.innerHTML = `
            <span class="stat-item">${stats.length} characters</span>
            <span class="stat-separator">&middot;</span>
            <span class="stat-item">.${stats.tld} TLD</span>
            <span class="stat-separator">&middot;</span>
            <span class="stat-item">${stats.tldInfo}</span>
            ${stats.memorabilityLabel ? `<span class="stat-separator">&middot;</span><span class="stat-item">${stats.memorabilityLabel}</span>` : ''}
        `;

        // Render registrar cards
        renderRegistrarCards(domain);

        // Show/hide preference option based on login state
        if (currentUser) {
            registrationPreference.hidden = false;
            loginForPreference.hidden = true;
            rememberRegistrar.checked = false;
        } else {
            registrationPreference.hidden = true;
            loginForPreference.hidden = false;
        }
    }

    function hideRegistrationView() {
        registrationView.hidden = true;
        currentRegistrationDomain = null;

        // Show results if we have them, otherwise welcome
        const activeTab = getActiveTab();
        if (activeTab && Object.keys(activeTab.categories).length > 0) {
            switchToTab(activeTab.id);
        } else {
            welcomeContent.hidden = false;
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
        const registrarOrder = ['namecheap', 'godaddy', 'porkbun'];

        const mainCards = registrarOrder.map((id, index) => {
            const registrar = REGISTRARS[id];
            const isPreferred = userPreferredRegistrar === id;
            const url = getRegistrarUrl(domain, registrar);

            return `
                <div class="registrar-card${isPreferred ? ' preferred' : ''}" style="animation-delay: ${index * 0.05}s">
                    <div class="registrar-info">
                        <div class="registrar-name">${registrar.name}</div>
                        <div class="registrar-tagline">${registrar.tagline}</div>
                    </div>
                    <div class="registrar-right">
                        <div class="registrar-price">${registrar.price}</div>
                        <a href="${url}" target="_blank" rel="noopener" class="registrar-btn" data-registrar="${id}" data-domain="${escapeHtml(domain)}">
                            Register &rarr;
                        </a>
                    </div>
                    ${isPreferred ? '<span class="preferred-badge">Preferred</span>' : ''}
                </div>
            `;
        }).join('');

        // Add "Other" card
        const otherCard = `
            <div class="registrar-card registrar-card-other" style="animation-delay: ${registrarOrder.length * 0.05}s" data-domain="${escapeHtml(domain)}">
                <div class="other-collapsed">
                    <div class="registrar-info">
                        <div class="registrar-name">Other</div>
                        <div class="registrar-tagline">Use a different registrar</div>
                    </div>
                    <div class="registrar-right">
                        <button type="button" class="registrar-btn other-expand-btn">
                            Choose &rarr;
                        </button>
                    </div>
                </div>
                <div class="other-expanded" hidden>
                    <div class="other-input-row">
                        <label class="other-label">Which registrar do you prefer?</label>
                        <input type="text" class="other-input" placeholder="e.g., Cloudflare, Hover, Google..." autocomplete="off">
                    </div>
                    <div class="other-actions">
                        <button type="button" class="other-cancel-btn">Cancel</button>
                        <button type="button" class="registrar-btn other-copy-btn" disabled>Copy domain</button>
                    </div>
                </div>
            </div>
        `;

        registrarCards.innerHTML = mainCards + otherCard;

        // Add click handlers for tracking and preference saving
        registrarCards.querySelectorAll('.registrar-btn:not(.other-expand-btn):not(.other-copy-btn)').forEach(btn => {
            btn.addEventListener('click', async (e) => {
                const registrarId = btn.dataset.registrar;
                const domain = btn.dataset.domain;

                // Track click
                fetch('/api/track/affiliate', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ domain, registrar: registrarId })
                }).catch(() => {});

                // Save preference if checkbox is checked
                if (rememberRegistrar && rememberRegistrar.checked && currentUser) {
                    await savePreferredRegistrar(registrarId);
                }
            });
        });

        // "Other" card expand/collapse handlers
        const otherCardEl = registrarCards.querySelector('.registrar-card-other');
        const collapsedEl = otherCardEl.querySelector('.other-collapsed');
        const expandedEl = otherCardEl.querySelector('.other-expanded');
        const expandBtn = otherCardEl.querySelector('.other-expand-btn');
        const cancelBtn = otherCardEl.querySelector('.other-cancel-btn');
        const copyBtn = otherCardEl.querySelector('.other-copy-btn');
        const input = otherCardEl.querySelector('.other-input');

        expandBtn.addEventListener('click', () => {
            collapsedEl.hidden = true;
            expandedEl.hidden = false;
            otherCardEl.classList.add('expanded');
            input.focus();
        });

        cancelBtn.addEventListener('click', () => {
            collapsedEl.hidden = false;
            expandedEl.hidden = true;
            otherCardEl.classList.remove('expanded');
            input.value = '';
            copyBtn.disabled = true;
        });

        input.addEventListener('input', () => {
            copyBtn.disabled = input.value.trim().length === 0;
        });

        // Allow Enter key to trigger copy
        input.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' && input.value.trim()) {
                copyBtn.click();
            }
        });

        copyBtn.addEventListener('click', async () => {
            const registrarName = input.value.trim();
            const domainToCopy = otherCardEl.dataset.domain;

            if (!registrarName) return;

            // Track the "other" registrar choice
            fetch('/api/track/affiliate', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ domain: domainToCopy, registrar: 'other', otherRegistrar: registrarName })
            }).catch(() => {});

            // Copy domain to clipboard
            try {
                await navigator.clipboard.writeText(domainToCopy);
                copyBtn.textContent = 'Copied!';
                copyBtn.classList.add('copied');
                setTimeout(() => {
                    copyBtn.textContent = 'Copy domain';
                    copyBtn.classList.remove('copied');
                }, 2000);
            } catch (err) {
                // Fallback for older browsers
                const textArea = document.createElement('textarea');
                textArea.value = domainToCopy;
                textArea.style.position = 'fixed';
                textArea.style.opacity = '0';
                document.body.appendChild(textArea);
                textArea.select();
                document.execCommand('copy');
                document.body.removeChild(textArea);
                copyBtn.textContent = 'Copied!';
                copyBtn.classList.add('copied');
                setTimeout(() => {
                    copyBtn.textContent = 'Copy domain';
                    copyBtn.classList.remove('copied');
                }, 2000);
            }
        });
    }

    // Back button handler
    registrationBack.addEventListener('click', hideRegistrationView);

    // Login link in preference area
    loginForPrefLink.addEventListener('click', (e) => {
        e.preventDefault();
        openLoginModal();
    });

    // Load preferences when auth state changes
    // We hook into the existing fetchCurrentUser by adding our own call after user is fetched
    const originalUpdateAuthUI = updateAuthUI;
    updateAuthUI = function() {
        originalUpdateAuthUI();
        if (currentUser) {
            loadUserPreferences();
        } else {
            userPreferredRegistrar = null;
        }
    };

    // Make showRegistrationView available globally
    window.showRegistrationView = showRegistrationView;
});
