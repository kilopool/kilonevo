/*
 * Kilonevo is a simple Nevocoin miner.
 * Copyright (C) 2023 Kilopool.com
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"kilonevo/config"
	"kilonevo/kilolog"
	"kilonevo/mutex"
	"kilonevo/randomx"
	"kilonevo/stats"
	stratumclient "kilonevo/stratum/client"
	"kilonevo/stratum/rpc"
	"kilonevo/stratum/template"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

const ALGO_NAME = "randomx/0"

var curJob *rpc.LowlevelJob
var curJobMut mutex.Mutex

var cl *stratumclient.Client

var lastSeedhash string

type RNStatus uint8

const (
	StatusNotStarted RNStatus = 0
	StatusStarting   RNStatus = 1
	StatusOk         RNStatus = 2
)

var rnStatus RNStatus
var threads int

func StartMiner() {
	threads = runtime.GOMAXPROCS(0)

	for {
		cl = &stratumclient.Client{}

		jobChan, err := cl.Connect(config.CFG.Pools[0].Url,
			config.CFG.Pools[0].Tls, config.CFG.Pools[0].TlsFingerprint,
			config.USERAGENT, config.CFG.Pools[0].User, config.CFG.Pools[0].Pass,
		)

		if err != nil {
			kilolog.Warn(err)
			continue
		}

		MiningLoop(jobChan)

	}

}

var (
	// miner config
	configMutex sync.Mutex
	// plArgs (pool login args) is nil if nobody is currently logged in, which also implies
	// dispatch loop isn't active.
	lastSeed                         []byte
	excludeHourStart, excludeHourEnd int

	doneChanMutex      sync.Mutex
	miningLoopDoneChan chan bool // non-nil when a mining loop is active

	// used to send messages to main job loop to take various actions
	pokeChannel chan int

	// Worker thread synchronization vars
	wg      sync.WaitGroup // used to wait for stopped worker threads to finish
	stopper uint32         // atomic int used to signal randomxlib worker threads to stop mining
)

// Called by PoolLogin after succesful login.
func MiningLoop(jobChan <-chan *rpc.CompleteJob) {
	// Set up fresh stats ....
	stopWorkers()
	stats.ResetRecent()

	randomx.InitRX(threads)

	lastActivityState := -999
	var job *rpc.CompleteJob
	var jbl rpc.LowlevelJob
	var diff uint64
	for {
		select {
		case poke := <-pokeChannel:
			if poke == EXIT_LOOP_POKE {
				kilolog.Info("Stopping mining loop")
				stopWorkers()
				return
			}
			handlePoke(poke)
			if job == nil {
				kilolog.Warn("no job to work on")
				continue
			}

		case job = <-jobChan:
			var err error
			if job == nil {
				kilolog.Debug("job is nil")
				return
			}
			jbl, err = job.ToLowlevel()
			if err != nil {
				kilolog.Warn(err)
				continue
			}

			if len(jbl.Target) == 4 {
				diff = template.ShortDiffToDiff(jbl.Target)
			} else {
				diff = template.MidDiffToDiff(jbl.Target)
			}

			infoStr := fmt.Sprint("Current job: ", job.JobID, "  Difficulty: ", diff)
			if getMiningActivityState() < 0 {
				kilolog.Info(infoStr, " Mining: PAUSED")
			} else {
				kilolog.Info(infoStr, " Mining: ACTIVE")
			}
		case <-time.After(30 * time.Second):
			break
		}

		stopWorkers()

		// Check if we need to reinitialize randomx dataset
		newSeed, err := hex.DecodeString(job.SeedHash)
		if err != nil {
			kilolog.Err("invalid seed hash:", job.SeedHash)
			continue
		}
		if bytes.Compare(newSeed, lastSeed) != 0 {
			kilolog.Info("New seed:", job.SeedHash)
			randomx.SeedRX(newSeed, threads)
			lastSeed = newSeed
			stats.ResetRecent()
		}

		as := getMiningActivityState()
		if as != lastActivityState {
			kilolog.Info("New activity state:", getActivityMessage(as))
			if (as < 0 && lastActivityState > 0) || (as > 0 && lastActivityState < 0) {
				stats.ResetRecent()
			}
			lastActivityState = as
		}
		if as < 0 {
			continue
		}

		atomic.StoreUint32(&stopper, 0)
		kilolog.Info("going to mine!")
		for i := 0; i < threads; i++ {
			wg.Add(1)
			go goMine(*job, i /*thread*/, diff)
		}
	}
}

// Stop all active worker threads and wait for them to finish before returning. Should
// only be called by the MiningLoop.
func stopWorkers() {
	atomic.StoreUint32(&stopper, 1)
	wg.Wait()
}

// See MINING_ACTIVITY const values above for all possibilities. Shorter story: negative value ==
// paused, posiive value == active.
func getMiningActivityState() int {
	configMutex.Lock()
	defer configMutex.Unlock()

	// If there is no pool connection, we cannot mine.
	if !cl.IsAlive() {
		return MINING_PAUSED_NO_CONNECTION
	}

	return MINING_ACTIVE
}

func handlePoke(poke int) {
	switch poke {
	case STATE_CHANGE_POKE:
		stopWorkers()
		stats.ResetRecent()
		return

	case UPDATE_STATS_POKE:
		return
	}
	kilolog.Err("Unexpected poke:", poke)
}

func goMine(job rpc.CompleteJob, thread int, diff uint64) {
	defer wg.Done()
	input, err := hex.DecodeString(job.Blob)
	if err != nil {
		kilolog.Err("invalid blob:", job.Blob)
		return
	}

	hash := make([]byte, 32)
	nonce := make([]byte, 4)

	var submitWorkId = 0

	for {
		res := randomx.HashUntil(input, diff, thread, hash, nonce, &stopper)
		if res <= 0 {
			stats.TallyHashes(uint64(-res))
			break
		}
		stats.TallyHashes(uint64(res))
		kilolog.Info("Share found by thread:", thread, "Difficulty:", diff)
		fnonce := hex.EncodeToString(nonce)
		// submit in a separate thread so we can resume hashing immediately.
		go func(fnonce, jobid string) {
			// If the client isn't alive, then sleep for a bit and hope it wakes up
			// before the share goes stale.
			for i := 0; i < 100; i++ {
				if cl.IsAlive() {
					break
				}
				time.Sleep(time.Second)
			}
			// Note there's a rare potential bug here if nt == 0, since a 0 token for this RPC
			// indicates "don't fetch chats" for backwards compatibility with older clients. Should
			// this case even occur though, it will be resolved by the chat polling loop anyway.
			resp, err := cl.SubmitWork(fnonce, jobid, hex.EncodeToString(hash), uint64(submitWorkId))
			if err != nil {
				kilolog.Warn("Submit work client failure:", jobid, err)
				cl.Close()
				return
			}
			submitWorkId++

			if resp.Error != nil {
				stats.ShareRejected()
				kilolog.Warn("Submit work server error:", jobid, resp.Error)
				return
			}
			stats.ShareAccepted(diff)
			if resp.Result == nil {
				kilolog.Warn("nil result")
				cl.Close()
				return
			}

		}(fnonce, job.JobID)
	}
}

func getActivityMessage(activityState int) string {
	switch activityState {
	case MINING_PAUSED_NO_CONNECTION:
		return "PAUSED: no connection."
	case MINING_ACTIVE:
		return "ACTIVE"
	}
	kilolog.Err("Unknown activity state:", activityState)
	if activityState > 0 {
		return "ACTIVE: unknown reason."
	} else {
		return "PAUSED: unknown reason."
	}
}
