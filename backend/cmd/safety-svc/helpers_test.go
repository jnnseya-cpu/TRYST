package main

import (
	"os"
	"os/exec"
	"testing"
)

func testCommand(t *testing.T, env []string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=^TestProductionRefusesUnsafeConfig$")
	cmd.Env = append([]string{"PATH=" + os.Getenv("PATH")}, env...)
	return cmd
}
