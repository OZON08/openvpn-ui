package state

import (
	"sync"

	"github.com/OZON08/openvpn-ui/lib/monitor"
	"github.com/OZON08/openvpn-ui/models"
)

var (
	GlobalCfg   models.Settings
	GlobalCfgMu sync.RWMutex
	// NOTE: GlobalCfg has many read sites across controllers and lib packages.
	// Only the write in settings.go is mutex-protected for now; adding RLock
	// to every read site would require significant refactoring.

	// Monitor is the live scraper instance. Nil when monitoring is disabled;
	// controllers must nil-check before dereferencing.
	Monitor *monitor.Scraper
)
