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

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
)

type Job struct {
	Blob   string `json:"blob"`
	JobID  string `json:"job_id"`
	Target string `json:"target"`
	Algo   string `json:"algo"`
}

type RXJob struct {
	Job
	Height   uint64 `json:"height,omitempty"`
	SeedHash string `json:"seed_hash,omitempty"`
}

type CompleteJob struct {
	RXJob
}

type LowlevelJob struct {
	Blob     []byte
	JobID    string
	Target   []byte
	Height   uint64
	SeedHash []byte
	Algo     string
}

func (j *CompleteJob) ToLowlevel() (LowlevelJob, error) {
	var l LowlevelJob
	var err error

	l.Algo = j.Algo
	l.JobID = j.JobID
	l.Height = j.Height

	l.Blob, err = hex.DecodeString(j.Blob)
	if err != nil {
		return l, err
	}

	l.SeedHash, err = hex.DecodeString(j.SeedHash)
	if err != nil {
		return l, err
	}

	target, err := hex.DecodeString(j.Target)
	if err != nil {
		return l, err
	}
	if len(target) != 4 && len(target) != 8 {
		return l, fmt.Errorf("target length %d not supported", len(target))
	}
	l.Target = target

	return l, nil
}

type LoginResponse struct {
	ID      uint64 `json:"id"`
	Jsonrpc string `json:"jsonrpc"`
	Result  *struct {
		ID  string       `json:"id"`
		Job *CompleteJob `job:"job"`
	} `json:"result,omitempty"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type JobRpc struct {
	Jsonrpc string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  CompleteJob `json:"params"`
}

type Response struct {
	ID      uint64 `json:"id"`
	Jsonrpc string `json:"jsonrpc,omitempty"`
	Method  string `json:"method,omitempty"`
	Status  string `json:"status,omitempty"`

	Job    *CompleteJob     `json:"params,omitempty"` // used to send jobs over the connection
	Result *json.RawMessage `json:"result,omitempty"` // used to return SubmitWork results

	Error any `json:"error,omitempty"`
}
