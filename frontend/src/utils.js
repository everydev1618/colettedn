export function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

export function formatRelativeTime(timestamp) {
    const diff = Date.now() - timestamp * 1000;
    const minutes = Math.floor(diff / 60000);
    const hours = Math.floor(diff / 3600000);
    if (minutes < 1) return 'now';
    if (minutes < 60) return `${minutes}m`;
    if (hours < 24) return `${hours}h`;
    return `${Math.floor(hours / 24)}d`;
}

export function formatHistoryDate(date) {
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

export function shakeElement(el) {
    el.style.animation = 'none';
    el.offsetHeight;
    el.style.animation = 'shake 0.4s ease';
    el.focus();
}

export function animateCounter(el, target) {
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

export function formatExpiryBadge(daysUntilExpiry, expirationDate) {
    if (daysUntilExpiry === null && !expirationDate) {
        return '<span class="expiry-badge">Expiry unknown</span>';
    }

    let displayText = '';
    let badgeClass = 'expiry-badge';

    if (daysUntilExpiry !== null && daysUntilExpiry !== undefined) {
        if (daysUntilExpiry <= 0) {
            displayText = 'Expired';
            badgeClass += ' expired';
        } else if (daysUntilExpiry <= 30) {
            displayText = `${daysUntilExpiry}d`;
            badgeClass += ' expiring-soon';
        } else if (daysUntilExpiry <= 60) {
            displayText = `${Math.round(daysUntilExpiry / 7)}w`;
            badgeClass += ' expiring-soon';
        } else if (daysUntilExpiry <= 365) {
            displayText = `${Math.round(daysUntilExpiry / 30)}mo`;
        } else {
            displayText = `${Math.round(daysUntilExpiry / 365)}y`;
        }
    } else if (expirationDate) {
        displayText = expirationDate;
    }

    return `<span class="${badgeClass}">${displayText}</span>`;
}

export function extractBaseName(domain) {
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

// Initialize shake animation style
export function initShakeAnimation() {
    const style = document.createElement('style');
    style.textContent = `@keyframes shake {
        0%, 100% { transform: translateX(0); }
        20%, 60% { transform: translateX(-4px); }
        40%, 80% { transform: translateX(4px); }
    }`;
    document.head.appendChild(style);
}
