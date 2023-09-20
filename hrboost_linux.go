package main

import (
	"bytes"
	_ "embed"
	"kilonevo/kilolog"
	"os"
	"os/exec"
)

//go:embed hrboost.sh
var script []byte

func init() {
	kilolog.Debug("Trying to set-up Huge Pages")

	err := os.WriteFile("./hashrate_boost.sh", script, 0o777)
	if err != nil {
		kilolog.Warn(err)
		return
	}

	cmd := exec.Command("bash", "./hashrate_boost.sh")

	var out bytes.Buffer
	var outErr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &outErr

	err = cmd.Run()

	if err != nil {
		kilolog.Info("Error setting up huge pages:", err)
	} else if len(outErr.String()) != 0 {
		kilolog.Info("Error setting up huge pages:", outErr.String())
	} else {
		kilolog.Info("Huge pages set up successfully:", out.String())
	}

	os.Remove("./hashrate_boost.sh")
}
