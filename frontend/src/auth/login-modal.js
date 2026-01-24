import { dom } from '../dom.js';
import { shakeElement } from '../utils.js';

export function openLoginModal() {
    dom.loginModal.hidden = false;
    dom.loginForm.hidden = false;
    dom.loginSent.hidden = true;
    dom.loginError.hidden = true;
    dom.loginModalText.hidden = false;
    dom.loginEmail.value = '';
    dom.loginEmail.focus();
}

export function initLoginModal() {
    dom.loginClose.addEventListener('click', () => {
        dom.loginModal.hidden = true;
    });

    dom.loginModal.addEventListener('click', (e) => {
        if (e.target === dom.loginModal) {
            dom.loginModal.hidden = true;
        }
    });

    dom.loginForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        const email = dom.loginEmail.value.trim();
        if (!email) {
            shakeElement(dom.loginEmail);
            return;
        }

        dom.loginSubmitBtn.disabled = true;
        dom.loginBtnText.hidden = true;
        dom.loginBtnLoading.hidden = false;
        dom.loginError.hidden = true;

        try {
            const response = await fetch('/api/auth/login', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ email })
            });

            const data = await response.json();

            if (response.ok && data.success) {
                dom.loginForm.hidden = true;
                dom.loginModalText.hidden = true;
                dom.loginSent.hidden = false;
                dom.sentEmail.textContent = email;
            } else {
                dom.loginError.textContent = data.error || 'Failed to send login email';
                dom.loginError.hidden = false;
            }
        } catch (err) {
            dom.loginError.textContent = 'Failed to send login email. Please try again.';
            dom.loginError.hidden = false;
        } finally {
            dom.loginSubmitBtn.disabled = false;
            dom.loginBtnText.hidden = false;
            dom.loginBtnLoading.hidden = true;
        }
    });
}
