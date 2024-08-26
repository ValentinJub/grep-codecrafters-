package main

func isBackReference(pattern string) bool {
	for i, c := range pattern {
		if i == 0 {
			if c != '\\' {
				return false
			}
		} else if c < '1' || c > '9' {
			return false
		}
	}
	return true
}

func isDigitMatch(pattern string) bool {
	return pattern == "\\d"
}

func isAlphaNumericMatch(pattern string) bool {
	return pattern == "\\w"
}

func isAlphaOrDigitMatch(pattern string) bool {
	return isAlphaNumericMatch(pattern) || isDigitMatch(pattern)
}

func isCharacterGroupMatch(pattern string) bool {
	return pattern[0] == '[' && pattern[len(pattern)-1] == ']'
}

func isCaptureGroupMatch(pattern string) bool {
	return pattern[0] == '(' && pattern[len(pattern)-1] == ')'
}

func getCharacterGroupPattern(pattern string, isPositive bool) []byte {
	patternLength := len(pattern) - 2
	if !isPositive {
		patternLength--
	}
	group := make([]byte, patternLength)
	for x, b := range pattern {
		if x == 0 || x == len(pattern)-1 || (!isPositive && x == 1) {
			continue
		}
		if !isPositive {
			group[x-2] = byte(b)
		} else {
			group[x-1] = byte(b)
		}
	}
	return group
}

// check for the presence of a wildcard character in a pattern
func patternHasAWildcard(pattern string) bool {
	for _, b := range pattern {
		if b == '.' {
			return true
		}
	}
	return false
}

func contains(slice []byte, target byte) bool {
	for _, b := range slice {
		if b == target {
			return true
		}
	}
	return false
}

func isAlphaNumeric(b byte) bool {
	return isAnyCaseLetter(b) || isUnderScore(b) || isDigit(b)
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func isLowerCaseLetter(b byte) bool {
	return b >= 'a' && b <= 'z'
}

func isUpperCaseLetter(b byte) bool {
	return b >= 'A' && b <= 'Z'
}

func isAnyCaseLetter(b byte) bool {
	return isLowerCaseLetter(b) || isUpperCaseLetter(b)
}

func isUnderScore(b byte) bool {
	return b == '_'
}
