package logviewer

import "unicode"

// charClass returns the vim word-motion class of a rune:
//
//	0 = whitespace, 1 = word char [a-zA-Z0-9_], 2 = punctuation/other
func charClass(r rune) int {
	if unicode.IsSpace(r) {
		return 0
	}
	if r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) {
		return 1
	}
	return 2
}

// wordForward returns the column of the next word start (vim `w`).
func wordForward(line string, col int) int {
	runes := []rune(line)
	n := len(runes)
	if col >= n-1 {
		return n - 1
	}
	if col < 0 {
		col = 0
	}

	// Skip current word class
	startClass := charClass(runes[col])
	i := col
	for i < n && charClass(runes[i]) == startClass {
		i++
	}
	// Skip whitespace
	for i < n && unicode.IsSpace(runes[i]) {
		i++
	}
	if i >= n {
		return n - 1
	}
	return i
}

// wordBackward returns the column of the previous word start (vim `b`).
func wordBackward(line string, col int) int {
	runes := []rune(line)
	n := len(runes)
	if col <= 0 {
		return 0
	}
	if col >= n {
		col = n - 1
	}

	i := col - 1
	// Skip whitespace before cursor
	for i > 0 && unicode.IsSpace(runes[i]) {
		i--
	}
	// Find start of word
	cls := charClass(runes[i])
	for i > 0 && charClass(runes[i-1]) == cls {
		i--
	}
	return i
}

// wordEnd returns the column of the end of the current/next word (vim `e`).
func wordEnd(line string, col int) int {
	runes := []rune(line)
	n := len(runes)
	if col >= n-1 {
		return n - 1
	}
	if col < 0 {
		col = 0
	}

	i := col + 1
	// Skip whitespace
	for i < n && unicode.IsSpace(runes[i]) {
		i++
	}
	if i >= n {
		return n - 1
	}
	// Skip to end of word class
	cls := charClass(runes[i])
	for i < n-1 && charClass(runes[i+1]) == cls {
		i++
	}
	return i
}
