package board

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

// selfMu guards selfHash, the hash of the most recent save this process
// performed via SaveMerging.
var selfMu sync.Mutex
var selfHash string

// markSelfWrite records the hash of a save this process just performed.
// The fs watcher fires on our own atomic rename just like it would for an
// external writer, and that notification can reach the UI before the
// saveResultMsg carrying the same hash does (they're two independent
// goroutines racing to the same Update loop). Checking IsSelfWrite lets
// the reload path recognize its own write immediately, without depending
// on message delivery order.
func markSelfWrite(hash string) {
	selfMu.Lock()
	selfHash = hash
	selfMu.Unlock()
}

// IsSelfWrite reports whether hash matches the most recent save this
// process performed via SaveMerging.
func IsSelfWrite(hash string) bool {
	selfMu.Lock()
	defer selfMu.Unlock()
	return hash == selfHash
}

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
//
// If the on-disk state can't be read at all - e.g. a concurrent writer is
// mid-write - SaveMerging fails rather than silently falling back to a
// blind overwrite: that's the one situation a real concurrent writer is
// most likely to produce, and overwriting blind there would defeat the
// point of merging in the first place. Callers should retry.
func SaveMerging(path string, base, ours *Board, lastHash string) (toWrite *Board, merged bool, hash string, err error) {
	saveMu.Lock()
	defer saveMu.Unlock()

	onDisk, diskHash, loadErr := Load(path)
	if loadErr != nil {
		return nil, false, "", fmt.Errorf("checking for concurrent changes: %w", loadErr)
	}

	toWrite = ours
	if diskHash != lastHash {
		toWrite = Merge(base, ours, onDisk)
		merged = true
	}
	hash, err = Save(path, toWrite)
	if err != nil {
		return nil, false, "", err
	}
	markSelfWrite(hash)
	return toWrite, merged, hash, nil
}

func Hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
