package stats

import "testing"

func TestTaskColor(t *testing.T) {
	if taskColor("OCP Java") == taskColor("Other") {
		t.Fatal("different task names should have different colors")
	}
	if taskColor("OCP Java") != taskColor("OCP Java") {
		t.Fatal("task colors should be stable")
	}
}
