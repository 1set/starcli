package util

import (
	"hash/fnv"
	"sort"
	"sync"

	"github.com/kyokomi/emoji/v2"
)

var (
	initEmojiOnce    sync.Once
	singleCharEmojis []string
)

func initEmoji() {
	// Initialize the list of single-character emojis
	for _, e := range emoji.CodeMap() {
		if len([]rune(e)) == 1 {
			singleCharEmojis = append(singleCharEmojis, e)
		}
	}
	// Sort the list for consistent ordering
	sort.Strings(singleCharEmojis)
}

// StringEmoji returns a consistent emoji for the given input string.
// The same input will always return the same emoji.
func StringEmoji(input string) string {
	// Initialize the emoji list once
	initEmojiOnce.Do(initEmoji)

	// If the input is empty, return a star emoji
	if input == "" {
		return `⭐`
	}

	// Use FNV hash for string to integer conversion
	h := fnv.New32a()
	h.Write([]byte(input))
	hash := h.Sum32()

	// Use the hash to select an emoji from the list
	index := int(hash) % len(singleCharEmojis)
	return singleCharEmojis[index]
}
