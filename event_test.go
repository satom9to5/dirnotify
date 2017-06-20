package dirnotify

import (
	"bufio"
	"errors"
	"fmt"
	"os"
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

	// emulate user manipulate function
	manipulate := func(manipulations []watchTestManipulation) {
		for _, m := range manipulations {
			absPath := _dir + "/" + m.dir + "/" + m.name

			switch m.Op {
			case Create:
				if m.isDir {
					if err = os.MkdirAll(absPath, 0777); err != nil {
						t.Fatalf("[SubTestWatch] failed to create directory: %s", err)
					}
				} else {
					if _, err = os.Create(absPath); err != nil {
						t.Fatalf("[SubTestWatch] failed to create file: %s", err)
					}
				}
			case Rename:
				renamedPath := _dir + "/" + m.toDir + "/" + m.toName
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
				fp, err := os.OpenFile(absPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
				if err != nil {
					t.Fatalf("[SubTestWatch] failed to open file: %s", err)
				}

				writer := bufio.NewWriter(fp)
				if _, err := writer.WriteString(makeTestDummyString(1048576)); err != nil {
					t.Fatalf("[SubTestWatch] failed to write file: %s", err)
				}

			}
		}

		// wait send events.
		for {
			time.Sleep(2 * time.Second)

			if len(*(_root.queues)) == 0 {
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
			case e := <-_root.ch:
				events = append(events, e)
			case <-time.After(2 * time.Second):
				t.Fatalf("[SubTestWatch] too long to wait for event.")
			case <-doneCh:
				if len(*(_root.queues)) == 0 {
					finished = true
					break
				}
			}
		}

		//fmt.Println(events)

		return events
	}

	// test create new file/directory
	go manipulate([]watchTestManipulation{
		{Create, true, "opt/etc", "httpd", "", ""},
		{Create, false, "usr/local/bin", "more.exe", "", ""},
		{Create, false, "opt/etc/httpd", "httpd.conf", "", ""},
	})

	events = eventWatch()

	testPatterns = []watchTestPattern{
		{Create, "opt", ""},
		{Create, "opt/etc", ""},
		{Create, "opt/etc/httpd", ""},
		{Create, "opt/etc/httpd/httpd.conf", ""},
		{Create, "usr/local/bin/more.exe", ""},
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
		{Remove, true, "opt/etc", "httpd", "", ""},
		{Remove, false, "usr/bin", "cat.exe", "", ""},
		{Rename, false, "usr/local/bin", "more.exe", "usr/local/bin", "less.exe"},
		{Rename, false, "usr/bin", "ls.exe", "usr/local/bin", "ls.exe"},
		{Rename, true, "usr/local", "etc", "usr", "etc"},
		{Create, false, "opt/etc/", "resolve.conf", "", ""},
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
		{Create, false, "usr/etc", "hosts.conf", "", ""},
	})

	events = eventWatch()

	testPatterns = []watchTestPattern{
		{Create, "usr/etc/hosts.conf", ""},
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

func testEventLength(es []Event, patterns []watchTestPattern) error {
	if len(es) != len(patterns) {
		return errors.New(fmt.Sprintf("length is different. expect: %d, fact: %d", len(patterns), len(es)))
	} else {
		return nil
	}
}

func testEvent(event Event, pattern watchTestPattern) error {
	p, err := event.Path()
	if err != nil {
		return err
	}

	absPath := _dir + "/" + pattern.absPath
	beforePath := _dir + "/" + pattern.beforePath

	if event.Op != pattern.Op {
		return errors.New(fmt.Sprintf("Op is different. expect: %d, fact: %d", pattern.Op, event.Op))
	}

	if p != absPath {
		return errors.New(fmt.Sprintf("Path is different. expect: %s, fact: %s", absPath, p))
	}

	if event.beforePath != "" && event.beforePath != beforePath {
		return errors.New(fmt.Sprintf("beforePath is different. expect: %s, fact: %s", beforePath, event.beforePath))
	}

	return nil
}

func makeTestDummyString(length int) string {
	n := make([]byte, length)

	return string(n)
}
