// DOM element cache
export const dom = {};

export function initDom() {
    // Form elements
    dom.form = document.getElementById('generate-form');
    dom.submitBtn = document.getElementById('submit-btn');
    dom.btnText = dom.submitBtn.querySelector('.btn-text');
    dom.btnLoading = dom.submitBtn.querySelector('.btn-loading');
    dom.resultsEl = document.getElementById('results');
    dom.welcomeContent = document.getElementById('welcome-content');
    dom.tldStyleInput = document.getElementById('tld-style');
    dom.tldCustomizeBtn = document.getElementById('tld-customize-btn');
    dom.tldCustomPanel = document.getElementById('tld-custom-panel');
    dom.tldCustomDone = document.getElementById('tld-custom-done');
    dom.descriptionInput = document.getElementById('description');

    // Maintenance
    dom.maintenanceOverlay = document.getElementById('maintenance-overlay');
    dom.maintenanceCountdown = document.getElementById('maintenance-countdown');

    // Tour elements
    dom.tourOverlay = document.getElementById('onboarding-tour');
    dom.tourSpotlight = document.getElementById('tour-spotlight');
    dom.tourTooltip = document.getElementById('tour-tooltip');
    dom.getStartedBtn = document.getElementById('get-started-btn');
    dom.statDomainsEl = document.getElementById('stat-domains');

    // Auth elements
    dom.loginModal = document.getElementById('login-modal');
    dom.loginForm = document.getElementById('login-form');
    dom.loginEmail = document.getElementById('login-email');
    dom.loginSubmitBtn = document.getElementById('login-submit-btn');
    dom.loginBtnText = dom.loginSubmitBtn.querySelector('.login-btn-text');
    dom.loginBtnLoading = dom.loginSubmitBtn.querySelector('.login-btn-loading');
    dom.loginSent = document.getElementById('login-sent');
    dom.sentEmail = document.getElementById('sent-email');
    dom.loginError = document.getElementById('login-error');
    dom.loginClose = document.getElementById('login-close');
    dom.loginModalText = dom.loginModal.querySelector('.modal-text');

    // User menu elements
    dom.signInBtn = document.getElementById('sign-in-btn');
    dom.userDropdown = document.getElementById('user-dropdown');
    dom.userBtn = document.getElementById('user-btn');
    dom.userEmailEl = document.getElementById('user-email');
    dom.dropdownMenu = document.getElementById('dropdown-menu');
    dom.searchConsoleBtn = document.getElementById('search-console-btn');
    dom.favoritesBtn = document.getElementById('favorites-btn');
    dom.logoutBtn = document.getElementById('logout-btn');

    // Favorites view elements
    dom.favoritesView = document.getElementById('favorites-view');
    dom.favoritesList = document.getElementById('favorites-list');
    dom.favoritesClose = document.getElementById('favorites-close');

    // History view elements
    dom.historyView = document.getElementById('history-view');
    dom.historyList = document.getElementById('history-list');
    dom.historyClose = document.getElementById('history-close');
    dom.historyBtn = document.getElementById('history-btn');

    // Monitoring view elements
    dom.monitoringView = document.getElementById('monitoring-view');
    dom.monitoringList = document.getElementById('monitoring-list');
    dom.monitoringClose = document.getElementById('monitoring-close');
    dom.monitoringBtn = document.getElementById('monitoring-btn');

    // Upgrade modal elements
    dom.upgradeModal = document.getElementById('upgrade-modal');
    dom.upgradeBtn = document.getElementById('upgrade-btn');
    dom.upgradeBtnText = dom.upgradeBtn.querySelector('.upgrade-btn-text');
    dom.upgradeBtnLoading = dom.upgradeBtn.querySelector('.upgrade-btn-loading');
    dom.upgradeClose = document.getElementById('upgrade-close');
    dom.upgradeError = document.getElementById('upgrade-error');

    // Upgrade/manage buttons in dropdowns
    dom.upgradeMenuBtn = document.getElementById('upgrade-menu-btn');
    dom.manageBtn = document.getElementById('manage-btn');

    // Admin buttons
    dom.adminBtn = document.getElementById('admin-btn');

    // Plan info elements
    dom.planName = document.getElementById('plan-name');
    dom.planDetail = document.getElementById('plan-detail');

    // Owned modal elements
    dom.ownedModal = document.getElementById('owned-modal');
    dom.ownedClose = document.getElementById('owned-close');
    dom.ownedDomainName = document.getElementById('owned-domain-name');
    dom.ownedError = document.getElementById('owned-error');

    // Tab limit modal elements
    dom.tabLimitModal = document.getElementById('tab-limit-modal');
    dom.tabLimitClose = document.getElementById('tab-limit-close');
    dom.tabLimitUpgradeBtn = document.getElementById('tab-limit-upgrade-btn');
    dom.tabLimitCloseTabBtn = document.getElementById('tab-limit-close-tab-btn');

    // Theme toggle elements
    dom.themeToggleBtn = document.getElementById('theme-toggle-btn');
    dom.themeToggleDropdown = document.getElementById('theme-toggle-dropdown');

    // Tab bar elements
    dom.tabBar = document.getElementById('tab-bar');
    dom.tabList = document.getElementById('tab-list');
    dom.tabNewBtn = document.getElementById('tab-new-btn');

    // Registration view elements
    dom.registrationView = document.getElementById('registration-view');
    dom.registrationBack = document.getElementById('registration-back');
    dom.registrationDomain = document.getElementById('registration-domain');
    dom.registrationStats = document.getElementById('registration-stats');
    dom.registrarCards = document.getElementById('registrar-cards');
    dom.registrationPreference = document.getElementById('registration-preference');
    dom.rememberRegistrar = document.getElementById('remember-registrar');
    dom.loginForPreference = document.getElementById('login-for-preference');
    dom.loginForPrefLink = document.getElementById('login-for-pref-link');

    // Logo
    dom.logoHome = document.getElementById('logo-home');
}
