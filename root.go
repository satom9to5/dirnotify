package dirnotify

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"sync"
	"time"
	// third party
	"github.com/satom9to5/fileinfo"
	"github.com/satom9to5/fsnotify"
)

type Root struct {
	root       *Node        // root node
	nodeMap    *NodeMap     // inode key
	queues     *eventQueues // event queue
	writeNodes *NodeMap     // nodes for check write event
	watcher    *fsnotify.Watcher
	Ch         chan Event
	ticker     *time.Ticker
	chkTicker  *time.Ticker // for check directory
	mu         sync.Mutex
	wg         sync.WaitGroup
}

func NewRoot(dirs []string) (*Root, error) {
	dir := dirs[0] // temporary
	fi, err := fileinfo.Stat(dir)
	if err != nil {
		return nil, err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	// root node
	rn := &Node{
		info:  fi,
		dirs:  map[string]*Node{},
		files: map[string]*Node{},
	}

	r := &Root{
		root:       rn,
		nodeMap:    &NodeMap{},
		queues:     &eventQueues{},
		writeNodes: &NodeMap{},
		watcher:    watcher,
		Ch:         make(chan Event),
	}

	// watcher add
	r.addNode(rn)

	return r, nil
}

func CreateNodeTree(dirs []string) (*Root, error) {
	r, err := NewRoot(dirs)
	if err != nil {
		return nil, err
	}

	if err := r.appendNodes(r.root); err != nil {
		return nil, err
	}

	return r, nil
}

// recursive call
func (r *Root) appendNodes(n *Node) error {
	fis, err := ioutil.ReadDir(n.Path())

	if err != nil {
		return err
	}

	for _, fi := range fis {
		chn, err := r.createAddNode(n.Path() + fileinfo.PathSep + fi.Name())
		if err != nil {
			return err
		}

		if !chn.IsDir() {
			continue
		}

		if err := r.appendNodes(chn); err != nil {
			return err
		}
	}

	return nil
}

func (r *Root) PrintTree() string {
	return r.root.PrintTree()
}

// watcher Close
func (r *Root) Close() {
	if r.watcher != nil {
		r.watcher.Close()
	}
}

type walkFunc func(fi fileinfo.FileInfo) error

func (r *Root) Walk(fn walkFunc) error {
	return r.root.walk(fn)
}

func (r *Root) addNode(n *Node) error {
	if err := r.nodeMap.add(n); err != nil {
		return err
	}

	// watcher add when directory
	if n.IsDir() {
		if err := r.watcher.Add(n.Path()); err != nil {
			if debug {
				log.Printf("[Root/RenameNode] watcher Add path: %s, error: %s\n", n.Path(), err)
			}

			return err
		}
	}

	return nil
}

func (r *Root) createAddNode(p string) (*Node, error) {
	paths := fileinfo.SplitPath(p, r.root.Dir())
	if len(paths) == 0 {
		return nil, errors.New("[Root/createAddNode] error: paths length is 0.")
	}

	pathsLen := len(paths)

	parent, ok := r.root.find(paths[0 : pathsLen-1])
	if parent == nil || !ok {
		return nil, errors.New("[Root/createAddNode] error: cannot found parent.")
	}

	node := NewChildNode(parent, paths[pathsLen-1])
	if node == nil {
		return nil, errors.New("[Root/createAddNode] error: cannot create new child node.")
	}

	return node, r.addNode(node)
}

// n: before rename node
// p: renamed path
func (r *Root) renameNode(n *Node, dir, name string) error {
	// remove from Root
	ino := n.Ino()
	if ino == 0 {
		return errors.New("Root/RemoveNode error: inode is empty.")
	}
	if dir == "" || name == "" {
		return errors.New("Root/RemoveNode error: rename dirname is empty.")
	}

	// find parent
	parent, err := r.Find(dir)
	if err != nil {
		return err
	}

	// node rename (recursive call)
	nodes, dirs, err := n.rename(name, parent)
	if err != nil {
		return err
	}

	// remove from wacher when directory
	for _, dir := range dirs {
		// ignore not exist diretory error.
		if err := r.watcher.Remove(dir); err != nil {
			if debug {
				log.Printf("[Root/RenameNode] watcher Remove path: %s, error: %s\n", dir, err)
			}
		}
	}

	// add new watcher directory
	for _, node := range nodes {
		if err := r.addNode(node); err != nil {
			return err
		}
	}

	return nil
}

func (r *Root) removeNode(n *Node) error {
	ino := n.Ino()
	if ino == 0 {
		return errors.New("[Root/RemoveNode] error: inode is empty.")
	}

	// node remove (recursive call)
	nodes, err := n.remove()
	if err != nil {
		return err
	}

	for _, node := range nodes {
		ino := node.Ino()
		// ignore cannot find error
		r.nodeMap.remove(ino)

		// remove from wacher when directory
		if node.IsDir() {
			// ignore not exist diretory error.
			if err := r.watcher.Remove(node.Path()); err != nil {
				if debug {
					log.Printf("[Root/RemoveNode] watcher Remove path: %s, error: %s\n", node.Path(), err)
				}
			}
		}
	}

	return nil
}

func (r *Root) Find(absPath string) (*Node, error) {
	paths := fileinfo.SplitPath(absPath, r.root.Dir())

	if len(paths) == 0 {
		return nil, errors.New("Find error: paths length is 0.")
	}

	n, ok := r.root.find(paths)
	if n == nil || !ok {
		return nil, errors.New(fmt.Sprintf("Find error: %s node cannot found.", absPath))
	}

	return n, nil
}

func (r *Root) InoFind(ino uint64) *Node {
	if ino == 0 {
		return nil
	}

	return r.nodeMap.get(ino)
}

func (r *Root) appendWriteNodes(ne nodeEvent) error {
	if ne.node == nil {
		return errors.New("[Root/appendWriteNodes] error: event.node is nil.")
	}

	if err := ne.checkWritableEvent(); err != nil {
		return err
	}

	if err := r.writeNodes.add(ne.node); err != nil {
		return err
	}

	return nil
}

// under called in Watch()

func (r *Root) addQueue(e fsnotify.Event) error {
	r.wg.Add(1)

	go func() {
		r.mu.Lock()

		if debug && e.Op > 0 {
			log.Println("[Root/addQueue] Events: " + e.Op.String() + " Name: " + e.Name)
		}

		// add queue
		r.queues.add(e, r)

		r.mu.Unlock()

		r.wg.Done()
	}()

	return nil
}

func (r *Root) queuesToEvent() {
	if len(*(r.queues)) == 0 {
		return
	}

	r.wg.Wait()

	// exec goroutine only 1.
	r.mu.Lock()
	defer r.mu.Unlock()

	// check executing on other goroutine.
	if len(*(r.queues)) == 0 {
		return
	}

	r.queues.sort()
	defer r.queues.clear()

	nodeEvents, err := r.queues.createNodeEvents(r)
	if err != nil {
		if debug {
			log.Println(err)
		}

		return
	}

	for _, ne := range *nodeEvents {
		event := newEvent(ne)
		if debug {
			log.Println("[Root/queuesToEvent] event: " + event.String())
		}

		// append writeEvents
		r.appendWriteNodes(ne)

		// send channel
		r.Ch <- event
	}
}

func (r *Root) checkWriteNodes() {
	if len(*(r.writeNodes)) == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	nodes := r.writeNodes.checkWriteComplete()

	if len(nodes) == 0 {
		return
	}

	for _, node := range nodes {
		if node.Size() > 0 {
			event := newEventByOpNode(WriteComplete, node)

			if debug {
				log.Println("[Root/checkWriteNodes] event: " + event.String())
			}

			// send channel
			r.Ch <- event
		}
	}
}

func (r *Root) checkDirectories() {
	r.mu.Lock()
	defer r.mu.Unlock()

	eqs, _ := r.root.checkDirectory()

	*(r.queues) = append(*(r.queues), eqs...)
}

// watch start on goroutine.
func (r *Root) Watch() {
	// check already watching.
	if r.ticker != nil {
		return
	}

	go func() {
		r.ticker = time.NewTicker(1 * time.Second)
		r.chkTicker = time.NewTicker(60 * time.Second)

		for {
			select {
			case e := <-r.watcher.Events:
				r.addQueue(e)
			case <-r.ticker.C:
				r.checkWriteNodes()
				r.queuesToEvent()
			case <-r.chkTicker.C:
				r.checkDirectories()
			}
		}
	}()
}
