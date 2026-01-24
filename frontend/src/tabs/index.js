import { dom } from '../dom.js';
import {
    tabs, activeTabId, currentUser,
    setTabs, setActiveTabId, incrementTabCounter, setComSiteChecks
} from '../state.js';
import { FREE_TAB_LIMIT } from '../config.js';
import { renderTabBar } from './render.js';
import { saveTabsToStorage } from './persistence.js';
import { renderResultsForTab } from '../search/results.js';
import { escapeHtml } from '../utils.js';

export function createTab(description = '', tldStyle = 'traditional') {
    const tabId = `tab-${incrementTabCounter()}`;
    const tab = {
        id: tabId,
        description: description,
        title: '', // AI-generated title
        tldStyle: tldStyle,
        categories: {},
        isLoading: false,
        error: null,
        rounds: 1,
        comSiteChecks: new Map()
    };

    // Max 10 tabs - remove oldest if exceeded
    if (tabs.length >= 10) {
        const oldestTab = tabs.shift();
        if (activeTabId === oldestTab.id) {
            setActiveTabId(null);
        }
    }

    tabs.push(tab);
    setActiveTabId(tab.id);
    renderTabBar();
    saveTabsToStorage();

    // Track tab open (fire and forget)
    fetch('/api/track/tab-open', { method: 'POST' }).catch(() => {});

    return tab;
}

export function getActiveTab() {
    return tabs.find(t => t.id === activeTabId) || null;
}

export function switchToTab(tabId) {
    const tab = tabs.find(t => t.id === tabId);
    if (!tab) return;

    setActiveTabId(tabId);
    setComSiteChecks(tab.comSiteChecks);
    renderTabBar();
    saveTabsToStorage();

    // Hide other views
    dom.favoritesView.hidden = true;
    dom.historyView.hidden = true;
    dom.monitoringView.hidden = true;
    dom.welcomeContent.hidden = true;
    if (dom.registrationView) dom.registrationView.hidden = true;

    // Show tab content
    if (tab.isLoading) {
        dom.resultsEl.innerHTML = `
            <div class="searching-state">
                <div class="search-animation">
                    <div class="orbit">
                        <div class="orbit-dot"></div>
                        <div class="orbit-dot"></div>
                        <div class="orbit-dot"></div>
                    </div>
                    <div class="orbit orbit-reverse">
                        <div class="orbit-dot"></div>
                        <div class="orbit-dot"></div>
                    </div>
                    <div class="search-icon">◇</div>
                </div>
                <p class="search-text">Searching for available domains<span class="search-dots"></span></p>
                <p class="search-subtext">May run up to 5 rounds to find the best options</p>
                <div class="tld-parade">
                    <span>.com</span><span>.io</span><span>.co</span><span>.dev</span><span>.app</span><span>.ai</span>
                </div>
            </div>`;
        dom.resultsEl.hidden = false;
    } else if (tab.error) {
        dom.resultsEl.innerHTML = `<p class="error-message">${escapeHtml(tab.error)}</p>`;
        dom.resultsEl.hidden = false;
    } else if (Object.keys(tab.categories).length > 0) {
        renderResultsForTab(tab);
        dom.resultsEl.hidden = false;
    } else {
        // Empty tab - show welcome but keep tab bar visible
        dom.resultsEl.hidden = true;
        dom.welcomeContent.hidden = false;
    }
}

export function closeTab(tabId) {
    const index = tabs.findIndex(t => t.id === tabId);
    if (index === -1) return;

    tabs.splice(index, 1);
    saveTabsToStorage();

    if (tabs.length === 0) {
        // No tabs left - show welcome
        setActiveTabId(null);
        dom.tabBar.hidden = true;
        dom.resultsEl.hidden = true;
        dom.welcomeContent.hidden = false;
    } else if (activeTabId === tabId) {
        // Closed active tab - switch to nearest
        const newIndex = Math.min(index, tabs.length - 1);
        switchToTab(tabs[newIndex].id);
    } else {
        renderTabBar();
    }
}

export function canCreateNewTab() {
    const isPro = currentUser && currentUser.subscriptionTier === 'pro';
    if (isPro) return true;
    return tabs.length < FREE_TAB_LIMIT;
}
