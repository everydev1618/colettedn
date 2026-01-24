import { dom } from '../dom.js';
import { logout } from './index.js';
import { openLoginModal } from './login-modal.js';
import { showFavoritesView } from '../views/favorites.js';
import { showHistoryView } from '../views/history.js';
import { showMonitoringView } from '../views/monitoring.js';
import { openUpgradeModal } from '../modals/upgrade.js';
import { apiFetch } from '../api.js';
import { tabs, activeTabId } from '../state.js';
import { getActiveTab, switchToTab } from '../tabs/index.js';

export function initUserMenu() {
    dom.signInBtn.addEventListener('click', openLoginModal);

    dom.userBtn.addEventListener('click', () => {
        const isOpen = !dom.dropdownMenu.hidden;
        dom.dropdownMenu.hidden = isOpen;
        dom.userDropdown.classList.toggle('open', !isOpen);
    });

    // Close dropdown when clicking outside
    document.addEventListener('click', (e) => {
        if (!dom.userDropdown.contains(e.target)) {
            dom.dropdownMenu.hidden = true;
            dom.userDropdown.classList.remove('open');
        }
    });

    dom.searchConsoleBtn.addEventListener('click', () => {
        dom.dropdownMenu.hidden = true;
        dom.userDropdown.classList.remove('open');
        dom.favoritesView.hidden = true;
        dom.historyView.hidden = true;
        const activeTab = getActiveTab();
        if (activeTab && Object.keys(activeTab.categories).length > 0) {
            switchToTab(activeTab.id);
        } else {
            dom.welcomeContent.hidden = false;
        }
    });

    dom.favoritesBtn.addEventListener('click', () => {
        dom.dropdownMenu.hidden = true;
        dom.userDropdown.classList.remove('open');
        showFavoritesView();
    });

    dom.historyBtn.addEventListener('click', () => {
        dom.dropdownMenu.hidden = true;
        dom.userDropdown.classList.remove('open');
        showHistoryView();
    });

    dom.monitoringBtn.addEventListener('click', () => {
        dom.dropdownMenu.hidden = true;
        dom.userDropdown.classList.remove('open');
        showMonitoringView();
    });

    dom.logoutBtn.addEventListener('click', () => {
        dom.dropdownMenu.hidden = true;
        dom.userDropdown.classList.remove('open');
        logout();
    });

    dom.upgradeMenuBtn.addEventListener('click', () => {
        dom.dropdownMenu.hidden = true;
        dom.userDropdown.classList.remove('open');
        openUpgradeModal();
    });

    dom.manageBtn.addEventListener('click', () => {
        dom.dropdownMenu.hidden = true;
        dom.userDropdown.classList.remove('open');
        openManageSubscription();
    });

    dom.adminBtn.addEventListener('click', () => {
        dom.dropdownMenu.hidden = true;
        dom.userDropdown.classList.remove('open');
        window.location.href = '/admin';
    });
}

async function openManageSubscription() {
    try {
        const response = await apiFetch('/api/billing/portal', {
            method: 'POST'
        });

        const data = await response.json();

        if (response.ok && data.url) {
            window.location.href = data.url;
        } else {
            alert(data.error || 'Failed to open subscription management');
        }
    } catch (err) {
        alert('Failed to open subscription management. Please try again.');
    }
}
