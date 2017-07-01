package dirnotify

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func SubTestRootFindDir(t *testing.T) {
	// test pattern
	patterns := []string{
		_dir,
		_dir + filepath.FromSlash("/usr/local/bin"),
	}

	for _, pattern := range patterns {
		if _, err := _root.Find(pattern); err != nil {
			t.Fatalf("[SubTestRootFindDir] failed to find directory: %s", err)
		}
	}
}

func SubTestManipulateFile(t *testing.T) {
	var node *Node
	var err error

	// test patterns
	patterns := []struct {
		addDir      string
		addFile     string
		renamedDir  string
		renamedFile string
	}{
		{"usr/local/bin", "more.exe", "usr/local/bin", "less.exe"},
	}

	for _, pattern := range patterns {
		addFile := pattern.addFile
		addPath := filepath.Join(_dir, filepath.FromSlash(pattern.addDir), pattern.addFile)
		renamedDir := filepath.Join(_dir, filepath.FromSlash(pattern.renamedDir))
		renamedFile := pattern.renamedFile
		renamedPath := filepath.Join(renamedDir, pattern.renamedFile)

		// create new file
		if f, err := os.Create(addPath); err != nil {
			t.Fatalf("[SubTestManipulateFile] failed to create file: %s", err)
		} else {
			f.Close()
		}

		if node, err = _root.createAddNode(addPath); err != nil {
			t.Fatalf("[SubTestManipulateFile] failed to Root/CreateAddNode: %s", err)
		} else {
			if err = testSamePathName(t, node, addPath, addFile); err != nil {
				t.Fatalf("[SubTestManipulateFile] failed to Root/CreateAddNode: %s", err)
			}
		}

		if node, err = _root.Find(addPath); err != nil {
			t.Fatalf("[SubTestManipulateFile] failed to Root/Find: %s", err)
		} else {
			if err = testSamePathName(t, node, addPath, addFile); err != nil {
				t.Fatalf("[SubTestManipulateFile] failed to Root/Find: %s", err)
			}
		}

		if node = _root.InoFind(node.Ino()); node == nil {
			t.Fatalf("[SubTestManipulateFile] failed to Root/InoFind: cannot find %s", node)
		}

		// rename file
		if err = os.Rename(addPath, renamedPath); err != nil {
			t.Fatalf("[SubTestManipulateFile] failed to rename file: %s", err)
		}

		if err = _root.renameNode(node, renamedDir, renamedFile); err != nil {
			t.Fatalf("[SubTestManipulateFile] failed to Root/RenameNode: %s", err)
		} else {
			if err = testSamePathName(t, node, renamedPath, renamedFile); err != nil {
				t.Fatalf("[SubTestManipulateFile] failed to Root/RenameNode: %s", err)
			}
		}

		if node, err = _root.Find(renamedPath); err != nil {
			t.Fatalf("[SubTestManipulateFile] failed to Root/Find: %s", err)
		} else {
			if err = testSamePathName(t, node, renamedPath, renamedFile); err != nil {
				t.Fatalf("[SubTestManipulateFile] failed to Root/Find: %s", err)
			}
		}

		node = _root.InoFind(node.Ino())
		if err = testSamePathName(t, node, renamedPath, renamedFile); err != nil {
			t.Fatalf("[SubTestManipulateFile] failed to Root/InoFind: %s", err)
		}

		// remove file
		if err = os.Remove(renamedPath); err != nil {
			t.Fatalf("[SubTestManipulateFile] failed to remove file: %s", err)
		}

		if err = _root.removeNode(node); err != nil {
			t.Fatalf("[SubTestManipulateFile] failed to Root/RemoveNode: %s", err)
		}

		if _, err = _root.Find(renamedPath); err == nil {
			t.Fatalf("[SubTestManipulateFile] failed to Root/Find: %s", err)
		}

		if node = _root.InoFind(node.Ino()); node != nil {
			t.Fatalf("[SubTestManipulateFile] failed to Root/InoFind: node is not nil")
		}
	}
}

