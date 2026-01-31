// Auth state
export let authToken = localStorage.getItem('authToken');
export let currentUser = null;
export let userFavorites = new Set();
export let userOwnedDomains = new Map(); // domain -> { acquisitionType, createdAt }
export let userMonitoring = new Map(); // domain -> { expirationDate, daysUntilExpiry, registrar }
export let usageInfo = null; // { used, limit, unlimited }

// Tab state
export let tabs = [];
export let activeTabId = null;
export let tabCounter = 0;

// Per-tab .com site checks
export let comSiteChecks = new Map();

// Per-tab TLD filter (null = "All")
export let tldFilters = new Map(); // tabId -> selectedTld (null for All)

// Registration state
export let currentRegistrationDomain = null;
export let userPreferredRegistrar = null;
export let userPreferredOtherRegistrar = null; // For "Other" registrar selection
export let pendingOwnedDomain = null;

// RDAP info cache (session-level)
export let rdapInfoCache = new Map();

// Maintenance
export let maintenanceTimer = null;

// Setters for state that needs to be modified from other modules
export function setAuthToken(token) {
    authToken = token;
    if (token) {
        localStorage.setItem('authToken', token);
    } else {
        localStorage.removeItem('authToken');
    }
}

export function setCurrentUser(user) {
    currentUser = user;
}

export function setUsageInfo(info) {
    usageInfo = info;
}

export function setTabs(newTabs) {
    tabs = newTabs;
}

export function setActiveTabId(id) {
    activeTabId = id;
}

export function setTabCounter(count) {
    tabCounter = count;
}

export function incrementTabCounter() {
    tabCounter++;
    return tabCounter;
}

export function setComSiteChecks(checks) {
    comSiteChecks = checks;
}

export function setTldFilter(tabId, tld) {
    tldFilters.set(tabId, tld);
}

export function getTldFilter(tabId) {
    return tldFilters.get(tabId) || null;
}

export function setCurrentRegistrationDomain(domain) {
    currentRegistrationDomain = domain;
}

export function setUserPreferredRegistrar(registrar) {
    userPreferredRegistrar = registrar;
}

export function setUserPreferredOtherRegistrar(registrar) {
    userPreferredOtherRegistrar = registrar;
}

export function setPendingOwnedDomain(domain) {
    pendingOwnedDomain = domain;
}

export function setMaintenanceTimer(timer) {
    maintenanceTimer = timer;
}
