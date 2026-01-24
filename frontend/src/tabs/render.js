import { dom } from '../dom.js';
import { tabs, activeTabId } from '../state.js';
import { escapeHtml } from '../utils.js';
import { switchToTab, closeTab } from './index.js';

export function renderTabBar() {
    if (tabs.length === 0) {
        dom.tabBar.hidden = true;
        return;
    }

    dom.tabBar.hidden = false;

    dom.tabList.innerHTML = tabs.map(tab => {
        const isActive = tab.id === activeTabId;
        // Use AI-generated title if available, otherwise fall back to description
        let title;
        if (tab.title) {
            title = tab.title.length > 20 ? tab.title.substring(0, 20) + '...' : tab.title;
        } else if (tab.description) {
            title = tab.description.length > 20 ? tab.description.substring(0, 20) + '...' : tab.description;
        } else {
            title = 'New Search';
        }

        return `
            <button class="tab${isActive ? ' active' : ''}" data-tab-id="${tab.id}">
                ${tab.isLoading ? '<span class="tab-spinner"></span>' : ''}
                <span class="tab-title">${escapeHtml(title)}</span>
                <span class="tab-close" data-tab-id="${tab.id}">&times;</span>
            </button>
        `;
    }).join('');

    // Add tab click handlers
    dom.tabList.querySelectorAll('.tab').forEach(tabEl => {
        tabEl.addEventListener('click', (e) => {
            // Don't switch if clicking close button
            if (e.target.classList.contains('tab-close')) return;
            switchToTab(tabEl.dataset.tabId);
        });
    });

    // Add close button handlers
    dom.tabList.querySelectorAll('.tab-close').forEach(closeBtn => {
        closeBtn.addEventListener('click', (e) => {
            e.stopPropagation();
            closeTab(closeBtn.dataset.tabId);
        });
    });
}
