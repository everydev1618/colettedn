// Lambda Function URL for generate endpoint (no timeout limit)
// Falls back to relative path if not set
const FUNCTION_URL = 'https://4tpzgbt5zo7kade7egg5uu75jy0inpuj.lambda-url.us-east-1.on.aws/';

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

    // Maintenance mode handling
    let maintenanceTimer = null;

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

            // Also check for 429 (rate limited) message that might indicate maintenance
            if (response.status === 429 || data.error) {
                if (response.status === 429) {
                    showError(data.error || 'Too many requests. Please wait a moment.');
                } else {
                    showError(data.error);
                }
                return;
            }

            // Results come back with availability already checked
            categories = data.categories || {};
            const rounds = data.rounds || 1;

            renderResults(rounds);

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
        const categoryOrder = ['Professional', 'Playful', 'Techy', 'Minimal'];
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

        const linkHtml = `<a href="${getAffiliateUrl(domain.name)}" target="_blank" rel="noopener" class="domain-link">Register →</a>`;

        return `
            <div class="domain-card" style="animation-delay: ${index * 0.03}s">
                <span class="domain-name">${escapeHtml(domain.name)}</span>
                <div class="domain-row">
                    <div class="domain-meta">${metaHtml}</div>
                    ${linkHtml}
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
        return `https://www.namecheap.com/domains/registration/results/?domain=${encodeURIComponent(domain)}`;
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
