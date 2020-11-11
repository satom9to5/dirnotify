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

func (eq eventQueue) String() string {
	result := fmt.Sprintf("Op: %s, dir: %s, base: %s", flagString(eq.Op), eq.dir, eq.base)

	if eq.node != nil {
		result += fmt.Sprintf(", ino: %d", eq.node.Ino())
	}

	return result
}

func (eq *eventQueue) Path() string {
	return eq.dir + fileinfo.PathSep + eq.base
}

func (eqs *eventQueues) clear() {
	*eqs = eventQueues{}
}

func (eqs *eventQueues) add(e fsnotify.Event, r *Root) {
	eq := eventQueue{}
	eq.dir, eq.base = fileinfo.Split(e.Name)

	switch true {
	case e.Op&fsnotify.Write == fsnotify.Write:
		eq.Op |= Write
	case e.Op&fsnotify.Create == fsnotify.Create:
		eq.Op |= Create
	case e.Op&fsnotify.Remove == fsnotify.Remove:
		eq.Op |= Remove
	case e.Op&fsnotify.Rename == fsnotify.Rename:
		eq.Op |= Rename
	case e.Op&fsnotify.Chmod == fsnotify.Chmod:
		eq.Op |= Chmod
	}

	// exist nodes except Create event.
	if n, err := r.Find(e.Name); err == nil {
		eq.node = n
	}

	*eqs = append(*eqs, eq)
}

func (eqs *eventQueues) addFromNodes(nodes []*Node) {
	for _, node := range nodes {
		eqs.addFromNode(node, Create)
	}

	eqs.sort()
}

func (eqs *eventQueues) addFromNode(n *Node, op Op) {
	eq := eventQueue{
		Op:   op,
		dir:  n.Dir(),
		base: n.Name(),
		node: n,
	}

	*eqs = append(*eqs, eq)
}

func (eqs *eventQueues) sort() {
	sort.Sort(eqs)
}

func (eqs *eventQueues) createNodeEvents(r *Root) (*nodeEvents, error) {
	nes := &nodeEvents{}

	for {
		if len(*eqs) == 0 {
			break
		}

		eq := (*eqs)[0]
		*eqs = (*eqs)[1:]

		if err := nes.add(eq, eqs, r); err != nil {
			return nil, err
		}
	}

	// check Move event.
	nes.updateOp()

	return nes, nil
}

// Sort Interface
func (eqs *eventQueues) Len() int {
	return len(*eqs)
}

// Sort Interface
func (eqs *eventQueues) Swap(i, j int) {
	(*eqs)[i], (*eqs)[j] = (*eqs)[j], (*eqs)[i]
}

// Sort Interface
func (eqs *eventQueues) Less(i, j int) bool {
	ie, je := (*eqs)[i], (*eqs)[j]

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
func (eqs *eventQueues) rename(from, to string) {
	for i, eq := range *eqs {
		(*eqs)[i].dir = strings.Replace(eq.dir, from, to, -1)
	}

	eqs.sort()
}
