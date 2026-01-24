import { authToken } from './state.js';
import { apiFetch } from './api.js';
import { dom } from './dom.js';

export function toggleTheme() {
    const currentTheme = document.documentElement.getAttribute('data-theme');
    const newTheme = currentTheme === 'dark' ? 'light' : 'dark';
    document.documentElement.setAttribute('data-theme', newTheme);
    localStorage.setItem('theme', newTheme);

    // Sync with server if logged in
    if (authToken) {
        apiFetch('/api/user/preferences', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ theme: newTheme })
        }).catch(err => console.error('Failed to save theme preference:', err));
    }
}

export function applyUserTheme(theme) {
    if (theme && (theme === 'light' || theme === 'dark')) {
        document.documentElement.setAttribute('data-theme', theme);
        localStorage.setItem('theme', theme);
    }
}

export function initTheme() {
    dom.themeToggleBtn.addEventListener('click', toggleTheme);
    dom.themeToggleDropdown.addEventListener('click', toggleTheme);
}
