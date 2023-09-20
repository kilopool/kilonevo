package stats

import (
	"kilonevo/config"
	"kilonevo/kilolog"
	"strconv"
	"sync"
	"time"
)

var (
	mutex sync.RWMutex

	lastTally       time.Time
	lastTallyHashes float64

	sharesAccepted   uint64
	sharesRejected   uint64
	poolSideHashes   uint64
	clientSideHashes uint64

	lastHr float64
)

func GetHashrate() float64 {
	mutex.RLock()
	defer mutex.RUnlock()
	return lastHr
}

func Init() {
	mutex.Lock()
	defer mutex.Unlock()
	now := time.Now()
	lastTally = now
}

func TallyHashes(hashes uint64) {
	mutex.Lock()
	defer mutex.Unlock()
	clientSideHashes += hashes

	seconds := time.Now().Sub(lastTally).Seconds()

	lastTallyHashes += float64(hashes)

	kilolog.Debug("seconds are", seconds)
	if seconds > float64(config.CFG.PrintInterval) {
		lastHr = float64(lastTallyHashes) / seconds
		kilolog.Statsf("Hashrate is now: %sH/s", fmtHash(lastHr))
		lastTally = time.Now()
		lastTallyHashes = 0
	}
}

func ShareAccepted(diffTarget uint64) {
	mutex.Lock()
	defer mutex.Unlock()
	sharesAccepted++
	poolSideHashes += diffTarget
}

func ShareRejected() {
	mutex.Lock()
	defer mutex.Unlock()
	sharesRejected++
}

// Call every time an event happens that may induce a big change in hashrate, e.g. reseeding,
// adding/removing threads, restablishing a connection. Make sure all workers are stopped before
// calling otherwise hashrate will turn out inaccurate.
func ResetRecent() {
	mutex.Lock()
	defer mutex.Unlock()
	now := time.Now()
	lastTally = now
	lastTallyHashes = 0
}

func fmtHash(a float64) string {
	if a > 1000000 {
		return strconv.FormatFloat(a/1000000, 'f', 2, 64) + " M"
	} else if a > 1000 {
		return strconv.FormatFloat(a/1000, 'f', 2, 64) + " k"
	} else {
		return strconv.FormatFloat(a, 'f', 0, 64) + " "
	}
}