func SubTestManipulateDirectory(t *testing.T) {
	var node, fileNode *Node
	var err error

	// test patterns
	patterns := []struct {
		addDir      string
		addName     string
		renamedDir  string
		renamedName string
		fileNames   []string
	}{
		{"usr/local/etc", "httpd", "usr/local/etc", "apache", []string{"httpd.conf", "mime.conf"}},
	}

	// for file remove check
	fileInodes := []uint64{}

	for _, pattern := range patterns {
		addName := pattern.addName
		addPath := filepath.Join(_dir, filepath.FromSlash(pattern.addDir), pattern.addName)
		renamedDir := filepath.Join(_dir, filepath.FromSlash(pattern.renamedDir))
		renamedName := pattern.renamedName
		renamedPath := filepath.Join(renamedDir, pattern.renamedName)

		// create new directory
		if err = os.MkdirAll(addPath, 0777); err != nil {
			t.Fatalf("[SubTestManipulateDirectory] failed to create directory: %s", err)
		}

		if node, err = _root.createAddNode(addPath); err != nil {
			t.Fatalf("[SubTestManipulateDirectory] failed to CreateAddNode: %s", err)
		} else {
			if err = testSamePathName(t, node, addPath, addName); err != nil {
				t.Fatalf("[SubTestManipulateDirectory] failed to Root/CreateAddNode: %s", err)
			}
		}

		if node, err = _root.Find(addPath); err != nil {
			t.Fatalf("[SubTestManipulateDirectory] failed to find directory: %s", err)
		} else {
			if err = testSamePathName(t, node, addPath, addName); err != nil {
				t.Fatalf("[SubTestManipulateDirectory] failed to Root/Find: %s", err)
			}
		}

		if node = _root.InoFind(node.Ino()); node == nil {
			t.Fatalf("[SubTestManipulateDirectory] failed to Root/InoFind: cannot find %s", node)
		}

		// append files on directory
		for _, fileName := range pattern.fileNames {
			filePath := filepath.Join(addPath, fileName)

			if f, err := os.Create(filePath); err != nil {
				t.Fatalf("[SubTestManipulateDirectory] failed to create file: %s", err)
			} else {
				f.Close()
			}

			if fileNode, err = _root.createAddNode(filePath); err != nil {
				t.Fatalf("[SubTestManipulateDirectory] failed to CreateAddNode: %s", err)
			} else {
				if err = testSamePathName(t, fileNode, filePath, fileName); err != nil {
					t.Fatalf("[SubTestManipulateDirectory] failed to Root/CreateAddNode: %s", err)
				}
			}

			if fileNode, err = _root.Find(filePath); err != nil {
				t.Fatalf("[SubTestManipulateDirectory] failed to find file: %s", err)
			} else {
				if err = testSamePathName(t, fileNode, filePath, fileName); err != nil {
					t.Fatalf("[SubTestManipulateDirectory] failed to Root/Find: %s", err)
				}
			}

			if fileNode = _root.InoFind(fileNode.Ino()); fileNode == nil {
				t.Fatalf("[SubTestManipulateDirectory] failed to Root/InoFind: cannot find %s", fileNode)
			}

			// add fileInodes
			fileInodes = append(fileInodes, fileNode.Ino())
		}

		// move directory
		if err = os.Rename(addPath, renamedPath); err != nil {
			t.Fatalf("[SubTestManipulateFile] failed to rename directory: %s", err)
		}

		if err = _root.renameNode(node, renamedDir, renamedName); err != nil {
			t.Fatalf("[SubTestManipulateFile] failed to Root/RenameNode: %s", err)
		} else {
			if err = testSamePathName(t, node, renamedPath, renamedName); err != nil {
				t.Fatalf("[SubTestManipulateFile] failed to Root/RenameNode: %s", err)
			}
		}

		if node, err = _root.Find(renamedPath); err != nil {
			t.Fatalf("[SubTestManipulateFile] failed to Root/Find: %s", err)
		} else {
			if err = testSamePathName(t, node, renamedPath, renamedName); err != nil {
				t.Fatalf("[SubTestManipulateFile] failed to Root/Find: %s", err)
			}
		}

		node = _root.InoFind(node.Ino())
		if err = testSamePathName(t, node, renamedPath, renamedName); err != nil {
			t.Fatalf("[SubTestManipulateFile] failed to Root/InoFind: %s", err)
		}

		// child files check
		for _, fileName := range pattern.fileNames {
			filePath := filepath.Join(renamedPath, fileName)
			if fileNode, err = _root.Find(filePath); err != nil {
				t.Fatalf("[SubTestManipulateDirectory] failed to find directory: %s", err)
			} else {
				if err = testSamePathName(t, fileNode, filePath, fileName); err != nil {
					t.Fatalf("[SubTestManipulateDirectory] failed to Root/Find: %s", err)
				}
			}

			if fileNode = _root.InoFind(fileNode.Ino()); fileNode == nil {
				t.Fatalf("[SubTestManipulateDirectory] failed to Root/InoFind: cannot find %s", fileNode)
			}
		}

		// remove directory
		if err = os.RemoveAll(renamedPath); err != nil {
			t.Fatalf("[SubTestManipulateDirectory] failed to remove directory: %s", err)
		}

		if err = _root.removeNode(node); err != nil {
			t.Fatalf("[SubTestManipulateDirectory] failed to Root/RemoveNode: %s", err)
		}

		if _, err = _root.Find(renamedPath); err == nil {
			t.Fatalf("[SubTestManipulateDirectory] failed to Root/Find: %s", err)
		}

		if node = _root.InoFind(node.Ino()); node != nil {
			t.Fatalf("[SubTestManipulateDirectory] failed to Root/InoFind: node is not nil")
		}

		// child files check
		for _, fileName := range pattern.fileNames {
			filePath := filepath.Join(renamedPath, fileName)
			if node, err = _root.Find(filePath); err == nil {
				t.Fatalf("[SubTestManipulateDirectory] failed to Root/Find: %s", err)
			}
		}

		for _, ino := range fileInodes {
			if fileNode = _root.InoFind(ino); fileNode != nil {
				t.Fatalf("[SubTestManipulateDirectory] failed to Root/InoFind: node is not nil")
			}
		}
	}

	testNodes(t, _root.node)
}

func testSamePathName(t *testing.T, node *Node, p, n string) error {
	if node == nil {
		return errors.New("node is nil")
	}

	if node.Path() != p || node.Name() != n {
		return errors.New(fmt.Sprintf("different path or name. target[path: %s, name: %s] node[path: %s, name: %s]", p, n, node.Path(), node.Name()))
	}

	return nil
}
