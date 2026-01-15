document.addEventListener('DOMContentLoaded', () => {
    const form = document.getElementById('generate-form');
    const submitBtn = document.getElementById('submit-btn');
    const btnText = submitBtn.querySelector('.btn-text');
    const btnLoading = submitBtn.querySelector('.btn-loading');
    const resultsSection = document.getElementById('results');
    const tldTabs = document.getElementById('tld-tabs');
    const domainList = document.getElementById('domain-list');
    const tldStyleInput = document.getElementById('tld-style');

    // Add shake animation for validation
    const style = document.createElement('style');
    style.textContent = `
        @keyframes shake {
            0%, 100% { transform: translateX(0); }
            10%, 30%, 50%, 70%, 90% { transform: translateX(-4px); }
            20%, 40%, 60%, 80% { transform: translateX(4px); }
        }
    `;
    document.head.appendChild(style);

    // Style selector buttons
    document.querySelectorAll('.style-option').forEach(btn => {
        btn.addEventListener('click', () => {
            document.querySelectorAll('.style-option').forEach(b => b.classList.remove('active'));
            btn.classList.add('active');
            tldStyleInput.value = btn.dataset.value;
        });
    });

    // Store domains by TLD
    let domainsByTld = {};
    let currentTld = null;

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
        resultsSection.hidden = true;
        domainsByTld = {};
        currentTld = null;

        try {
            const response = await fetch('/api/generate', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    description,
                    tldStyle,
                }),
            });

            const data = await response.json();

            if (data.error) {
                showError(data.error);
                return;
            }

            // Group domains by TLD
            data.domains.forEach(d => {
                const tld = getTld(d.name);
                if (!domainsByTld[tld]) {
                    domainsByTld[tld] = [];
                }
                domainsByTld[tld].push({ ...d, checking: true });
            });

            renderTabs();
            renderDomains();

            // Smooth reveal of results
            resultsSection.hidden = false;
            resultsSection.style.animation = 'none';
            resultsSection.offsetHeight; // Trigger reflow
            resultsSection.style.animation = 'slideUp 0.6s cubic-bezier(0.16, 1, 0.3, 1) both';

            // Check availability
            checkAvailability(data.domains.map(d => d.name));
        } catch (err) {
            showError('Failed to generate domains. Please try again.');
            console.error(err);
        } finally {
            submitBtn.disabled = false;
            btnText.hidden = false;
            btnLoading.hidden = true;
        }
    });

    function getTld(domain) {
        const parts = domain.split('.');
        return '.' + parts[parts.length - 1];
    }

    function renderTabs() {
        // Sort TLDs with .com first
        const tlds = Object.keys(domainsByTld).sort((a, b) => {
            if (a === '.com') return -1;
            if (b === '.com') return 1;
            return a.localeCompare(b);
        });

        if (!currentTld || !domainsByTld[currentTld]) {
            currentTld = tlds[0];
        }

        tldTabs.innerHTML = tlds.map(tld => {
            const count = domainsByTld[tld].length;
            const isActive = tld === currentTld ? 'active' : '';
            return `<button class="tld-tab ${isActive}" data-tld="${tld}">${tld}<span class="count">(${count})</span></button>`;
        }).join('');

        // Add click handlers
        tldTabs.querySelectorAll('.tld-tab').forEach(tab => {
            tab.addEventListener('click', () => {
                currentTld = tab.dataset.tld;
                renderTabs();
                renderDomains();
            });
        });
    }

    function renderDomains() {
        const domains = domainsByTld[currentTld] || [];

        if (domains.length === 0) {
            domainList.innerHTML = '<p class="no-results">No available domains found for this TLD</p>';
            return;
        }

        domainList.innerHTML = domains.map((domain, index) => {
            let actionsHtml;
            if (domain.checking) {
                actionsHtml = '<span class="status-badge checking">Checking...</span>';
            } else if (domain.available) {
                const priceTag = domain.price ? `<span class="price-tag">$${domain.price.toFixed(2)}/yr</span>` : '';
                const premiumTag = domain.isPremium ? '<span class="premium-tag">Premium</span>' : '';
                actionsHtml = `
                    <span class="status-badge available">Available</span>
                    ${premiumTag}
                    ${priceTag}
                    <a href="${getAffiliateUrl(domain.name)}" target="_blank" rel="noopener" class="buy-link">Register &rarr;</a>
                `;
            } else {
                return ''; // Don't render taken domains
            }

            return `
                <div class="domain-item" style="animation-delay: ${0.05 + index * 0.05}s">
                    <span class="domain-name">${escapeHtml(domain.name)}</span>
                    <div class="domain-actions">${actionsHtml}</div>
                </div>
            `;
        }).join('');
    }

    async function checkAvailability(domains) {
        try {
            const response = await fetch('/api/check', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ domains }),
            });

            const data = await response.json();

            if (data.error) {
                console.warn('Availability check failed:', data.error);
                // Mark all as needing manual check
                Object.keys(domainsByTld).forEach(tld => {
                    domainsByTld[tld].forEach(d => {
                        d.checking = false;
                        d.available = true; // Assume available, let user verify
                    });
                });
                renderTabs();
                renderDomains();
                return;
            }

            // Update domain status
            data.results.forEach(result => {
                const tld = getTld(result.name);
                if (domainsByTld[tld]) {
                    const domain = domainsByTld[tld].find(d => d.name === result.name);
                    if (domain) {
                        domain.checking = false;
                        domain.available = result.available;
                        domain.isPremium = result.isPremium;
                        domain.price = result.price;
                    }
                }
            });

            // Remove taken domains and update counts
            Object.keys(domainsByTld).forEach(tld => {
                domainsByTld[tld] = domainsByTld[tld].filter(d => d.checking || d.available);
            });

            // Remove empty TLDs
            Object.keys(domainsByTld).forEach(tld => {
                if (domainsByTld[tld].length === 0) {
                    delete domainsByTld[tld];
                }
            });

            // Update current TLD if it was removed
            if (!domainsByTld[currentTld]) {
                const tlds = Object.keys(domainsByTld).sort((a, b) => {
                    if (a === '.com') return -1;
                    if (b === '.com') return 1;
                    return a.localeCompare(b);
                });
                currentTld = tlds[0] || null;
            }

            renderTabs();
            renderDomains();
        } catch (err) {
            console.error('Availability check error:', err);
        }
    }

    function getAffiliateUrl(domain) {
        return `https://www.namecheap.com/domains/registration/results/?domain=${encodeURIComponent(domain)}`;
    }

    function showError(message) {
        domainList.innerHTML = `<p class="error-message">${escapeHtml(message)}</p>`;
        tldTabs.innerHTML = '';
        resultsSection.hidden = false;
    }

    function escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    function shakeElement(element) {
        element.style.animation = 'none';
        element.offsetHeight; // Trigger reflow
        element.style.animation = 'shake 0.5s ease-out';
        element.focus();
        element.addEventListener('animationend', () => {
            element.style.animation = '';
        }, { once: true });
    }
});
