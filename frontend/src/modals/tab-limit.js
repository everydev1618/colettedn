import { dom } from '../dom.js';
import { openUpgradeModal } from './upgrade.js';

export function showTabLimitModal() {
    dom.tabLimitModal.hidden = false;
}

export function hideTabLimitModal() {
    dom.tabLimitModal.hidden = true;
}

export function initTabLimitModal() {
    dom.tabLimitClose.addEventListener('click', hideTabLimitModal);

    dom.tabLimitModal.addEventListener('click', (e) => {
        if (e.target === dom.tabLimitModal) hideTabLimitModal();
    });

    dom.tabLimitUpgradeBtn.addEventListener('click', () => {
        hideTabLimitModal();
        openUpgradeModal();
    });

    dom.tabLimitCloseTabBtn.addEventListener('click', () => {
        hideTabLimitModal();
        // Focus on the tab bar so user can close a tab
        // Highlight the tabs briefly to indicate they should close one
        dom.tabBar.classList.add('tab-highlight');
        setTimeout(() => dom.tabBar.classList.remove('tab-highlight'), 2000);
    });
}
