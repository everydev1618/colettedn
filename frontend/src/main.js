// Main entry point for the Colette DN frontend

import { initDom } from './dom.js';
import { initShakeAnimation } from './utils.js';
import { initTheme } from './theme.js';
import { initAuth } from './auth/index.js';
import { initLoginModal } from './auth/login-modal.js';
import { initUserMenu } from './auth/user-menu.js';
import { initUpgradeModal } from './modals/upgrade.js';
import { initOwnedModal } from './modals/owned.js';
import { initTabLimitModal } from './modals/tab-limit.js';
import { initDomainDetailModal } from './modals/domain-detail.js';
import { initWelcome } from './views/welcome.js';
import { initFavorites } from './views/favorites.js';
import { initHistory } from './views/history.js';
import { initMonitoring } from './views/monitoring.js';
import { initRegistration } from './views/registration.js';
import { loadTabsFromStorage } from './tabs/persistence.js';
import { initSearchForm } from './search/form.js';

// Initialize app when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    // Initialize DOM element cache
    initDom();

    // Initialize shake animation
    initShakeAnimation();

    // Initialize theme
    initTheme();

    // Initialize auth
    initLoginModal();
    initUserMenu();
    initAuth();

    // Initialize modals
    initUpgradeModal();
    initOwnedModal();
    initTabLimitModal();
    initDomainDetailModal();

    // Initialize views
    initWelcome();
    initFavorites();
    initHistory();
    initMonitoring();
    initRegistration();

    // Load tabs from storage
    loadTabsFromStorage();

    // Initialize search form
    initSearchForm();
});
