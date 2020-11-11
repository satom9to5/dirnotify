package dirnotify

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"time"
	// third party
	"github.com/satom9to5/fileinfo"
)

/**
 * node tree sample
 *
 * root (directory)
 * |- foo (dirs)
 * | |- foo.txt (files)
 * |- bar (dirs)
 * | |- work (dirs)
 * | |- foo.txt (files)
 * | |- bar.txt (files)
 * |- test.txt (files)
 */

type Node struct {
	info   *fileinfo.FileInfo
	parent *Node            // parent directory
	dirs   map[string]*Node // directory(has directories or files)
	files  map[string]*Node // file(end node)
}

func NewChildNode(parent *Node, childName string) *Node {
	if parent == nil {
		return nil
	}

	absPath := parent.Path() + fileinfo.PathSep + childName

	fi, err := fileinfo.Stat(absPath)
	if err != nil {
		if debug {
			log.Printf("[NewChildNode] fileinfo error: %s\n", absPath)
		}

		return nil
	}

	n := &Node{
		info:   fi,
		parent: parent,
	}

	// add parent dirs or files
	// watcher add on directory
	if fi.IsDir() {
		n.dirs = map[string]*Node{}
		n.files = map[string]*Node{}

		parent.dirs[childName] = n
	} else {
		parent.files[childName] = n
	}

	return n
}

func (n Node) String() string {
	result := fmt.Sprintf("info: [%s]", n.info)

	if len(n.dirs) == 0 && len(n.files) == 0 {
		return result
	}

	if len(n.dirs) > 0 {
		result += ", dirs: ["

		names := []string{}
		for name, _ := range n.dirs {
			names = append(names, name)
		}

		result += strings.Join(names, ",") + "]"
	}

	if len(n.files) > 0 {
		result += ", files: ["

		names := []string{}
		for name, _ := range n.files {
			names = append(names, name)
		}

		result += strings.Join(names, ",") + "]"
	}

	return result
}

func (n *Node) FileInfo() *fileinfo.FileInfo {
	return n.info
}

func (n *Node) Name() string {
	return n.info.Name()
}

func (n *Node) Size() int64 {
	return n.info.Size()
}

func (n *Node) ModTime() time.Time {
	return n.info.ModTime()
}

func (n *Node) IsDir() bool {
	return n.info.IsDir()
}

func (n *Node) Dir() string {
	return n.info.Dir()
}

func (n *Node) Path() string {
	return n.info.Path()
}

func (n *Node) Ino() uint64 {
	return n.info.Ino()
}

func (n *Node) Stat() error {
	fi, err := fileinfo.Stat(n.Path())
	if err != nil {
		return nil
	}

	n.info = fi

	return nil
}

// 2nd return bool
// true: target.
// false: non target.
func (n *Node) find(paths []string) (*Node, bool) {
	switch len(paths) {
	case 0:
		return nil, false
	case 1:
		return n, n.Name() == paths[0]
	}

	childPath := paths[1]

	if _, ok := n.dirs[childPath]; ok {
		// when directory
		return n.dirs[childPath].find(paths[1:])
	} else if _, ok := n.files[childPath]; ok {
		// when file
		return n.files[childPath].find(paths[1:])
	} else {
		return n, false
	}
}

// first call on Rename
func (n *Node) rename(name string, parent *Node) (dirNodes []*Node, oldDirs []string, err error) {
	curName := n.Name()

	// remove from parents
	if n.IsDir() {
		oldDirs = append(oldDirs, n.Path())

		if _, ok := n.parent.dirs[curName]; ok {
			delete(n.parent.dirs, curName)
		}
	} else {
		if _, ok := n.parent.files[curName]; ok {
			delete(n.parent.files, curName)
		}
	}

	if n.parent != parent {
		n.parent = parent
	}

	// update fileinfo
	absPath := n.parent.Path() + fileinfo.PathSep + name

	fi, err := fileinfo.Stat(absPath)
	if err != nil {
		return
	}

	n.info = fi

	// add parents
	if n.IsDir() {
		dirNodes = append(dirNodes, n)

		n.parent.dirs[name] = n
	} else {
		n.parent.files[name] = n
	}

	// rename children files/directories
	if nodes, dirs, err := n.updateChildren(); err == nil {
		return append(dirNodes, nodes...), append(oldDirs, dirs...), nil
	} else {
		return nil, nil, err
	}
}

func (n *Node) updateInfo() ([]*Node, []string, error) {
	oldPath := n.Path()

	absPath := n.parent.Path() + fileinfo.PathSep + n.Name()

	fi, err := fileinfo.Stat(absPath)
	if err != nil {
		return nil, nil, err
	}

	n.info = fi

	if nodes, dirs, err := n.updateChildren(); err == nil {
		if n.IsDir() {
			nodes = append(nodes, n)
			dirs = append(dirs, oldPath)
		}

		return nodes, dirs, nil
	} else {
		return nil, nil, err
	}
}

func (n *Node) updateChildren() (dirNodes []*Node, oldDirs []string, err error) {
	for _, file := range n.files {
		if _, _, err = file.updateInfo(); err != nil {
			return
		}
	}

	for _, dir := range n.dirs {
		if nodes, dirs, err := dir.updateInfo(); err == nil {
			dirNodes = append(dirNodes, nodes...)
			oldDirs = append(oldDirs, dirs...)
		} else {
			return nil, nil, err
		}
	}

	return
}

