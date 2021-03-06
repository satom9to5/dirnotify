package dirnotify

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

var (
	_dirs   = []string{""}
	_subdir = ""
	_root   = &Root{}
)

func TestMain(m *testing.M) {
	EnableDebug()

	code := m.Run()

	after()

	os.Exit(code)
}

// delete temp directory
func after() {
	removeTempDir()
}

func TestNonWatch(t *testing.T) {
	// prepare
	makeTempDir()
	createTestNodeTree(t)

	SubTestRootFindDir(t)
	SubTestManipulateFile(t)
	SubTestManipulateDirectory(t)

	_root.Close()

	removeTempDir()
}

func TestWatch(t *testing.T) {
	makeTempDir()

	createTestNodeTree(t)
	_root.Watch()

	// main tests
	SubTestWatch(t)

	_root.Close()
	removeTempDir()
}

// shared test & initialize
func createTestNodeTree(t *testing.T) {
	r, err := CreateNodeTree(_dirs)
	if err != nil {
		t.Fatalf("[testCreateNodeTree] cannot create Root: %s", err)
	}

	// use result after test.
	_root = r
}

func makeTempDir() {
	_dirs[0] = tempdir()

	dirs := []string{
		"/bin",
		"/sbin",
		"/usr/bin",
		"/usr/sbin",
		"/usr/local/bin",
		"/usr/local/etc",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(_dirs[0]+filepath.FromSlash(dir), 0777); err != nil {
			fmt.Printf("failed to create directory: %s", err)
			os.Exit(1)
		}
	}

	// file name is windows style because can't distinct if unix style
	files := []string{
		"/usr/bin/ls.exe",
		"/usr/sbin/ip.exe",
		"/usr/bin/cat.exe",
	}

	for _, file := range files {
		if f, err := os.Create(_dirs[0] + filepath.FromSlash(file)); err != nil {
			fmt.Printf("failed to create file: %v", err)
			os.Exit(1)
		} else {
			f.Close()
		}
	}
}

// delete temp directory
func removeTempDir() {
	os.RemoveAll(_dirs[0])
}

func tempdir() string {
	dir, err := ioutil.TempDir("", "dirnotify")

	if err != nil {
		fmt.Printf("failed to create test directory: %s", err)
		os.Exit(1)
	}

	return dir
}

func tempfile(dir string) string {
	f, err := ioutil.TempFile(dir, "dirnotify")

	if err != nil {
		fmt.Printf("failed to create test file: %v", err)
		os.Exit(1)
	}

	defer f.Close()
	return f.Name()
}
