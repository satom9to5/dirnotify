package dirnotify

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type watchTestManipulation struct {
	Op
	isDir  bool
	dir    string
	name   string
	toDir  string // for rename
	toName string // for rename
	size   int
}

type watchTestPattern struct {
	Op
	absPath    string
	beforePath string
}

func SubTestWatch(t *testing.T) {
	var events []Event
	var testPatterns []watchTestPattern
	var err error

	doneCh := make(chan bool)

	writeFile := func(p string, size int) {
		fp, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			t.Fatalf("[SubTestWatch] failed to open file: %s", err)
		}
		defer fp.Close()

		writer := bufio.NewWriter(fp)
		// append header size
		if _, err := writer.WriteString(makeTestDummyString(size + 4096)); err != nil {
			t.Fatalf("[SubTestWatch] failed to write file: %s", err)
		}
	}

	// emulate user manipulate function
	manipulate := func(manipulations []watchTestManipulation) {
		for _, m := range manipulations {
			absPath := filepath.Join(_dir, filepath.FromSlash(m.dir), m.name)

			switch m.Op {
			case Create:
				if m.isDir {
					if err = os.MkdirAll(absPath, 0777); err != nil {
						t.Fatalf("[SubTestWatch] failed to create directory: %s", err)
					}
				} else {
					if m.size > 0 {
						writeFile(absPath, m.size)
					} else {
						if f, err := os.Create(absPath); err != nil {
							t.Fatalf("[SubTestWatch] failed to create file: %s", err)
						} else {
							f.Close()
						}
					}
				}
			case Rename:
				renamedPath := filepath.Join(_dir, filepath.FromSlash(m.toDir), m.toName)
				if err = os.Rename(absPath, renamedPath); err != nil {
					t.Fatalf("[SubTestWatch] failed to rename: %s", err)
				}
			case Remove:
				if m.isDir {
					if err = os.RemoveAll(absPath); err != nil {
						t.Fatalf("[SubTestWatch] failed to remove directory: %s", err)
					}
				} else {
					if err = os.Remove(absPath); err != nil {
						t.Fatalf("[SubTestWatch] failed to remove file: %s", err)
					}
				}
			case Write:
				writeFile(absPath, m.size)
			}
		}

		// wait send events.
		for {
			time.Sleep(2 * time.Second)

			if len(*(_root.queues)) == 0 && len(*(_root.writeNodes)) == 0 {
				break
			}
		}

		doneCh <- true
	}

	// wait send events function
	eventWatch := func() []Event {
		events := []Event{}

		finished := false
		// create new file
		//for !finished {
		for !finished {
			select {
			case e := <-_root.Ch:
				events = append(events, e)
			case <-time.After(3 * time.Second):
				t.Fatalf("[SubTestWatch] too long to wait for event.")
			case <-doneCh:
				if len(*(_root.queues)) == 0 {
					finished = true
					break
				}
			}
		}

		return events
	}

	// test create new file/directory
	go manipulate([]watchTestManipulation{
		{Create, true, "opt/etc", "httpd", "", "", 0},
		{Create, false, "usr/local/bin", "more.exe", "", "", 0},
		{Create, false, "opt/etc/httpd", "httpd.conf", "", "", 8192}, // until watcher.Add parent directory
	})

	events = eventWatch()

	testPatterns = []watchTestPattern{
		{Create, "opt", ""},
		{Create, "opt/etc", ""},
		{Create, "opt/etc/httpd", ""},
		{Create, "opt/etc/httpd/httpd.conf", ""},
		{Create, "usr/local/bin/more.exe", ""},
		{WriteComplete, "opt/etc/httpd/httpd.conf", ""},
	}

	if err = testEventLength(events, testPatterns); err != nil {
		t.Fatalf("[SubTestWatch] event length is different: %s", err)
	}

	for i, event := range events {
		pattern := testPatterns[i]

		if err = testEvent(event, pattern); err != nil {
			t.Fatalf("[SubTestWatch] event info is different: %s", err)
		}
	}

	// test rename&move&remove file/directory
	go manipulate([]watchTestManipulation{
		{Remove, true, "opt/etc", "httpd", "", "", 0},
		{Remove, false, "usr/bin", "cat.exe", "", "", 0},
		{Rename, false, "usr/local/bin", "more.exe", "usr/local/bin", "less.exe", 0},
		{Rename, false, "usr/bin", "ls.exe", "usr/local/bin", "ls.exe", 0},
		{Rename, true, "usr/local", "etc", "usr", "etc", 0},
		{Create, false, "opt/etc/", "resolve.conf", "", "", 8192},
	})

	events = eventWatch()

	testPatterns = []watchTestPattern{
		{Move, "usr/etc", "usr/local/etc"},
		{Create, "opt/etc/resolve.conf", ""},
		{Remove, "opt/etc/httpd", ""},
		{Remove, "usr/bin/cat.exe", ""},
		{Move, "usr/local/bin/ls.exe", "usr/bin/ls.exe"},
		{Remove, "opt/etc/httpd/httpd.conf", ""},
		{Move, "usr/local/bin/less.exe", "usr/local/bin/more.exe"},
		{WriteComplete, "opt/etc/resolve.conf", ""},
	}

	if err = testEventLength(events, testPatterns); err != nil {
		t.Fatalf("[SubTestWatch] event length is different: %s", err)
	}

	for i, event := range events {
		pattern := testPatterns[i]

		if err = testEvent(event, pattern); err != nil {
			t.Fatalf("[SubTestWatch] event info is different: %s", err)
		}
	}

	// test renamed directory event
	go manipulate([]watchTestManipulation{
		{Create, false, "usr/etc", "hosts.conf", "", "", 0},
		{Write, false, "usr/local/bin", "ls.exe", "", "", 16384},
		{Create, false, "usr/bin", "grep.exe", "", "", 0},
		{Write, false, "usr/bin", "grep.exe", "", "", 1024000},
	})

	events = eventWatch()

	testPatterns = []watchTestPattern{
		{Create, "usr/bin/grep.exe", ""},
		{Create, "usr/etc/hosts.conf", ""},
		{WriteComplete, "usr/bin/grep.exe", ""},
		{WriteComplete, "usr/local/bin/ls.exe", ""},
	}

	if err = testEventLength(events, testPatterns); err != nil {
		t.Fatalf("[SubTestWatch] event length is different: %s", err)
	}

	for i, event := range events {
		pattern := testPatterns[i]

		if err = testEvent(event, pattern); err != nil {
			t.Fatalf("[SubTestWatch] event info is different: %s", err)
		}
	}

	testNodes(t, _root.node)
}

func testEventLength(events []Event, patterns []watchTestPattern) error {
	if len(events) != len(patterns) {
		return errors.New(fmt.Sprintf("length is different. expect: %d, fact: %d", len(patterns), len(events)))
	} else {
		return nil
	}
}

func testEvent(e Event, pattern watchTestPattern) error {
	absPath := filepath.Join(_dir, filepath.FromSlash(pattern.absPath))
	beforePath := filepath.Join(_dir, filepath.FromSlash(pattern.beforePath))

	if e.Op() != pattern.Op {
		return errors.New(fmt.Sprintf("Op is different. expect: %d, fact: %d, path: %s", pattern.Op, e.Op(), e.Path))
	}

	if e.Path() != absPath {
		return errors.New(fmt.Sprintf("Path is different. expect: %s, fact: %s", absPath, e.Path()))
	}

	if e.BeforePath() != "" && e.BeforePath() != beforePath {
		return errors.New(fmt.Sprintf("beforePath is different. expect: %s, fact: %s", beforePath, e.BeforePath()))
	}

	return nil
}

func makeTestDummyString(length int) string {
	n := make([]byte, length)

	return string(n)
}
