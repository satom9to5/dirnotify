package dirnotify

import (
	"testing"
)

func TestEventQueue(t *testing.T) {
	eqs := createTestEventQueues()

	// sort check
	eqs.Sort()

	eqPatterns := eventQueues{
		eventQueue{Op: Create, dir: "/usr/bin", base: "more"},
		eventQueue{Op: Remove, dir: "/usr/bin", base: "less"},
		eventQueue{Op: Rename, dir: "/usr/bin", base: "more"},
		eventQueue{Op: Rename, dir: "/usr/sbin", base: "more"},
		eventQueue{Op: Create, dir: "/usr/bar/bin", base: "more"},
		eventQueue{Op: Remove, dir: "/usr/foo/bin", base: "less"},
	}

	for i, eqPt := range eqPatterns {
		if eqPt != eqs[i] {
			t.Fatalf("[TestEventQueue] eventQueue is different: %s : %s", eqPt, eqs[i])
		}
	}

	// rename check
	eqs.Rename("/usr/bin", "/var/bin")

	renamedDirs := []string{
		"/usr/sbin",
		"/var/bin",
		"/var/bin",
		"/var/bin",
		"/usr/bar/bin",
		"/usr/foo/bin",
	}

	for i, dir := range renamedDirs {
		if dir != eqs[i].dir {
			t.Fatalf("[TestEventQueue] dir is different: %s : %s", dir, eqs[i].dir)
		}
	}
}

func createTestEventQueues() eventQueues {
	return eventQueues{
		eventQueue{Op: Rename, dir: "/usr/sbin", base: "more"},
		eventQueue{Op: Remove, dir: "/usr/foo/bin", base: "less"},
		eventQueue{Op: Create, dir: "/usr/bar/bin", base: "more"},
		eventQueue{Op: Rename, dir: "/usr/bin", base: "more"},
		eventQueue{Op: Remove, dir: "/usr/bin", base: "less"},
		eventQueue{Op: Create, dir: "/usr/bin", base: "more"},
	}
}
