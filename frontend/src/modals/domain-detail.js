import { dom } from '../dom.js';
import { rdapInfoCache } from '../state.js';
import { escapeHtml, formatDate, formatDomainAge } from '../utils.js';

let currentDomain = null;

export function openDomainDetailModal(domain) {
    currentDomain = domain;
    dom.domainDetailName.textContent = domain;
    dom.domainDetailModal.hidden = false;

    // Check cache first
    const cached = rdapInfoCache.get(domain.toLowerCase());
    if (cached) {
        renderDomainDetail(cached);
        return;
    }

    // Show loading state
    dom.domainDetailContent.innerHTML = `
        <div class="domain-detail-loading">
            <div class="skeleton-line"></div>
            <div class="skeleton-line short"></div>
            <div class="skeleton-grid">
                <div class="skeleton-item"></div>
                <div class="skeleton-item"></div>
                <div class="skeleton-item"></div>
                <div class="skeleton-item"></div>
            </div>
        </div>
    `;

    // Fetch RDAP info
    fetchRdapInfo(domain);
}

async function fetchRdapInfo(domain) {
    try {
        const response = await fetch('/api/rdap', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ domains: [domain] })
        });
        const data = await response.json();

        // Only render if we're still showing this domain
        if (currentDomain !== domain) return;

        if (data.error) {
            throw new Error(data.error);
        }

        // Extract the domain info from results map
        const domainInfo = data.results && data.results[domain];
        if (!domainInfo) {
            // Domain not found in RDAP - likely available
            const availableInfo = { available: true };
            rdapInfoCache.set(domain.toLowerCase(), availableInfo);
            renderDomainDetail(availableInfo);
            return;
        }

        // Cache the result
        rdapInfoCache.set(domain.toLowerCase(), domainInfo);
        renderDomainDetail(domainInfo);
    } catch (err) {
        console.error('Failed to fetch RDAP info:', err);
        dom.domainDetailContent.innerHTML = `
            <div class="domain-detail-error">
                <p>Unable to fetch domain information.</p>
                <p class="error-hint">Please try again later.</p>
            </div>
        `;
    }
}

function renderDomainDetail(data) {
    const isAvailable = data.available === true;

    if (isAvailable) {
        dom.domainDetailContent.innerHTML = `
            <div class="domain-detail-available">
                <span class="available-badge">Available for Registration</span>
                <p class="available-hint">This domain is not currently registered and can be purchased.</p>
            </div>
        `;
        return;
    }

    // Registered domain - show details
    const domainAge = formatDomainAge(data.createdDate);
    const createdDate = formatDate(data.createdDate);
    const expiresDate = formatDate(data.expirationDate);
    const updatedDate = formatDate(data.updatedDate);
    const registrar = data.registrar || 'Unknown';
    const dnssec = data.dnssec ? 'Enabled' : 'Not enabled';

    // Nameservers
    const nameservers = data.nameservers || [];
    const nameserversHtml = nameservers.length > 0
        ? nameservers.map(ns => `<li>${escapeHtml(ns)}</li>`).join('')
        : '<li class="no-data">Not available</li>';

    // Status codes
    const statuses = data.status || [];
    const statusHtml = statuses.length > 0
        ? statuses.map(s => `<span class="status-code">${escapeHtml(s)}</span>`).join('')
        : '<span class="no-data">None</span>';

    dom.domainDetailContent.innerHTML = `
        ${domainAge ? `<div class="domain-age">Registered for <strong>${domainAge}</strong></div>` : ''}

        <div class="domain-detail-grid">
            <div class="detail-item">
                <span class="detail-label">Registrar</span>
                <span class="detail-value">${escapeHtml(registrar)}</span>
            </div>
            <div class="detail-item">
                <span class="detail-label">Created</span>
                <span class="detail-value">${createdDate}</span>
            </div>
            <div class="detail-item">
                <span class="detail-label">Expires</span>
                <span class="detail-value">${expiresDate}</span>
            </div>
            <div class="detail-item">
                <span class="detail-label">Updated</span>
                <span class="detail-value">${updatedDate}</span>
            </div>
            <div class="detail-item">
                <span class="detail-label">DNSSEC</span>
                <span class="detail-value">${dnssec}</span>
            </div>
        </div>

        <div class="domain-detail-section">
            <h4 class="section-title">Nameservers</h4>
            <ul class="nameserver-list">${nameserversHtml}</ul>
        </div>

        <div class="domain-detail-section">
            <h4 class="section-title">Status</h4>
            <div class="status-list">${statusHtml}</div>
        </div>
    `;
}

export function initDomainDetailModal() {
    dom.domainDetailClose.addEventListener('click', () => {
        dom.domainDetailModal.hidden = true;
        currentDomain = null;
    });

    dom.domainDetailModal.addEventListener('click', (e) => {
        if (e.target === dom.domainDetailModal) {
            dom.domainDetailModal.hidden = true;
            currentDomain = null;
        }
    });

    // Close on Escape key
    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape' && !dom.domainDetailModal.hidden) {
            dom.domainDetailModal.hidden = true;
            currentDomain = null;
        }
    });
}
