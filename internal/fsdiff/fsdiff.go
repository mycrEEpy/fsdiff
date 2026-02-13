package fsdiff

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/charlievieth/fastwalk"
)

var (
	createSnapshot = flag.Bool("n", false, "new snapshot")
	rootDir        = flag.String("d", "", "root dir")

	snapDir = filepath.Join(os.TempDir(), "fsdiff")
)

type Walker struct {
	ch    chan string
	nodes []string
}

func New() (*Walker, error) {
	return &Walker{
		ch:    make(chan string, 1024),
		nodes: make([]string, 0, 1024),
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
	sort.Strings(w.nodes)

	if *createSnapshot {
		err := w.createSnapshot()
		if err != nil {
			return fmt.Errorf("failed to create snapshot: %w", err)
		}

		return nil
	}

	var err error

	switch flag.NArg() {
	case 0:
		err = w.diff("")
	case 1:
		err = w.diff(flag.Args()[0])
	default:
		return fmt.Errorf("invalid arguments: %+v", os.Args)
	}
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
	defer snapFile.Close()

	bufferedWriter := bufio.NewWriter(snapFile)
	defer bufferedWriter.Flush()

	snapWriter := gzip.NewWriter(bufferedWriter)
	defer snapWriter.Close()

	snapHasher := sha256.New()

	for _, node := range w.nodes {
		_, err = snapWriter.Write([]byte(node + "\n"))
		if err != nil {
			return fmt.Errorf("failed to write to snapshot file: %w", err)
		}

		snapHasher.Write([]byte(node))
	}

	err = snapWriter.Flush()
	if err != nil {
		return fmt.Errorf("failed to write snapshot file: %w", err)
	}

	snapHash := hex.EncodeToString(snapHasher.Sum(nil))

	hashFileName := filepath.Join(snapDir, snapHash+".gz")

	err = os.Rename(snapFile.Name(), hashFileName)
	if err != nil {
		return fmt.Errorf("failed to rename snapshot file: %w", err)
	}

	latestSnap := filepath.Join(snapDir, "latest")

	err = os.Remove(latestSnap)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove latest snapshot file: %w", err)
	}

	err = os.Symlink(hashFileName, latestSnap)
	if err != nil {
		return fmt.Errorf("failed to link snapshot file: %w", err)
	}

	fmt.Printf("snapshot: %s\n", snapHash)
	fmt.Printf("nodes: %d\n", len(w.nodes))

	return nil
}

func (w *Walker) walk(path string, d fs.DirEntry, _ error) error {
	fi, err := d.Info()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	w.ch <- fmt.Sprintf("%d %s", fi.Size(), path)

	return nil
}

func (w *Walker) collect() {
	for path := range w.ch {
		w.nodes = append(w.nodes, path)
	}
}

func (w *Walker) diff(snapHash string) error {
	snapHashFile := "latest"

	if len(snapHash) > 0 {
		snapHashFile = snapHash + ".gz"
	}

	snapFile, err := os.Open(filepath.Join(snapDir, snapHashFile))
	if err != nil {
		return fmt.Errorf("failed to open snapshot file: %w", err)
	}
	defer snapFile.Close()

	snapReader, err := gzip.NewReader(snapFile)
	if err != nil {
		return fmt.Errorf("failed to open snapshot file reader: %w", err)
	}
	defer snapReader.Close()

	snapScanner := bufio.NewScanner(snapReader)

	snap := make([]string, 0, 1024)

	for snapScanner.Scan() {
		snap = append(snap, snapScanner.Text())
	}

	diff := difference(w.nodes, snap)

	for _, path := range diff {
		fmt.Println(path)
	}

	return nil
}

// a is the newer snapshot, b is the older
func difference(a, b []string) []string {
	ma := make(map[string]string, len(a))
	mb := make(map[string]string, len(b))

	for _, item := range a {
		var size, path string
		fmt.Sscanf(item, "%s %s", &size, &path)
		ma[path] = size
	}

	for _, item := range b {
		var size, path string
		fmt.Sscanf(item, "%s %s", &size, &path)
		mb[path] = size
	}

	var diff []string

	for path, sizeA := range ma {
		if sizeB, found := mb[path]; !found {
			diff = append(diff, "+++ "+path)
		} else if sizeA != sizeB {
			diff = append(diff, "~~~ "+path)
		}
	}

	for path := range mb {
		if _, found := ma[path]; !found {
			diff = append(diff, "--- "+path)
		}
	}

	return diff
}
