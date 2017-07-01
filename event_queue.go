package dirnotify

import (
	"fmt"
	"sort"
	"strings"
	// third party
	"github.com/satom9to5/fileinfo"
	"github.com/satom9to5/fsnotify"
)

type eventQueue struct {
	Op   Op
	dir  string // parent directory path
	base string // target file or directory
	node *Node
}

type eventQueues []eventQueue

func (q eventQueue) String() string {
	result := fmt.Sprintf("Op: %s, dir: %s, base: %s", flagString(q.Op), q.dir, q.base)

	if q.node != nil {
		result += fmt.Sprint(", ino: %d", q.node.Ino())
	}

	return result
}

func (q *eventQueue) Path() string {
	return q.dir + fileinfo.PathSep + q.base
}

func (eq *eventQueues) clear() {
	*eq = eventQueues{}
}

func (eq *eventQueues) add(e fsnotify.Event, r *Root) {
	q := eventQueue{}
	q.dir, q.base = fileinfo.Split(e.Name)

	switch true {
	case e.Op&fsnotify.Write == fsnotify.Write:
		q.Op |= Write
	case e.Op&fsnotify.Create == fsnotify.Create:
		q.Op |= Create
	case e.Op&fsnotify.Remove == fsnotify.Remove:
		q.Op |= Remove
	case e.Op&fsnotify.Rename == fsnotify.Rename:
		q.Op |= Rename
	case e.Op&fsnotify.Chmod == fsnotify.Chmod:
		q.Op |= Chmod
	}

	// exist nodes except Create event.
	if n, err := r.Find(e.Name); err == nil {
		q.node = n
	}

	*eq = append(*eq, q)
}

func (eq *eventQueues) addFromNodes(nodes []*Node) {
	for _, node := range nodes {
		q := eventQueue{
			Op:   Create,
			dir:  node.Dir(),
			base: node.Name(),
			node: node,
		}

		*eq = append(*eq, q)
	}

	eq.sort()
}

func (eq *eventQueues) sort() {
	sort.Sort(eq)
}

func (eq *eventQueues) createNodeEvents(r *Root) (*nodeEvents, error) {
	nes := &nodeEvents{}

	for {
		if len(*eq) == 0 {
			break
		}

		q := (*eq)[0]
		*eq = (*eq)[1:]

		if err := nes.add(q, eq, r); err != nil {
			return nil, err
		}
	}

	// check Move event.
	nes.updateOp()

	return nes, nil
}

// Sort Interface
func (eq *eventQueues) Len() int {
	return len(*eq)
}

// Sort Interface
func (eq *eventQueues) Swap(i, j int) {
	(*eq)[i], (*eq)[j] = (*eq)[j], (*eq)[i]
}

// Sort Interface
func (eq *eventQueues) Less(i, j int) bool {
	ie, je := (*eq)[i], (*eq)[j]

	if ie.dir == je.dir {
		if ie.Op == je.Op {
			return ie.base < je.base
		} else {
			return ie.Op < je.Op
		}
	} else {
		iePath, jePath := ie.Path(), je.Path()
		ieDepth, jeDepth := len(strings.Split(iePath, fileinfo.PathSep)), len(strings.Split(jePath, fileinfo.PathSep))

		if ieDepth == jeDepth {
			return iePath < jePath
		} else {
			return ieDepth < jeDepth
		}
	}
}

// rename path
func (eq *eventQueues) rename(from, to string) {
	for i, q := range *eq {
		(*eq)[i].dir = strings.Replace(q.dir, from, to, -1)
	}

	eq.sort()
}
