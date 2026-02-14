package fsdiff

import (
	"bufio"
	"compress/gzip"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/charlievieth/fastwalk"
)

var (
	createSnapshot  = flag.Bool("s", false, "create new snapshot")
	compareSnapshot = flag.String("c", "latest", "compare snapshot id")
	rootDir         = flag.String("d", "", "snapshot directory (default \"working directory\")")

	snapDir = filepath.Join(os.TempDir(), "fsdiff")
)

type Walker struct {
	ch    chan nodeInfo
	nodes map[string]int64
}

type nodeInfo struct {
	path string
	size int64
}

func New() (*Walker, error) {
	return &Walker{
		ch:    make(chan nodeInfo, 1024),
		nodes: make(map[string]int64, 4096),
	}, nil
}

func (w *Walker) Run() error {
	if *rootDir == "" {
		var err error

		*rootDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}
	}

	go func() {
		err := fastwalk.Walk(&fastwalk.DefaultConfig, *rootDir, w.walk)
		if err != nil {
			fmt.Printf("failed to walk directory: %s\n", err)
			os.Exit(1)
		}

		close(w.ch)
	}()

	w.collect()

	if *createSnapshot {
		err := w.createSnapshot()
		if err != nil {
			return fmt.Errorf("failed to create snapshot: %w", err)
		}

		return nil
	}

	err := w.diff(*compareSnapshot)
	if err != nil {
		return fmt.Errorf("failed to diff: %w", err)
	}

	return nil
}

func (w *Walker) createSnapshot() error {
	err := os.MkdirAll(snapDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create directory %s: %s", snapDir, err)
	}

	snapFile, err := os.CreateTemp(snapDir, "")
	if err != nil {
		return fmt.Errorf("failed to create snapshot file: %w", err)
	}
	defer func() {_ = snapFile.Close()}()

	bufferedWriter := bufio.NewWriter(snapFile)
	defer func() {_ = bufferedWriter.Flush()}()

	snapWriter := gzip.NewWriter(bufferedWriter)
	defer func() {_ = snapWriter.Close()}()

	for path, size := range w.nodes {
		line := fmt.Sprintf("%d %q\n", size, path)

		_, err = snapWriter.Write([]byte(line))
		if err != nil {
			return fmt.Errorf("failed to write to snapshot file: %w", err)
		}
	}

	err = snapWriter.Flush()
	if err != nil {
		return fmt.Errorf("failed to write snapshot file: %w", err)
	}

	latestSnap := filepath.Join(snapDir, "latest")

	err = os.Remove(latestSnap)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove latest snapshot file: %w", err)
	}

	err = os.Symlink(snapFile.Name(), latestSnap)
	if err != nil {
		return fmt.Errorf("failed to link snapshot file: %w", err)
	}

	fmt.Printf("snapshot: %s\n", filepath.Base(snapFile.Name()))
	fmt.Printf("nodes: %d\n", len(w.nodes))

	return nil
}

func (w *Walker) walk(path string, d fs.DirEntry, _ error) error {
	fi, err := d.Info()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	w.ch <- nodeInfo{path: path, size: fi.Size()}

	return nil
}

func (w *Walker) collect() {
	for ni := range w.ch {
		w.nodes[ni.path] = ni.size
	}
}

func (w *Walker) diff(snapId string) error {
	snapFile, err := os.Open(filepath.Join(snapDir, snapId))
	if err != nil {
		return fmt.Errorf("failed to open snapshot file: %w", err)
	}
	defer func() {_ = snapFile.Close()}()

	snapReader, err := gzip.NewReader(snapFile)
	if err != nil {
		return fmt.Errorf("failed to open snapshot file reader: %w", err)
	}
	defer func() {_ = snapReader.Close()}()

	snapScanner := bufio.NewScanner(snapReader)

	snap := make(map[string]int64, 4096)

	for snapScanner.Scan() {
		var path string
		var size int64

		_, err = fmt.Sscanf(snapScanner.Text(), "%d %q", &size, &path)
		if err != nil {
			return fmt.Errorf("failed to parse snapshot file: %w", err)
		}

		snap[path] = size
	}

	diff := difference(w.nodes, snap)

	for _, path := range diff {
		fmt.Println(path)
	}

	return nil
}

// a is the newer snapshot, b is the older
func difference(a, b map[string]int64) []string {
	var diff []string

	for path, sizeA := range a {
		if sizeB, found := b[path]; !found {
			diff = append(diff, "+++ "+path)
		} else if sizeA != sizeB {
			diff = append(diff, "~~~ "+path)
		}

		delete(b, path)
	}

	for path := range b {
		diff = append(diff, "--- "+path)
	}

	return diff
}
