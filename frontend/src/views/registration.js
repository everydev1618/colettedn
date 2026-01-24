import { dom } from '../dom.js';
import { authToken, currentUser, setCurrentRegistrationDomain, setUserPreferredRegistrar, userPreferredRegistrar } from '../state.js';
import { apiFetch } from '../api.js';
import { escapeHtml } from '../utils.js';
import { REGISTRARS, TLD_INFO } from '../config.js';
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
            // Apply user's saved theme preference
            if (data.theme) {
                applyUserTheme(data.theme);
            }
        }
    } catch (err) {
        console.error('Failed to load preferences:', err);
    }
}

async function savePreferredRegistrar(registrarId) {
    if (!authToken) return;
    try {
        await apiFetch('/api/user/preferences', {
            method: 'PUT',
            body: JSON.stringify({ preferredRegistrar: registrarId })
        });
        setUserPreferredRegistrar(registrarId);
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

    // Show/hide preference option based on login state
    if (currentUser) {
        dom.registrationPreference.hidden = false;
        dom.loginForPreference.hidden = true;
        dom.rememberRegistrar.checked = false;
    } else {
        dom.registrationPreference.hidden = true;
        dom.loginForPreference.hidden = false;
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

    dom.registrarCards.innerHTML = mainCards + otherCard;

    // Add click handlers for tracking and preference saving
    dom.registrarCards.querySelectorAll('.registrar-btn:not(.other-expand-btn):not(.other-copy-btn)').forEach(btn => {
        btn.addEventListener('click', async (e) => {
            const registrarId = btn.dataset.registrar;
            const clickedDomain = btn.dataset.domain;

            // Track click
            fetch('/api/track/affiliate', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ domain: clickedDomain, registrar: registrarId })
            }).catch(() => {});

            // Save preference if checkbox is checked
            if (dom.rememberRegistrar && dom.rememberRegistrar.checked && currentUser) {
                await savePreferredRegistrar(registrarId);
            }
        });
    });

    // "Other" card expand/collapse handlers
    const otherCardEl = dom.registrarCards.querySelector('.registrar-card-other');
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

export function initRegistration() {
    // Back button handler
    dom.registrationBack.addEventListener('click', hideRegistrationView);

    // Login link in preference area
    dom.loginForPrefLink.addEventListener('click', (e) => {
        e.preventDefault();
        openLoginModal();
    });
}

// Make showRegistrationView available globally for legacy code
window.showRegistrationView = showRegistrationView;
