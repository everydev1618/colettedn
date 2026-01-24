import { userFavorites, userOwnedDomains, comSiteChecks } from '../state.js';
import { escapeHtml, formatRelativeTime, extractBaseName } from '../utils.js';
import { getComStatusHtml } from './com-check.js';

export function isComInResults(baseName, categories) {
    // Check if {baseName}.com is already in the results
    const comDomain = baseName + '.com';
    for (const cat of Object.values(categories)) {
        if (cat.some(d => d.name.toLowerCase() === comDomain)) {
            return true;
        }
    }
    return false;
}

export function renderDomainCard(domain, index, categories) {
    let metaHtml = '';

    const statusClass = domain.available === false ? 'taken' : (domain.available === true ? 'available' : 'unverified');
    const statusText = domain.available === true ? 'Available' : (domain.available === false ? 'Taken' : 'Verify');

    metaHtml = `<span class="domain-status ${statusClass}">${statusText}</span>`;

    // Build score squares HTML if score available
    let scoreBarHtml = '';
    if (domain.score) {
        // Convert 0-100 score to 1-5 squares
        const filledSquares = Math.ceil(domain.score / 20);
        const tooltips = [
            '',
            'Weak choice — hard to remember or spell',
            'Passable — some drawbacks but usable',
            'Decent — reasonably brandable',
            'Strong — memorable and professional',
            'Excellent — short, catchy, and brandable'
        ];
        const tooltip = tooltips[filledSquares] || '';
        let squares = '';
        for (let i = 1; i <= 5; i++) {
            squares += `<div class="score-square ${i <= filledSquares ? 'filled' : ''}"></div>`;
        }
        scoreBarHtml = `
            <div class="domain-score-bar score-level-${filledSquares}" title="${tooltip}">
                ${squares}
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
                comCheckHtml = getComStatusHtml(comCheck.status, comCheck.domain, comCheck.expirationDate, comCheck.daysUntilExpiry);
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
                <span class="domain-name" title="Click for domain details">${escapeHtml(domain.name)}</span>
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
