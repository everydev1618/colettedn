package handler

import (
	"net/http"
	"time"

	"github.com/everydev1618/colettedn/internal/analytics"
	"github.com/everydev1618/colettedn/internal/auth"
	"github.com/everydev1618/colettedn/internal/user"
)

const adminEmail = "etdebruin@gmail.com"

// ComprehensiveStats contains all admin dashboard metrics
type ComprehensiveStats struct {
	// Revenue metrics
	Revenue RevenueStats `json:"revenue"`

	// User metrics
	Users UserMetrics `json:"users"`

	// Engagement metrics
	Engagement EngagementMetrics `json:"engagement"`

	// Operational metrics
	Operational OperationalMetrics `json:"operational"`

	// Recent activity
	RecentSubscribers []SubscriberInfo `json:"recentSubscribers"`
	RecentSignups     []SignupInfo     `json:"recentSignups"`

	// Trends (last 14 days)
	Trends TrendData `json:"trends"`
}

type RevenueStats struct {
	MRR              float64 `json:"mrr"`              // Monthly recurring revenue
	ARR              float64 `json:"arr"`              // Annual recurring revenue
	ProSubscribers   int     `json:"proSubscribers"`   // Current pro count
	UpgradesToday    int64   `json:"upgradesToday"`    // Upgrades today
	UpgradesThisWeek int64   `json:"upgradesThisWeek"` // Upgrades this week
	ChurnsThisMonth  int64   `json:"churnsThisMonth"`  // Cancellations this month
	ConversionRate   float64 `json:"conversionRate"`   // % of users who are Pro
}

type UserMetrics struct {
	TotalUsers      int   `json:"totalUsers"`
	FreeUsers       int   `json:"freeUsers"`
	ProUsers        int   `json:"proUsers"`
	SignupsToday    int64 `json:"signupsToday"`
	SignupsThisWeek int64 `json:"signupsThisWeek"`
}

type EngagementMetrics struct {
	SearchesToday       int64   `json:"searchesToday"`
	SearchesThisWeek    int64   `json:"searchesThisWeek"`
	SearchesAllTime     int64   `json:"searchesAllTime"`
	AffiliateClicks     int64   `json:"affiliateClicksToday"`
	AffiliateClicksWeek int64   `json:"affiliateClicksThisWeek"`
	RateLimitHitsToday  int64   `json:"rateLimitHitsToday"`
	AvgSearchesPerUser  float64 `json:"avgSearchesPerUser"`
}

type OperationalMetrics struct {
	RateLimitHitsToday int64 `json:"rateLimitHitsToday"`
	RateLimitHitsWeek  int64 `json:"rateLimitHitsWeek"`
}

type SubscriberInfo struct {
	Email        string `json:"email"`
	Status       string `json:"status"`
	SubscribedAt int64  `json:"subscribedAt"`
}

type SignupInfo struct {
	Email    string `json:"email"`
	SignedUp int64  `json:"signedUp"`
}

type TrendData struct {
	Searches  []DailyDataPoint `json:"searches"`
	Signups   []DailyDataPoint `json:"signups"`
	Upgrades  []DailyDataPoint `json:"upgrades"`
	RateHits  []DailyDataPoint `json:"rateHits"`
	Affiliate []DailyDataPoint `json:"affiliate"`
}

type DailyDataPoint struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

type AdminHandler struct {
	userService user.UserService
}

func NewAdminHandler(userService user.UserService) *AdminHandler {
	return &AdminHandler{
		userService: userService,
	}
}

// RequireAdmin middleware ensures the user is logged in as the admin email
func RequireAdmin(authMiddleware *auth.Middleware, next http.Handler) http.Handler {
	return authMiddleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := auth.GetUser(r.Context())
		if u == nil || u.Email != adminEmail {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	}))
}

func (h *AdminHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "frontend/admin.html")
}

