package cache

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteCache implements Cacher using SQLite for local development
type SQLiteCache struct {
	db  *sql.DB
	ttl time.Duration
}

func NewSQLite(dbPath string, ttl time.Duration) (*SQLiteCache, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Create table if not exists (with price column)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS domain_availability (
			domain TEXT PRIMARY KEY,
			available INTEGER NOT NULL,
			is_premium INTEGER NOT NULL,
			price REAL,
			checked_at DATETIME NOT NULL
		)
	`)
	if err != nil {
		return nil, err
	}

	// Add price column if it doesn't exist (migration for existing DBs)
	db.Exec(`ALTER TABLE domain_availability ADD COLUMN price REAL`)

	// Create index on checked_at for cleanup
	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_checked_at ON domain_availability(checked_at)
	`)
	if err != nil {
		return nil, err
	}

	return &SQLiteCache{db: db, ttl: ttl}, nil
}

func (c *SQLiteCache) Get(domain string) (*CachedResult, bool) {
	var result CachedResult
	var available, isPremium int
	var price sql.NullFloat64
	var checkedAt string

	err := c.db.QueryRow(`
		SELECT domain, available, is_premium, price, checked_at
		FROM domain_availability
		WHERE domain = ?
	`, domain).Scan(&result.Domain, &available, &isPremium, &price, &checkedAt)

	if err != nil {
		return nil, false
	}

	result.Available = available == 1
	result.IsPremium = isPremium == 1
	if price.Valid {
		result.Price = &price.Float64
	}
	result.CheckedAt, _ = time.Parse(time.RFC3339, checkedAt)

	// Check if expired
	if time.Since(result.CheckedAt) > c.ttl {
		return nil, false
	}

	return &result, true
}

func (c *SQLiteCache) GetMany(domains []string) (map[string]*CachedResult, []string) {
	cached := make(map[string]*CachedResult)
	var uncached []string

	for _, d := range domains {
		if result, ok := c.Get(d); ok {
			cached[d] = result
		} else {
			uncached = append(uncached, d)
		}
	}

	return cached, uncached
}

func (c *SQLiteCache) Set(domain string, available, isPremium bool, price *float64) error {
	availableInt := 0
	if available {
		availableInt = 1
	}
	isPremiumInt := 0
	if isPremium {
		isPremiumInt = 1
	}

	var priceVal interface{} = nil
	if price != nil {
		priceVal = *price
	}

	_, err := c.db.Exec(`
		INSERT OR REPLACE INTO domain_availability (domain, available, is_premium, price, checked_at)
		VALUES (?, ?, ?, ?, ?)
	`, domain, availableInt, isPremiumInt, priceVal, time.Now().Format(time.RFC3339))

	return err
}

func (c *SQLiteCache) Cleanup() error {
	cutoff := time.Now().Add(-c.ttl).Format(time.RFC3339)
	_, err := c.db.Exec(`DELETE FROM domain_availability WHERE checked_at < ?`, cutoff)
	return err
}

func (c *SQLiteCache) Close() error {
	return c.db.Close()
}
