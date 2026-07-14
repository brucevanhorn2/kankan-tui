package board

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// saveMu serializes this process's own writes to the board file. kankan-tui
// only ever manages one board file per process, so a single package-level
// lock is enough to stop two of our own overlapping saves (e.g. from rapid
// keystrokes) from racing each other's read-merge-write cycle; it does
// nothing to coordinate with other processes, which is what the merge in
// SaveMerging is for.
var saveMu sync.Mutex

// Load reads a board file and returns it along with a content hash, so the
// caller can later tell its own writes apart from external changes to the
// same file (see Watcher).
func Load(path string) (*Board, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	var b Board
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, "", err
	}
	return &b, Hash(data), nil
}

// Save writes the board as indented JSON, using a temp-file-then-rename
// so a crash mid-write can't corrupt the board file. Returns a hash of the
// bytes written.
func Save(path string, b *Board) (string, error) {
	b.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".kankan-tui-*.tmp")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return "", err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return "", err
	}
	return Hash(data), nil
}

// SaveMerging writes ours to path, first checking whether the file has
// changed since lastHash - meaning some other process (an agent, another
// kankan-tui instance) wrote to it since this process last synced its own
// state. If so, it three-way merges base (the last state this process
// knows both sides agreed on), ours, and the freshly-read on-disk state
// before writing, instead of blindly overwriting the other writer's
// change. Returns the board actually written (ours unchanged, or the
// merge result) and whether a merge happened.
func SaveMerging(path string, base, ours *Board, lastHash string) (toWrite *Board, merged bool, hash string, err error) {
	saveMu.Lock()
	defer saveMu.Unlock()

	toWrite = ours
	if base != nil {
		if onDisk, diskHash, loadErr := Load(path); loadErr == nil && diskHash != lastHash {
			toWrite = Merge(base, ours, onDisk)
			merged = true
		}
	}
	hash, err = Save(path, toWrite)
	if err != nil {
		return nil, false, "", err
	}
	return toWrite, merged, hash, nil
}

func Hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
