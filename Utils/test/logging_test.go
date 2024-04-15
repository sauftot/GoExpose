package test

import (
	"Utils"
	"testing"
)

func TestSetupLoggerWriter(t *testing.T) {
	t.Log("Testing SetupLoggerWriter...")
	dir := t.TempDir()
	t.Log("TempDir: ", dir)
	Utils.SetupLoggerWriter(dir, "client", false)
	t.Log("SetupLoggerWriter test done")
}
