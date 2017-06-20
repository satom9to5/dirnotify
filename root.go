package dirnotify

import (
	"errors"
	"fmt"
	"io/ioutil"
	"sync"
	"time"
	// third party
	"github.com/fsnotify/fsnotify"
	"github.com/satom9to5/fileinfo"
)

var (
	debug = false
)

func EnableDebug() {
	debug = true
}

func DisableDebug() {
	debug = true
}

type Root struct {
	node    *Node            // root node
	nodes   map[uint64]*Node // inode key
	queues  *eventQueues     // event queue
	watcher *fsnotify.Watcher
	ch      chan Event
	ticker  *time.Ticker
	mu      sync.Mutex
	wg      sync.WaitGroup
}

func NewRoot(dir string) (*Root, error) {
	fi, err := fileinfo.Stat(dir)
	if err != nil {
		return nil, err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	// root node
	n := &Node{
		info:  fi,
		dirs:  map[string]*Node{},
		files: map[string]*Node{},
	}

	r := &Root{
		node:    n,
		nodes:   map[uint64]*Node{},
		queues:  &eventQueues{},
		watcher: watcher,
		ch:      make(chan Event),
	}

	n.root = r

	// watcher add
	r.AddNode(n)

	return r, nil
}

func CreateNodeTree(dir string) (*Root, error) {
	r, err := NewRoot(dir)
	if err != nil {
		return nil, err
	}

	if err := r.appendNodes(r.node); err != nil {
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
		chn, err := r.CreateAddNode(n.Path() + fileinfo.PathSep + fi.Name())
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

func (r *Root) PrintTree() {
	r.node.PrintTree()
}

// watcher Close
func (r *Root) Close() {
	if r.watcher != nil {
		r.watcher.Close()
	}
}

func (r *Root) AddNode(n *Node) error {
	ino := n.Ino()
	if ino == 0 {
		return errors.New("Root/AddNode error: inode is empty.")
	}

	r.nodes[ino] = n

	// watcher add when directory
	if n.IsDir() {
		return r.watcher.Add(n.Path())
	} else {
		return nil
	}
}

func (r *Root) CreateAddNode(p string) (*Node, error) {
	paths := fileinfo.SplitPath(p, r.node.Dir())
	if len(paths) == 0 {
		return nil, errors.New("Root/CreateAddNode error: paths length is 0.")
	}

	pathsLen := len(paths)

	parent, ok := r.node.find(paths[0 : pathsLen-1])
	if parent == nil || !ok {
		return nil, errors.New("Root/CreateAddNode error: cannot found parent.")
	}

	node := NewChildNode(parent, paths[pathsLen-1])
	if node == nil {
		return nil, errors.New("Root/CreateAddNode error: cannot create new child node.")
	}

	return node, r.AddNode(node)
}

// n: before rename node
// p: renamed path
func (r *Root) RenameNode(n *Node, dir, name string) error {
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
		if err := r.watcher.Remove(dir); err != nil {
			return err
		}
	}

	// add new watcher directory
	for _, node := range nodes {
		if err := r.AddNode(node); err != nil {
			return err
		}
	}

	return nil
}

func (r *Root) RemoveNode(n *Node) error {
	ino := n.Ino()
	if ino == 0 {
		return errors.New("Root RemoveNode error: inode is empty.")
	}

	// node remove (recursive call)
	nodes, err := n.remove()
	if err != nil {
		return err
	}

	for _, node := range nodes {
		ino := node.Ino()
		if _, ok := r.nodes[ino]; ok {
			// remove from inode map.
			delete(r.nodes, ino)
		}

		// remove from wacher when directory
		if node.IsDir() {
			// ignore not exist diretory error.
			r.watcher.Remove(node.Path())
		}
	}

	return nil
}

func (r *Root) AddQueue(e fsnotify.Event) error {
	r.wg.Add(1)

	go func() {
		r.mu.Lock()

		if debug {
			fmt.Println("[Root/AddQueue] Events: " + e.Op.String() + " Name: " + e.Name)
		}

		// add queue
		r.queues.Add(e, r)

		r.mu.Unlock()

		r.wg.Done()
	}()

	return nil
}

func (r *Root) QueuesToEvent() {
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

	r.queues.Sort()
	defer r.queues.Clear()

	if err := r.queues.CreateEvents(r); err != nil {
		fmt.Println(err)
	}
}

func (r *Root) Find(absPath string) (*Node, error) {
	paths := fileinfo.SplitPath(absPath, r.node.Dir())

	if len(paths) == 0 {
		return nil, errors.New("Find error: paths length is 0.")
	}

	n, ok := r.node.find(paths)
	if n == nil || !ok {
		return nil, errors.New(fmt.Sprintf("Find error: %s node cannot found.", absPath))
	}

	return n, nil
}

func (r *Root) InoFind(ino uint64) *Node {
	if ino == 0 {
		return nil
	}

	if node, ok := r.nodes[ino]; ok {
		return node
	} else {
		return nil
	}
}

// watch start on goroutine.
func (r *Root) Watch() {
	// check already watching.
	if r.ticker != nil {
		return
	}

	go func() {
		r.ticker = time.NewTicker(1 * time.Second)

		for {
			select {
			case e := <-r.watcher.Events:
				r.AddQueue(e)
			case <-r.ticker.C:
				r.QueuesToEvent()
			}
		}
	}()
}
