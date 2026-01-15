document.addEventListener('DOMContentLoaded', () => {
    const form = document.getElementById('generate-form');
    const submitBtn = document.getElementById('submit-btn');
    const btnText = submitBtn.querySelector('.btn-text');
    const btnLoading = submitBtn.querySelector('.btn-loading');
    const resultsEl = document.getElementById('results');
    const emptyState = document.getElementById('empty-state');
    const tldStyleInput = document.getElementById('tld-style');

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

        try {
            const response = await fetch('/api/generate', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ description, tldStyle }),
            });

            const data = await response.json();

            if (data.error) {
                showError(data.error);
                return;
            }

            // Store categorized domains
            categories = data.categories || {};

            // Mark all as checking
            Object.keys(categories).forEach(cat => {
                categories[cat] = categories[cat].map(d => ({
                    ...d,
                    checking: true
                }));
            });

            renderResults();
            emptyState.hidden = true;
            resultsEl.hidden = false;

            // Collect all domains for availability check
            const allDomains = Object.values(categories).flat().map(d => d.name);
            checkAvailability(allDomains);

        } catch (err) {
            showError('Failed to generate domains. Please try again.');
            console.error(err);
        } finally {
            submitBtn.disabled = false;
            btnText.hidden = false;
            btnLoading.hidden = true;
        }
    });

    function renderResults() {
        const categoryOrder = ['Professional', 'Playful', 'Techy', 'Minimal'];

        resultsEl.innerHTML = categoryOrder
            .filter(cat => categories[cat] && categories[cat].length > 0)
            .map(cat => {
                const domains = categories[cat].filter(d => d.checking || d.available);
                if (domains.length === 0) return '';

                return `
                    <section class="category">
                        <div class="category-header">
                            <h2 class="category-title">${cat}</h2>
                            <div class="category-line"></div>
                        </div>
                        <div class="domain-grid">
                            ${domains.map((d, i) => renderDomainCard(d, i)).join('')}
                        </div>
                    </section>
                `;
            }).join('');

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

        if (domain.checking) {
            metaHtml = '<span class="domain-status checking">Checking</span>';
        } else if (domain.available) {
            const statusClass = domain.unverified ? 'unverified' : 'available';
            const statusText = domain.unverified ? 'Verify' : 'Available';

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
        }

        const linkHtml = domain.available && !domain.checking
            ? `<a href="${getAffiliateUrl(domain.name)}" target="_blank" rel="noopener" class="domain-link">Register →</a>`
            : '';

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

    async function checkAvailability(domains) {
        try {
            const response = await fetch('/api/check', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ domains }),
            });

            const data = await response.json();

            if (data.error) {
                console.warn('Availability check failed:', data.error);
                // Mark all as unverified
                Object.keys(categories).forEach(cat => {
                    categories[cat].forEach(d => {
                        d.checking = false;
                        d.available = true;
                        d.unverified = true;
                    });
                });
                renderResults();
                return;
            }

            // Update domain status
            const resultMap = {};
            data.results.forEach(r => {
                resultMap[r.name.toLowerCase()] = r;
            });

            Object.keys(categories).forEach(cat => {
                categories[cat].forEach(d => {
                    const result = resultMap[d.name.toLowerCase()];
                    if (result) {
                        d.checking = false;
                        d.available = result.available !== null ? result.available : true;
                        d.isPremium = result.isPremium;
                        d.price = result.price;
                        d.unverified = result.available === null;
                        d.fromCache = result.fromCache;
                        d.checkedAt = result.checkedAt;
                    }
                });
                // Filter out taken domains
                categories[cat] = categories[cat].filter(d => d.checking || d.available);
            });

            renderResults();
        } catch (err) {
            console.error('Availability check error:', err);
        }
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
        renderResults();

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
                        domain.unverified = result.available === null;
                        domain.fromCache = false;
                        domain.checkedAt = result.checkedAt;

                        if (!domain.available) {
                            categories[cat] = categories[cat].filter(d => d.name !== domainName);
                        }
                    }
                });
            }
            renderResults();
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
