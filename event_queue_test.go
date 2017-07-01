package dirnotify

import (
	"path/filepath"
	"testing"
)

func TestEventQueue(t *testing.T) {
	eqs := createTestEventQueues()

	// sort check
	eqs.sort()

	eqPatterns := eventQueues{
		eventQueue{Op: Create, dir: filepath.FromSlash("/usr/bin"), base: "more"},
		eventQueue{Op: Remove, dir: filepath.FromSlash("/usr/bin"), base: "less"},
		eventQueue{Op: Rename, dir: filepath.FromSlash("/usr/bin"), base: "more"},
		eventQueue{Op: Rename, dir: filepath.FromSlash("/usr/sbin"), base: "more"},
		eventQueue{Op: Create, dir: filepath.FromSlash("/usr/bar/bin"), base: "more"},
		eventQueue{Op: Remove, dir: filepath.FromSlash("/usr/foo/bin"), base: "less"},
	}

	for i, eqPt := range eqPatterns {
		if eqPt != eqs[i] {
			t.Fatalf("[TestEventQueue] eventQueue is different: %s : %s", eqPt, eqs[i])
		}
	}

	// rename check
	eqs.rename(filepath.FromSlash("/usr/bin"), filepath.FromSlash("/var/bin"))

	renamedDirs := []string{
		"/usr/sbin",
		"/var/bin",
		"/var/bin",
		"/var/bin",
		"/usr/bar/bin",
		"/usr/foo/bin",
	}

	for i, dir := range renamedDirs {
		dir = filepath.FromSlash(dir)
		if dir != eqs[i].dir {
			t.Fatalf("[TestEventQueue] dir is different: %s : %s", dir, eqs[i].dir)
		}
	}
}

func createTestEventQueues() eventQueues {
	return eventQueues{
		eventQueue{Op: Rename, dir: filepath.FromSlash("/usr/sbin"), base: "more"},
		eventQueue{Op: Remove, dir: filepath.FromSlash("/usr/foo/bin"), base: "less"},
		eventQueue{Op: Create, dir: filepath.FromSlash("/usr/bar/bin"), base: "more"},
		eventQueue{Op: Rename, dir: filepath.FromSlash("/usr/bin"), base: "more"},
		eventQueue{Op: Remove, dir: filepath.FromSlash("/usr/bin"), base: "less"},
		eventQueue{Op: Create, dir: filepath.FromSlash("/usr/bin"), base: "more"},
	}
}
