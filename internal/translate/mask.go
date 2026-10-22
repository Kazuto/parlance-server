package translate

import (
	"fmt"
	"slices"
)

// Mask replaces placeholders such as {name} with numbered ones, {1}, {2}, and
// returns a function that puts the originals back into a translation. The models
// keep numbered placeholders as they are, but tend to translate named ones, e.g.
// {count} into {Anzahl}. The same placeholder gets the same number each time.
func Mask(text string) (string, func(string) string) {
	var names []string
	masked := placeholderPattern.ReplaceAllStringFunc(text, func(name string) string {
		index := slices.Index(names, name)
		if index < 0 {
			names = append(names, name)
			index = len(names) - 1
		}
		return fmt.Sprintf("{%d}", index+1)
	})

	unmask := func(translation string) string {
		return placeholderPattern.ReplaceAllStringFunc(translation, func(token string) string {
			var number int
			if _, err := fmt.Sscanf(token, "{%d}", &number); err != nil || fmt.Sprintf("{%d}", number) != token {
				return token
			}
			if number < 1 || number > len(names) {
				return token
			}
			return names[number-1]
		})
	}
	return masked, unmask
}