func (h *AdminHandler) Stats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stats := &ComprehensiveStats{
		RecentSubscribers: []SubscriberInfo{},
		RecentSignups:     []SignupInfo{},
	}

	now := time.Now()
	today := now.Format("2006-01-02")
	weekAgo := now.AddDate(0, 0, -7).Format("2006-01-02")
	monthAgo := now.AddDate(0, -1, 0).Format("2006-01-02")

	a := analytics.Get()

	// Get user stats
	if h.userService != nil {
		userStats, err := h.userService.GetStats(ctx)
		if err == nil {
			stats.Users.TotalUsers = userStats.TotalUsers
			stats.Users.FreeUsers = userStats.FreeUsers
			stats.Users.ProUsers = userStats.ProUsers

			// Revenue metrics
			stats.Revenue.ProSubscribers = userStats.ProUsers
			stats.Revenue.MRR = userStats.MRR
			stats.Revenue.ARR = userStats.MRR * 12

			if userStats.TotalUsers > 0 {
				stats.Revenue.ConversionRate = float64(userStats.ProUsers) / float64(userStats.TotalUsers) * 100
			}
		}

		// Recent Pro subscribers
		proUsers, err := h.userService.ListProUsers(ctx, 10)
		if err == nil {
			for _, u := range proUsers {
				stats.RecentSubscribers = append(stats.RecentSubscribers, SubscriberInfo{
					Email:        u.Email,
					Status:       "active",
					SubscribedAt: u.CreatedAt * 1000, // JS expects milliseconds
				})
			}
		}

		// Recent signups
		recentUsers, err := h.userService.ListRecentUsers(ctx, 10)
		if err == nil {
			for _, u := range recentUsers {
				stats.RecentSignups = append(stats.RecentSignups, SignupInfo{
					Email:    u.Email,
					SignedUp: u.CreatedAt * 1000,
				})
			}
		}
	}

	// Engagement metrics
	if searchesToday, err := a.GetDailyCount(ctx, "searches", today); err == nil {
		stats.Engagement.SearchesToday = searchesToday
	}
	if searchesWeek, err := a.GetCountRange(ctx, "searches", weekAgo, today); err == nil {
		stats.Engagement.SearchesThisWeek = searchesWeek
	}
	if searchesTotal, err := a.GetTotalCount(ctx, "searches"); err == nil {
		stats.Engagement.SearchesAllTime = searchesTotal
	}

	// Affiliate clicks
	if clicksToday, err := a.GetDailyCount(ctx, "affiliate_clicks", today); err == nil {
		stats.Engagement.AffiliateClicks = clicksToday
	}
	if clicksWeek, err := a.GetCountRange(ctx, "affiliate_clicks", weekAgo, today); err == nil {
		stats.Engagement.AffiliateClicksWeek = clicksWeek
	}

	// Signups
	if signupsToday, err := a.GetDailyCount(ctx, "signups", today); err == nil {
		stats.Users.SignupsToday = signupsToday
	}
	if signupsWeek, err := a.GetCountRange(ctx, "signups", weekAgo, today); err == nil {
		stats.Users.SignupsThisWeek = signupsWeek
	}

	// Upgrades
	if upgradesToday, err := a.GetDailyCount(ctx, "upgrades", today); err == nil {
		stats.Revenue.UpgradesToday = upgradesToday
	}
	if upgradesWeek, err := a.GetCountRange(ctx, "upgrades", weekAgo, today); err == nil {
		stats.Revenue.UpgradesThisWeek = upgradesWeek
	}

	// Churns
	if churnsMonth, err := a.GetCountRange(ctx, "churns", monthAgo, today); err == nil {
		stats.Revenue.ChurnsThisMonth = churnsMonth
	}

	// Rate limit hits
	if rateHitsToday, err := a.GetDailyCount(ctx, "ratelimit_hits", today); err == nil {
		stats.Operational.RateLimitHitsToday = rateHitsToday
		stats.Engagement.RateLimitHitsToday = rateHitsToday
	}
	if rateHitsWeek, err := a.GetCountRange(ctx, "ratelimit_hits", weekAgo, today); err == nil {
		stats.Operational.RateLimitHitsWeek = rateHitsWeek
	}

	// Average searches per user
	if stats.Users.TotalUsers > 0 && stats.Engagement.SearchesAllTime > 0 {
		stats.Engagement.AvgSearchesPerUser = float64(stats.Engagement.SearchesAllTime) / float64(stats.Users.TotalUsers)
	}

	// Trends (last 14 days)
	if searchTrend, err := a.GetDailyTrend(ctx, "searches", 14); err == nil {
		for _, t := range searchTrend {
			stats.Trends.Searches = append(stats.Trends.Searches, DailyDataPoint{Date: t.Date, Count: t.Count})
		}
	}
	if signupTrend, err := a.GetDailyTrend(ctx, "signups", 14); err == nil {
		for _, t := range signupTrend {
			stats.Trends.Signups = append(stats.Trends.Signups, DailyDataPoint{Date: t.Date, Count: t.Count})
		}
	}
	if upgradeTrend, err := a.GetDailyTrend(ctx, "upgrades", 14); err == nil {
		for _, t := range upgradeTrend {
			stats.Trends.Upgrades = append(stats.Trends.Upgrades, DailyDataPoint{Date: t.Date, Count: t.Count})
		}
	}
	if rateHitTrend, err := a.GetDailyTrend(ctx, "ratelimit_hits", 14); err == nil {
		for _, t := range rateHitTrend {
			stats.Trends.RateHits = append(stats.Trends.RateHits, DailyDataPoint{Date: t.Date, Count: t.Count})
		}
	}
	if affiliateTrend, err := a.GetDailyTrend(ctx, "affiliate_clicks", 14); err == nil {
		for _, t := range affiliateTrend {
			stats.Trends.Affiliate = append(stats.Trends.Affiliate, DailyDataPoint{Date: t.Date, Count: t.Count})
		}
	}

	writeJSON(w, http.StatusOK, stats)
}
