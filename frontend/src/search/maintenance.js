import { dom } from '../dom.js';
import { maintenanceTimer, setMaintenanceTimer } from '../state.js';

export function showMaintenanceMode() {
    dom.maintenanceOverlay.hidden = false;
    document.body.style.overflow = 'hidden';

    // Start 15-minute countdown
    let remaining = 15 * 60; // 15 minutes in seconds

    function updateCountdown() {
        const mins = Math.floor(remaining / 60);
        const secs = remaining % 60;
        dom.maintenanceCountdown.textContent = `${mins}:${secs.toString().padStart(2, '0')}`;

        if (remaining > 0) {
            remaining--;
        }
    }

    updateCountdown();
    if (maintenanceTimer) clearInterval(maintenanceTimer);
    setMaintenanceTimer(setInterval(updateCountdown, 1000));
}

export function hideMaintenanceMode() {
    dom.maintenanceOverlay.hidden = true;
    document.body.style.overflow = '';
    if (maintenanceTimer) {
        clearInterval(maintenanceTimer);
        setMaintenanceTimer(null);
    }
}
