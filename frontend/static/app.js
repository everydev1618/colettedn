// Lambda Function URL for generate endpoint (no timeout limit)
// Falls back to relative path if not set
const FUNCTION_URL = 'https://4tpzgbt5zo7kade7egg5uu75jy0inpuj.lambda-url.us-east-1.on.aws/';

// Auth state
let authToken = localStorage.getItem('authToken');
let currentUser = null;
let userFavorites = new Set();

document.addEventListener('DOMContentLoaded', () => {
    const form = document.getElementById('generate-form');
    const submitBtn = document.getElementById('submit-btn');
    const btnText = submitBtn.querySelector('.btn-text');
    const btnLoading = submitBtn.querySelector('.btn-loading');
    const resultsEl = document.getElementById('results');
    const emptyState = document.getElementById('empty-state');
    const tldStyleInput = document.getElementById('tld-style');
    const maintenanceOverlay = document.getElementById('maintenance-overlay');
    const maintenanceCountdown = document.getElementById('maintenance-countdown');

    // Hero state elements
    const heroState = document.getElementById('hero-state');
    const heroForm = document.getElementById('hero-form');
    const heroDescription = document.getElementById('hero-description');
    const heroSubmitBtn = heroForm.querySelector('.hero-submit');
    const heroBtnText = heroSubmitBtn.querySelector('.hero-btn-text');
    const heroBtnLoading = heroSubmitBtn.querySelector('.hero-btn-loading');
    const appLayout = document.querySelector('.app-layout');

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
    const favoritesBtn = document.getElementById('favorites-btn');
    const logoutBtn = document.getElementById('logout-btn');

    // Favorites view elements
    const favoritesView = document.getElementById('favorites-view');
    const favoritesList = document.getElementById('favorites-list');
    const favoritesClose = document.getElementById('favorites-close');

    // Hero auth elements
    const heroSignInBtn = document.getElementById('hero-sign-in-btn');
    const heroUserDropdown = document.getElementById('hero-user-dropdown');
    const heroUserBtn = document.getElementById('hero-user-btn');
    const heroUserEmail = document.getElementById('hero-user-email');
    const heroDropdownMenu = document.getElementById('hero-dropdown-menu');
    const heroFavoritesBtn = document.getElementById('hero-favorites-btn');
    const heroHistoryBtn = document.getElementById('hero-history-btn');
    const heroLogoutBtn = document.getElementById('hero-logout-btn');

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
    const heroUpgradeBtn = document.getElementById('hero-upgrade-btn');
    const heroManageBtn = document.getElementById('hero-manage-btn');

    // Admin buttons
    const adminBtn = document.getElementById('admin-btn');
    const heroAdminBtn = document.getElementById('hero-admin-btn');
    const ADMIN_EMAIL = 'etdebruin@gmail.com';

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

                // Always go to app layout when logged in
                heroState.hidden = true;
                appLayout.hidden = false;
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

    function updateAuthUI() {
        const isPro = currentUser && currentUser.subscriptionTier === 'pro';

        if (currentUser) {
            // Header (app layout)
            signInBtn.hidden = true;
            userDropdown.hidden = false;
            userEmailEl.textContent = currentUser.email;
            userEmailEl.innerHTML = isPro
                ? `${escapeHtml(currentUser.email)} <span class="pro-badge">Pro</span>`
                : escapeHtml(currentUser.email);

            // Hero (landing page)
            heroSignInBtn.hidden = true;
            heroUserDropdown.hidden = false;
            heroUserEmail.innerHTML = isPro
                ? `${escapeHtml(currentUser.email)} <span class="pro-badge">Pro</span>`
                : escapeHtml(currentUser.email);

            // Show upgrade or manage button based on tier
            upgradeMenuBtn.hidden = isPro;
            manageBtn.hidden = !isPro;
            heroUpgradeBtn.hidden = isPro;
            heroManageBtn.hidden = !isPro;

            // Show admin button only for admin email
            const isAdmin = currentUser.email === ADMIN_EMAIL;
            adminBtn.hidden = !isAdmin;
            heroAdminBtn.hidden = !isAdmin;
        } else {
            // Header (app layout)
            signInBtn.hidden = false;
            userDropdown.hidden = true;
            // Hero (landing page)
            heroSignInBtn.hidden = false;
            heroUserDropdown.hidden = true;
        }
    }

    function logout() {
        authToken = null;
        currentUser = null;
        userFavorites.clear();
        localStorage.removeItem('authToken');
        updateAuthUI();
        // POST to logout endpoint (fire and forget)
        fetch('/api/auth/logout', { method: 'POST', headers: getAuthHeaders() }).catch(() => {});
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
    heroSignInBtn.addEventListener('click', openLoginModal);

    // Hero dropdown handlers
    heroUserBtn.addEventListener('click', () => {
        const isOpen = !heroDropdownMenu.hidden;
        heroDropdownMenu.hidden = isOpen;
        heroUserDropdown.classList.toggle('open', !isOpen);
    });

    heroFavoritesBtn.addEventListener('click', () => {
        heroDropdownMenu.hidden = true;
        heroUserDropdown.classList.remove('open');
        // Transition to app layout and show favorites
        heroState.hidden = true;
        appLayout.hidden = false;
        showFavoritesView();
    });

    heroHistoryBtn.addEventListener('click', () => {
        heroDropdownMenu.hidden = true;
        heroUserDropdown.classList.remove('open');
        // Transition to app layout and show history
        heroState.hidden = true;
        appLayout.hidden = false;
        showHistoryView();
    });

    heroLogoutBtn.addEventListener('click', () => {
        heroDropdownMenu.hidden = true;
        heroUserDropdown.classList.remove('open');
        logout();
    });

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

    // Upgrade button in dropdowns
    upgradeMenuBtn.addEventListener('click', () => {
        dropdownMenu.hidden = true;
        userDropdown.classList.remove('open');
        openUpgradeModal();
    });

    heroUpgradeBtn.addEventListener('click', () => {
        heroDropdownMenu.hidden = true;
        heroUserDropdown.classList.remove('open');
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

    heroManageBtn.addEventListener('click', () => {
        heroDropdownMenu.hidden = true;
        heroUserDropdown.classList.remove('open');
        openManageSubscription();
    });

    // Admin button handlers
    adminBtn.addEventListener('click', () => {
        dropdownMenu.hidden = true;
        userDropdown.classList.remove('open');
        window.location.href = '/admin';
    });

    heroAdminBtn.addEventListener('click', () => {
        heroDropdownMenu.hidden = true;
        heroUserDropdown.classList.remove('open');
        window.location.href = '/admin';
    });

    // User dropdown handlers
    userBtn.addEventListener('click', () => {
        const isOpen = !dropdownMenu.hidden;
        dropdownMenu.hidden = isOpen;
        userDropdown.classList.toggle('open', !isOpen);
    });

    // Close dropdowns when clicking outside
    document.addEventListener('click', (e) => {
        if (!userDropdown.contains(e.target)) {
            dropdownMenu.hidden = true;
            userDropdown.classList.remove('open');
        }
        if (!heroUserDropdown.contains(e.target)) {
            heroDropdownMenu.hidden = true;
            heroUserDropdown.classList.remove('open');
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
        emptyState.hidden = true;
        resultsEl.hidden = true;
        historyView.hidden = true;
        favoritesView.hidden = false;
        await renderFavoritesView();
    }

    function hideFavoritesView() {
        favoritesView.hidden = true;
        historyView.hidden = true;
        if (Object.keys(categories).length > 0) {
            resultsEl.hidden = false;
        } else {
            emptyState.hidden = false;
        }
    }

    favoritesClose.addEventListener('click', hideFavoritesView);

    // History view handlers
    async function showHistoryView() {
        emptyState.hidden = true;
        resultsEl.hidden = true;
        favoritesView.hidden = true;
        historyView.hidden = false;
        await renderHistoryView();
    }

    function hideHistoryView() {
        historyView.hidden = true;
        favoritesView.hidden = true;
        if (Object.keys(categories).length > 0) {
            resultsEl.hidden = false;
        } else {
            emptyState.hidden = false;
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

    // Save search to history
    async function saveSearchHistory(description, tldStyle, categoriesData) {
        if (!authToken) return;

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
        favoritesList.innerHTML = favArray.map((domain, i) => `
            <div class="domain-card" style="animation-delay: ${i * 0.03}s">
                <span class="domain-name">${escapeHtml(domain)}</span>
                <div class="domain-row">
                    <button class="favorite-btn favorited" data-domain="${escapeHtml(domain)}" title="Remove from favorites">
                        ♥
                    </button>
                    <a href="${getAffiliateUrl(domain)}" target="_blank" rel="noopener" class="domain-link">Register &rarr;</a>
                </div>
            </div>
        `).join('');

        // Add remove handlers
        favoritesList.querySelectorAll('.favorite-btn').forEach(btn => {
            btn.addEventListener('click', async () => {
                await toggleFavorite(btn.dataset.domain);
                await renderFavoritesView();
            });
        });
    }

    async function toggleFavorite(domain) {
        if (!authToken) {
            openLoginModal();
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

    // Track current TLD style for hero form
    let heroTldStyle = 'traditional';

    // Logo click - return to landing page
    document.getElementById('logo-home').addEventListener('click', (e) => {
        e.preventDefault();
        appLayout.hidden = true;
        heroState.hidden = false;
        favoritesView.hidden = true;
    });

    // Hero TLD toggle handlers
    document.querySelectorAll('.hero-tld-toggle').forEach(btn => {
        btn.addEventListener('click', () => {
            document.querySelectorAll('.hero-tld-toggle').forEach(b => b.classList.remove('active'));
            btn.classList.add('active');
            heroTldStyle = btn.dataset.value;
        });
    });

    // Hero form submission - transition to results view
    heroForm.addEventListener('submit', async (e) => {
        e.preventDefault();

        const description = heroDescription.value.trim();
        if (!description) {
            shakeElement(heroDescription);
            return;
        }

        // Sync values to main form
        document.getElementById('description').value = description;
        tldStyleInput.value = heroTldStyle;
        // Sync the TLD toggle UI in the header
        document.querySelectorAll('.tld-toggle').forEach(b => {
            b.classList.toggle('active', b.dataset.value === heroTldStyle);
        });

        // Transition to app layout
        heroState.hidden = true;
        appLayout.hidden = false;

        // Trigger the main form submit
        form.dispatchEvent(new Event('submit'));
    });

    // TLD toggle handlers
    document.querySelectorAll('.tld-toggle').forEach(btn => {
        btn.addEventListener('click', () => {
            document.querySelectorAll('.tld-toggle').forEach(b => b.classList.remove('active'));
            btn.classList.add('active');
            tldStyleInput.value = btn.dataset.value;
        });
    });

    // Store categorized results
    let categories = {};

    form.addEventListener('submit', async (e) => {
        e.preventDefault();

        const description = document.getElementById('description').value.trim();
        const tldStyle = tldStyleInput.value;

        if (!description) {
            shakeElement(document.getElementById('description'));
            return;
        }

        // Hide favorites/history view if showing
        favoritesView.hidden = true;
        historyView.hidden = true;

        // Show loading state
        submitBtn.disabled = true;
        btnText.hidden = true;
        btnLoading.hidden = false;
        categories = {};
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
        emptyState.hidden = true;
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
                showMaintenanceMode();
                return;
            }

            const data = await response.json();

            // Handle rate limiting - show upgrade modal if available
            if (response.status === 429) {
                if (data.upgradeAvailable && authToken) {
                    openUpgradeModal();
                } else if (data.upgradeAvailable && !authToken) {
                    // Show login first, then upgrade
                    openLoginModal();
                } else {
                    showError(data.error || 'Too many requests. Please wait a moment.');
                }
                return;
            }

            if (data.error) {
                showError(data.error);
                return;
            }

            // Results come back with availability already checked
            categories = data.categories || {};
            const rounds = data.rounds || 1;

            renderResults(rounds);

            // Save to history if logged in
            saveSearchHistory(description, tldStyle, categories);

        } catch (err) {
            showError('Failed to generate domains. Please try again.');
            console.error(err);
        } finally {
            submitBtn.disabled = false;
            btnText.hidden = false;
            btnLoading.hidden = true;
        }
    });

    function renderResults(rounds) {
        const categoryOrder = ['Professional', 'Playful', 'Creative', 'Minimal'];
        const totalDomains = Object.values(categories).flat().length;

        // Small inline badge for multi-round searches
        const roundsBadge = rounds > 1
            ? `<span class="rounds-badge">${rounds} rounds · ${totalDomains} found</span>`
            : '';

        const sectionsHtml = categoryOrder
            .map((cat, idx) => {
                const domains = categories[cat] || [];
                // Put the rounds badge after the first category title
                const badge = idx === 0 ? roundsBadge : '';
                const gridContent = domains.length > 0
                    ? domains.map((d, i) => renderDomainCard(d, i)).join('')
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

        resultsEl.innerHTML = sectionsHtml;

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
    }

    function renderDomainCard(domain, index) {
        let metaHtml = '';

        const statusClass = domain.available === false ? 'taken' : 'available';
        const statusText = domain.available === null ? 'Verify' : 'Available';

        metaHtml = `<span class="domain-status ${statusClass}">${statusText}</span>`;

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

        const isFavorited = userFavorites.has(domain.name.toLowerCase());
        const heartIcon = isFavorited ? '♥' : '♡';
        const heartClass = isFavorited ? 'favorited' : '';

        const linkHtml = `<a href="${getAffiliateUrl(domain.name)}" target="_blank" rel="noopener" class="domain-link">Register &rarr;</a>`;

        return `
            <div class="domain-card" style="animation-delay: ${index * 0.03}s">
                <span class="domain-name">${escapeHtml(domain.name)}</span>
                <div class="domain-row">
                    <div class="domain-meta">${metaHtml}</div>
                    <div class="domain-actions">
                        <button class="favorite-btn ${heartClass}" data-domain="${escapeHtml(domain.name)}" title="${isFavorited ? 'Remove from favorites' : 'Add to favorites'}">
                            ${heartIcon}
                        </button>
                        ${linkHtml}
                    </div>
                </div>
            </div>
        `;
    }

    async function refreshDomain(domainName) {
        // Find and mark as checking
        Object.keys(categories).forEach(cat => {
            const domain = categories[cat].find(d => d.name.toLowerCase() === domainName.toLowerCase());
            if (domain) {
                domain.checking = true;
                domain.fromCache = false;
            }
        });
        renderResults(1);

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
            renderResults(1);
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
        emptyState.hidden = true;
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
});
