document.addEventListener('DOMContentLoaded', () => {
    const form = document.getElementById('generate-form');
    const submitBtn = document.getElementById('submit-btn');
    const btnText = submitBtn.querySelector('.btn-text');
    const btnLoading = submitBtn.querySelector('.btn-loading');
    const resultsSection = document.getElementById('results');
    const domainList = document.getElementById('domain-list');

    form.addEventListener('submit', async (e) => {
        e.preventDefault();

        const keywords = document.getElementById('keywords').value
            .split(',')
            .map(k => k.trim())
            .filter(k => k.length > 0);

        const industry = document.getElementById('industry').value;
        const vibe = document.getElementById('vibe').value;

        const tldCheckboxes = document.querySelectorAll('input[name="tld"]:checked');
        const tlds = Array.from(tldCheckboxes).map(cb => cb.value);

        if (keywords.length === 0) {
            alert('Please enter at least one keyword');
            return;
        }

        if (tlds.length === 0) {
            alert('Please select at least one TLD');
            return;
        }

        // Show loading state
        submitBtn.disabled = true;
        btnText.hidden = true;
        btnLoading.hidden = false;
        resultsSection.hidden = true;

        try {
            const response = await fetch('/api/generate', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    keywords,
                    industry,
                    vibe,
                    tlds,
                }),
            });

            const data = await response.json();

            if (data.error) {
                showError(data.error);
                return;
            }

            displayResults(data.domains);
        } catch (err) {
            showError('Failed to generate domains. Please try again.');
            console.error(err);
        } finally {
            submitBtn.disabled = false;
            btnText.hidden = false;
            btnLoading.hidden = true;
        }
    });

    function displayResults(domains) {
        domainList.innerHTML = '';

        if (!domains || domains.length === 0) {
            domainList.innerHTML = '<p class="error-message">No domains generated. Try different keywords.</p>';
            resultsSection.hidden = false;
            return;
        }

        domains.forEach(domain => {
            const item = document.createElement('div');
            item.className = 'domain-item';

            // Namecheap affiliate link (placeholder - will be configured)
            const affiliateUrl = `https://www.namecheap.com/domains/registration/results/?domain=${encodeURIComponent(domain.name)}`;

            item.innerHTML = `
                <span class="domain-name">${escapeHtml(domain.name)}</span>
                <div class="domain-actions">
                    <a href="${affiliateUrl}" target="_blank" rel="noopener" class="buy-link">Check availability →</a>
                </div>
            `;

            domainList.appendChild(item);
        });

        resultsSection.hidden = false;
    }

    function showError(message) {
        domainList.innerHTML = `<p class="error-message">${escapeHtml(message)}</p>`;
        resultsSection.hidden = false;
    }

    function escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
});