func (n *Node) remove() (removeNodes []*Node, err error) {
	if n.parent == nil {
		return nil, errors.New("Node remove error: parent is nil.")
	}

	if _, ok := n.parent.dirs[n.Name()]; ok {
		delete(n.parent.dirs, n.Name())
	}

	if _, ok := n.parent.files[n.Name()]; ok {
		delete(n.parent.files, n.Name())
	}

	removeNodes = append(removeNodes, n)

	for _, file := range n.files {
		if nodes, err := file.remove(); err == nil {
			removeNodes = append(removeNodes, nodes...)
		} else {
			return removeNodes, err
		}
	}

	// recursive call
	for _, dir := range n.dirs {
		if nodes, err := dir.remove(); err == nil {
			removeNodes = append(removeNodes, nodes...)
		} else {
			return removeNodes, err
		}
	}

	return
}

func (n *Node) children() []*Node {
	nodes := []*Node{}

	for _, file := range n.files {
		nodes = append(nodes, file)
	}

	for _, dir := range n.dirs {
		nodes = append(nodes, dir)
		nodes = append(nodes, dir.children()...)
	}

	return nodes
}

// debug
func (n *Node) PrintTree() string {
	return n.printTree(0)
}

func (n *Node) printTree(depth int) string {
	str := fmt.Sprintf("%-"+strconv.Itoa(depth*2)+"s%s\n", "", n)

	if !n.IsDir() {
		return str
	}

	for _, file := range n.files {
		str += file.printTree(depth + 1)
	}

	for _, dir := range n.dirs {
		str += dir.printTree(depth + 1)
	}

	return str
}

func (n *Node) walk(fn walkFunc) error {
	if n.info == nil {
		return errors.New("[Node/walk] error: node info is nil.")
	}

	fn(*(n.info))

	for _, file := range n.files {
		if err := file.walk(fn); err != nil {
			return err
		}
	}

	for _, dir := range n.dirs {
		if err := dir.walk(fn); err != nil {
			return err
		}
	}

	return nil
}

func (n *Node) checkCurrentDirectory() (eqs eventQueues, err error) {
	if !n.IsDir() {
		return eqs, errors.New("[Node/checkCurrentDirectory] error: node is not directory.")
	}

	curTime := n.ModTime()

	if err = n.Stat(); err != nil {
		return
	}

	// check when update ModTime
	if curTime != n.ModTime() {
		eqs, err = n.checkDirectory()
		if debug && err != nil {
			log.Println("[Node/checkCurrentDirectory] error: " + err.Error())
		}
	}

	// check child directories
	var deqs eventQueues
	for _, dir := range n.dirs {
		deqs, err = dir.checkCurrentDirectory()
		if debug && err != nil {
			log.Println("[Node/checkCurrentDirectory] error: " + err.Error())
		}

		eqs = append(eqs, deqs...)
	}

	return
}

func (n *Node) checkDirectory() (eventQueues, error) {
	eqs := eventQueues{}

	fis, err := ioutil.ReadDir(n.Path())

	if err != nil {
		return eqs, err
	}

	// get current directories & files
	dirNames, fileNames := []string{}, []string{}

	for _, fi := range fis {
		if fi.IsDir() {
			dirNames = append(dirNames, fi.Name())
		} else {
			fileNames = append(fileNames, fi.Name())
		}
	}

	// compare current dirs&files and nodes
	dirs, files := map[string]*Node{}, map[string]*Node{}
	// delete when include dirNames&fileNames.
	removeDirs, removeFiles := n.dirs, n.files
	// not found dirs&files path
	nfDirs, nfFiles := []string{}, []string{}

	// append not found dirs&files
	for _, dirName := range dirNames {
		_, name := fileinfo.Split(dirName)

		if dir, ok := removeDirs[name]; ok {
			dirs[name] = dir
			delete(removeDirs, name)
		} else {
			nfDirs = append(nfDirs, name)
		}
	}

	for _, fileName := range fileNames {
		_, name := fileinfo.Split(fileName)

		if file, ok := removeFiles[name]; ok {
			files[name] = file
			delete(removeFiles, name)
		} else {
			nfFiles = append(nfFiles, name)
		}
	}

	// replace Node.dirs & Node.files
	n.dirs = dirs
	n.files = files

	// append not found dirs&files
	for _, dirName := range nfDirs {
		node := NewChildNode(n, dirName)
		if debug {
			fmt.Println("[Node/checkDirectory] add directory: " + node.String())
		}
		eqs.addFromNode(node, Create)
	}

	for _, fileName := range nfFiles {
		node := NewChildNode(n, fileName)
		if debug {
			fmt.Println("[Node/checkDirectory] add file: " + node.String())
		}
		eqs.addFromNode(node, Create)
	}

	// append remove nodes
	for _, dir := range removeDirs {
		if debug {
			fmt.Println("[Node/checkDirectory] remove directory: " + dir.String())
		}
		eqs.addFromNode(dir, Remove)
	}

	for _, file := range removeFiles {
		if debug {
			fmt.Println("[Node/checkDirectory] remove file: " + file.String())
		}
		eqs.addFromNode(file, Remove)
	}

	return eqs, nil
}
