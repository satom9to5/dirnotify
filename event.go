package dirnotify

import (
	"fmt"
	"strings"
	"time"
)

const (
	Create Op = 1 << iota
	Remove
	Rename
	Write
	Chmod
	Move
	WriteComplete
)

type Op uint32

type Event struct {
	op         Op
	path       string
	beforePath string
	size       int64
	modTime    time.Time
	isDir      bool
}

func newEvent(ne nodeEvent) Event {
	node := ne.node
	if node == nil {
		return Event{}
	}

	return Event{
		op:         ne.Op,
		path:       node.Path(),
		beforePath: ne.beforePath,
		size:       node.Size(),
		modTime:    node.ModTime(),
		isDir:      node.IsDir(),
	}
}

func newEventByOpNode(op Op, node *Node) Event {
	return Event{
		op:      op,
		path:    node.Path(),
		size:    node.Size(),
		modTime: node.ModTime(),
		isDir:   node.IsDir(),
	}
}

var (
	flagList = []struct {
		Op
		name string
	}{
		{Create, "Create"},
		{Remove, "Remove"},
		{Rename, "Rename"},
		{Write, "Write"},
		{Chmod, "Chmod"},
		{Move, "Move"},
		{WriteComplete, "WriteComplete"},
	}
)

func flagString(op Op) string {
	flags := []string{}

	for _, f := range flagList {
		if op&f.Op == f.Op {
			flags = append(flags, f.name)
		}
	}

	return strings.Join(flags, "|")
}

func (e Event) String() string {
	t := "file"
	if e.isDir {
		t = "directory"
	}

	str := fmt.Sprintf("Op: %s, Path: %s, ", flagString(e.op), e.path)
	if e.beforePath != "" {
		str += fmt.Sprintf("BeforePath: %s, ", e.beforePath)
	}
	return str + fmt.Sprintf("Size: %d, ModTime: %s, Type: %s", e.size, e.modTime.String(), t)
}

func (e Event) Op() Op {
	return e.op
}

func (e Event) Path() string {
	return e.path
}

func (e Event) BeforePath() string {
	return e.beforePath
}

func (e Event) Size() int64 {
	return e.size
}

func (e Event) ModTime() time.Time {
	return e.modTime
}

func (e Event) IsDir() bool {
	return e.isDir
}
