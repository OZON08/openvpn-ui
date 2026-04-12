package state

import (
	"sync"

	"github.com/OZON08/openvpn-ui/models"
)

var (
	GlobalCfg   models.Settings
	GlobalCfgMu sync.RWMutex
	// NOTE: GlobalCfg has many read sites across controllers and lib packages.
	// Only the write in settings.go is mutex-protected for now; adding RLock
	// to every read site would require significant refactoring.
)
