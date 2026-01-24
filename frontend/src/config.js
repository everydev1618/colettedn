// Lambda Function URL for generate endpoint (no timeout limit)
// Falls back to relative path if not set
export const FUNCTION_URL = 'https://4tpzgbt5zo7kade7egg5uu75jy0inpuj.lambda-url.us-east-1.on.aws/';

export const FREE_TAB_LIMIT = 3;

export const ADMIN_EMAIL = 'etdebruin@gmail.com';

export const REGISTRARS = {
    namecheap: {
        id: 'namecheap',
        name: 'Namecheap',
        affiliateUrl: 'https://namecheap.pxf.io/c/6878241/1632743/5618',
        getUrl: (d) => `https://www.namecheap.com/domains/registration/results/?domain=${encodeURIComponent(d)}`,
        tagline: 'Best value',
        price: '~$8.88/yr'
    },
    godaddy: {
        id: 'godaddy',
        name: 'GoDaddy',
        affiliateUrl: null,
        getUrl: (d) => `https://www.godaddy.com/domainsearch/find?domainToCheck=${encodeURIComponent(d)}`,
        tagline: 'Most popular',
        price: '~$12.99/yr'
    },
    porkbun: {
        id: 'porkbun',
        name: 'Porkbun',
        affiliateUrl: null,
        getUrl: (d) => `https://porkbun.com/checkout/search?q=${encodeURIComponent(d)}`,
        tagline: 'Great prices',
        price: '~$9.73/yr'
    }
};

// Other registrar options with direct search URLs (sorted alphabetically)
export const OTHER_REGISTRARS = {
    cloudflare: {
        id: 'cloudflare',
        name: 'Cloudflare',
        getUrl: (d) => `https://domains.cloudflare.com/?domain=${encodeURIComponent(d)}`
    },
    'domain.com': {
        id: 'domain.com',
        name: 'Domain.com',
        getUrl: (d) => `https://www.domain.com/registration/?search=${encodeURIComponent(d)}`
    },
    dynadot: {
        id: 'dynadot',
        name: 'Dynadot',
        getUrl: (d) => `https://www.dynadot.com/domain/search?domain=${encodeURIComponent(d)}`
    },
    hover: {
        id: 'hover',
        name: 'Hover',
        getUrl: (d) => `https://www.hover.com/domains/results?q=${encodeURIComponent(d)}`
    },
    'name.com': {
        id: 'name.com',
        name: 'Name.com',
        getUrl: (d) => `https://www.name.com/domain/search?q=${encodeURIComponent(d)}`
    },
    squarespace: {
        id: 'squarespace',
        name: 'Squarespace',
        getUrl: (d) => `https://domains.squarespace.com/domain-search?query=${encodeURIComponent(d)}`
    },
    vercel: {
        id: 'vercel',
        name: 'Vercel',
        getUrl: (d) => `https://vercel.com/domains?q=${encodeURIComponent(d)}`
    }
};

export const TLD_INFO = {
    'com': 'Most recognized TLD',
    'io': 'Tech & startup favorite',
    'co': 'Company & commercial',
    'net': 'Network & infrastructure',
    'org': 'Organizations & nonprofits',
    'ai': 'AI & machine learning',
    'app': 'Mobile & web apps',
    'dev': 'Developer tools',
    'me': 'Personal branding',
    'xyz': 'Modern & creative',
    'tech': 'Technology focused',
    'site': 'General websites',
    'online': 'Online presence'
};
