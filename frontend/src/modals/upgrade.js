import { dom } from '../dom.js';
import { apiFetch } from '../api.js';

export function openUpgradeModal() {
    dom.upgradeModal.hidden = false;
    dom.upgradeError.hidden = true;
}

export function initUpgradeModal() {
    dom.upgradeClose.addEventListener('click', () => {
        dom.upgradeModal.hidden = true;
    });

    dom.upgradeModal.addEventListener('click', (e) => {
        if (e.target === dom.upgradeModal) {
            dom.upgradeModal.hidden = true;
        }
    });

    dom.upgradeBtn.addEventListener('click', async () => {
        dom.upgradeBtn.disabled = true;
        dom.upgradeBtnText.hidden = true;
        dom.upgradeBtnLoading.hidden = false;
        dom.upgradeError.hidden = true;

        try {
            const response = await apiFetch('/api/billing/checkout', {
                method: 'POST'
            });

            const data = await response.json();

            if (response.ok && data.url) {
                // Redirect to Stripe Checkout
                window.location.href = data.url;
            } else {
                dom.upgradeError.textContent = data.error || 'Failed to start checkout';
                dom.upgradeError.hidden = false;
            }
        } catch (err) {
            dom.upgradeError.textContent = 'Failed to start checkout. Please try again.';
            dom.upgradeError.hidden = false;
        } finally {
            dom.upgradeBtn.disabled = false;
            dom.upgradeBtnText.hidden = false;
            dom.upgradeBtnLoading.hidden = true;
        }
    });
}
