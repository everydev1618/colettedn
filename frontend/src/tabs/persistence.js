import { tabs, activeTabId, tabCounter, setTabs, setActiveTabId, setTabCounter, setComSiteChecks } from '../state.js';
import { renderTabBar } from './render.js';
import { renderResultsForTab } from '../search/results.js';

export function saveTabsToStorage() {
    try {
        // Convert tabs to serializable format (Maps become arrays)
        const serializableTabs = tabs.map(tab => ({
            ...tab,
            comSiteChecks: Array.from(tab.comSiteChecks.entries())
        }));
        localStorage.setItem('colette_tabs', JSON.stringify({
            tabs: serializableTabs,
            activeTabId: activeTabId,
            tabCounter: tabCounter
        }));
    } catch (err) {
        console.error('Failed to save tabs:', err);
    }
}

export function loadTabsFromStorage() {
    try {
        const stored = localStorage.getItem('colette_tabs');
        if (!stored) return false;

        const data = JSON.parse(stored);
        if (!data.tabs || !Array.isArray(data.tabs)) return false;

        // Restore tabs with Maps
        const restoredTabs = data.tabs.map(tab => ({
            ...tab,
            comSiteChecks: new Map(tab.comSiteChecks || []),
            isLoading: false // Reset loading state on page load
        }));

        setTabs(restoredTabs);
        setTabCounter(data.tabCounter || restoredTabs.length);
        setActiveTabId(data.activeTabId);

        // Verify active tab still exists
        if (activeTabId && !tabs.find(t => t.id === activeTabId)) {
            setActiveTabId(tabs.length > 0 ? tabs[0].id : null);
        }

        if (tabs.length > 0) {
            renderTabBar();
            if (activeTabId) {
                const activeTab = tabs.find(t => t.id === activeTabId);
                if (activeTab) {
                    setComSiteChecks(activeTab.comSiteChecks);
                    if (Object.keys(activeTab.categories).length > 0) {
                        renderResultsForTab(activeTab);
                    }
                }
            }
            return true;
        }
    } catch (err) {
        console.error('Failed to load tabs:', err);
        localStorage.removeItem('colette_tabs');
    }
    return false;
}
