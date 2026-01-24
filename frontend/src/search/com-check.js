import { comSiteChecks } from '../state.js';
import { extractBaseName } from '../utils.js';
import { saveTabsToStorage } from '../tabs/persistence.js';

export async function checkComSite(domain) {
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
                domain: data.domain,
                expirationDate: data.expirationDate,
                daysUntilExpiry: data.daysUntilExpiry,
                registrar: data.registrar
            });
            saveTabsToStorage();
            return data;
        }
    } catch (err) {
        console.error('Failed to check .com site:', err);
    }
    return null;
}

export function getComStatusHtml(status, comDomain, expirationDate, daysUntilExpiry) {
    // Format expiration info for taken domains
    let expiryHtml = '';
    if (daysUntilExpiry !== undefined && daysUntilExpiry !== null && status !== 'available') {
        if (daysUntilExpiry <= 0) {
            expiryHtml = '<span class="com-expiry expiring-soon">expired!</span>';
        } else if (daysUntilExpiry <= 90) {
            expiryHtml = `<span class="com-expiry expiring-soon">expires ${daysUntilExpiry}d</span>`;
        } else if (expirationDate) {
            // Format as "Mar 2025"
            const date = new Date(expirationDate);
            const month = date.toLocaleString('en-US', { month: 'short' });
            const year = date.getFullYear();
            expiryHtml = `<span class="com-expiry">exp ${month} ${year}</span>`;
        }
    }

    if (status === 'active') {
        return `<span class="com-status com-active" title="${comDomain} has an active website">⚠ .com active</span>${expiryHtml}`;
    } else if (status === 'parked') {
        return `<span class="com-status com-parked" title="${comDomain} is parked/for sale">◐ .com parked</span>${expiryHtml}`;
    } else if (status === 'available') {
        return `<span class="com-status com-available" title="${comDomain} is available!">✓ .com free</span>`;
    } else {
        return `<span class="com-status com-inactive" title="${comDomain} has no active site">✓ .com clear</span>${expiryHtml}`;
    }
}
