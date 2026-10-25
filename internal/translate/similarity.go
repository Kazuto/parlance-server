package translate

import "strings"

// Similarity tells how alike two texts are, from 0 to 1: the share of the longer
// text's characters that stay when editing one into the other, ignoring case and
// surrounding space. "Delete entry" and "Delete entries" are 0.79 alike.
func Similarity(a, b string) float64 {
	x := []rune(strings.ToLower(strings.TrimSpace(a)))
	y := []rune(strings.ToLower(strings.TrimSpace(b)))

	longest := max(len(x), len(y))
	if longest == 0 {
		return 1
	}

	return 1 - float64(editDistance(x, y))/float64(longest)
}

// editDistance counts the characters to insert, delete or replace to turn a into b
// (Levenshtein distance), keeping two rows of the table.
func editDistance(a, b []rune) int {
	previous := make([]int, len(b)+1)
	current := make([]int, len(b)+1)

	for j := range previous {
		previous[j] = j
	}

	for i := 1; i <= len(a); i++ {
		current[0] = i

		for j := 1; j <= len(b); j++ {
			replace := previous[j-1]
			if a[i-1] != b[j-1] {
				replace++
			}

			current[j] = min(previous[j]+1, current[j-1]+1, replace)
		}

		previous, current = current, previous
	}

	return previous[len(b)]
}
