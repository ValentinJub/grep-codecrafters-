package main

import (
	"fmt"
	"strconv"
	"strings"
)

type Sign string

const (
	Plus        Sign = "+"
	Alternation Sign = "|"
	Optional    Sign = "?"
)

type Pattern struct {
	pattern  string
	sign     Sign
	nextbyte byte
}

type MatchedPattern struct {
	match string
	Pattern
}

type GrepHandler struct {
	pattern                                         string           //the raw pattern
	line                                            []byte           //the line to match
	line_cursor, pattern_cursor, line_cursor_offset int              //cursors
	match_start, match_end                          bool             //start and end string anchors
	patterns                                        []Pattern        //the slice of our subpatterns
	matched_patterns                                []MatchedPattern //keep track of the pattern we matched
	backreferences                                  []string         //keep track of the backreference in capture groups
}

func newGrepHandler(line []byte, pattern string) *GrepHandler {
	return &GrepHandler{line: line, pattern: pattern, backreferences: make([]string, 0), matched_patterns: make([]MatchedPattern, 0)}
}

func newMatchedPattern(match string, pattern Pattern) MatchedPattern {
	return MatchedPattern{match: match, Pattern: pattern}
}

func (gh *GrepHandler) ExtractPatterns() bool {
	gh.patterns = make([]Pattern, 0)
	var current_pattern string
	backslash := false       // \w \d
	character_group := false //[sbd] or [^dhb]
	capture_group := false
	var parenthesis_count int
	alternation := false
	backreference := false
	var previousChar rune
	var currentSign Sign
	for x, b := range gh.pattern {
		switch b {
		case '|':
			alternation = true
			current_pattern += string(b)
		case '(':
			capture_group = true
			parenthesis_count++
			if current_pattern != "" && parenthesis_count <= 1 {
				gh.patterns = append(gh.patterns, Pattern{pattern: current_pattern})
				current_pattern = ""
			}
			current_pattern += string(b)
		case ')':
			current_pattern += string(b)
			parenthesis_count--
			if parenthesis_count == 0 {
				capture_group = false
			}
			if currentSign != "" {
				p := Pattern{pattern: current_pattern, sign: currentSign}
				gh.patterns = append(gh.patterns, p)
				currentSign = ""
				current_pattern = ""
			} else if alternation && parenthesis_count < 1 {
				alternation = false
				p := Pattern{pattern: current_pattern, sign: Alternation}
				gh.patterns = append(gh.patterns, p)
				current_pattern = ""
			} else if parenthesis_count < 1 {
				p := Pattern{pattern: current_pattern}
				gh.patterns = append(gh.patterns, p)
				current_pattern = ""
			}
		case '[':
			character_group = true
			if !capture_group && current_pattern != "" {
				gh.patterns = append(gh.patterns, Pattern{pattern: current_pattern})
				current_pattern = ""
			}
			current_pattern += string(b)
		case ']':
			if character_group && !capture_group {
				character_group = false
				current_pattern += string(b)
				p := Pattern{pattern: current_pattern}
				gh.patterns = append(gh.patterns, p)
				current_pattern = ""
			} else {
				current_pattern += string(b)
			}
			character_group = false
		case '+', '?':
			if capture_group || character_group { //we add the quantifier as a literal inside the capture group && character group
				current_pattern += string(b)
			} else if previousChar == ']' { // we add the quantifier as a sign to the character group Pattern
				gh.patterns[len(gh.patterns)-1].sign = Sign(b)
			} else if current_pattern == "" && isAlphaOrDigitMatch(gh.patterns[len(gh.patterns)-1].pattern) { //we add the quantifier to a \w or \d previous pattern
				gh.patterns[len(gh.patterns)-1].sign = Sign(b)
			} else if len(current_pattern) > 1 { // bba+ -> bb , a +
				last_char := current_pattern[len(current_pattern)-1]
				current_pattern = current_pattern[:len(current_pattern)-1]                            //trim the current pattern
				gh.patterns = append(gh.patterns, Pattern{pattern: current_pattern})                  // add the bb
				gh.patterns = append(gh.patterns, Pattern{pattern: string(last_char), sign: Sign(b)}) // add the a+
				current_pattern = ""
			} else if len(current_pattern) == 1 {
				gh.patterns = append(gh.patterns, Pattern{pattern: current_pattern, sign: Sign(b)})
				current_pattern = ""
			} else {
				current_pattern += string(b)
			}
		case '\\':
			if backslash {
				backslash = false
			} else {
				if current_pattern != "" && !character_group && !capture_group {
					p := Pattern{pattern: current_pattern}
					gh.patterns = append(gh.patterns, p)
					current_pattern = ""
				}
				backslash = true
			}
			current_pattern += string(b)
		case 'w', 'd':
			if backslash {
				backslash = false
				current_pattern += string(b)
				if !character_group && !capture_group {
					p := Pattern{pattern: current_pattern}
					gh.patterns = append(gh.patterns, p)
					current_pattern = ""
				}
			} else {
				current_pattern += string(b)
			}
		default:

			if b == '^' && x == 0 { //start of string anchor
				gh.match_start = true
				continue
			} else if b == '$' && x+1 == len(gh.pattern) { //end of string anchor
				gh.match_end = true
				continue
			} else if b >= '1' && b <= '9' && (backslash || backreference) {
				backreference = true
				backslash = false
				current_pattern += string(b)
			} else {
				if backreference {
					backreference = false
					if parenthesis_count < 1 {
						p := Pattern{pattern: current_pattern}
						gh.patterns = append(gh.patterns, p)
						current_pattern = ""
					}
				}
				current_pattern += string(b)
			}
		}
		previousChar = b
	}
	if current_pattern != "" {
		p := Pattern{pattern: current_pattern}
		gh.patterns = append(gh.patterns, p)
	}
	return len(gh.patterns) > 0
}

// match additional chars
func (gh *GrepHandler) matchQuantifierPlus(matcher func(b byte) bool) string {
	copy_cursor := gh.line_cursor
	for j := gh.line_cursor; j < len(gh.line); j++ {
		if matcher(gh.line[j]) {
			gh.line_cursor++
		} else {
			return string(gh.line[copy_cursor:j])
		}
	}
	return string(gh.line[copy_cursor:])
}

// Wrapper for matching \w or \d, in combination with signs such as + ?
func (gh *GrepHandler) matchCharOrDigit() (string, bool) {
	//match the first byte only
	if gh.match_start || gh.pattern_cursor != 0 {
		if gh.patterns[gh.pattern_cursor].sign == Optional {
			if gh.patterns[gh.pattern_cursor].pattern == "\\w" {
				if isDigit(gh.line[gh.line_cursor]) {
					gh.line_cursor++
					return string(gh.line[gh.line_cursor-1 : gh.line_cursor]), true
				}
			} else if gh.patterns[gh.pattern_cursor].pattern == "\\d" {
				if isAlphaNumeric(gh.line[gh.line_cursor]) {
					gh.line_cursor++
					return string(gh.line[gh.line_cursor-1 : gh.line_cursor]), true
				}
			}
		} else if gh.patterns[gh.pattern_cursor].sign != Optional && isDigit(gh.line[gh.line_cursor]) && gh.patterns[gh.pattern_cursor].pattern == "\\d" {
			gh.match_start = false
			start := gh.line_cursor
			gh.line_cursor++
			if gh.patterns[gh.pattern_cursor].sign == "+" {
				gh.matchQuantifierPlus(isDigit)
			}
			return string(gh.line[start:gh.line_cursor]), true
		} else if gh.patterns[gh.pattern_cursor].sign != Optional && isAlphaNumeric(gh.line[gh.line_cursor]) && gh.patterns[gh.pattern_cursor].pattern == "\\w" {
			gh.match_start = false
			start := gh.line_cursor
			gh.line_cursor++
			if gh.patterns[gh.pattern_cursor].sign == "+" {
				gh.matchQuantifierPlus(isAlphaNumeric)
			}
			return string(gh.line[start:gh.line_cursor]), true
		}
	} else {
		for j := gh.line_cursor; j < len(gh.line); j++ {
			if gh.patterns[gh.pattern_cursor].sign == Optional {
				if gh.patterns[gh.pattern_cursor].pattern == "\\w" {
					if isDigit(gh.line[gh.line_cursor]) {
						gh.line_cursor++
						return string(gh.line[gh.line_cursor-1 : gh.line_cursor]), true
					}
				} else if gh.patterns[gh.pattern_cursor].pattern == "\\d" {
					if isAlphaNumeric(gh.line[gh.line_cursor]) {
						gh.line_cursor++
						return string(gh.line[gh.line_cursor-1 : gh.line_cursor]), true
					}
				}
				return "", true
			} else if isDigit(gh.line[j]) && gh.patterns[gh.pattern_cursor].pattern == "\\d" {
				start := j
				gh.line_cursor = j + 1
				if gh.patterns[gh.pattern_cursor].sign == "+" {
					gh.matchQuantifierPlus(isDigit)
				}
				return string(gh.line[start:gh.line_cursor]), true
			} else if isAlphaNumeric(gh.line[j]) && gh.patterns[gh.pattern_cursor].pattern == "\\w" {
				start := j
				gh.line_cursor = j + 1
				if gh.patterns[gh.pattern_cursor].sign == "+" {
					gh.matchQuantifierPlus(isAlphaNumeric)
				}
				return string(gh.line[start:gh.line_cursor]), true
			}
		}
	}
	return "", false
}

func (gh *GrepHandler) matchPatterns() (bool, error) {
	//go through each subpattern
	for gh.pattern_cursor = 0; gh.pattern_cursor < len(gh.patterns); gh.pattern_cursor++ {
		//tracks if we can match the subpattern, if not, no match
		part_ok := false
		pattern := gh.patterns[gh.pattern_cursor].pattern
		if isDigitMatch(pattern) || isAlphaNumericMatch(pattern) { //\w or \d
			if v, match := gh.matchCharOrDigit(); !match {
				return false, nil
			} else {
				gh.matched_patterns = append(gh.matched_patterns, newMatchedPattern(v, gh.patterns[gh.pattern_cursor]))
			}
		} else if isBackReference(pattern) { // \1 \2 \3 etc....
			index, ok := strconv.Atoi(pattern[1:])
			if ok != nil {
				fmt.Print(ok)
				panic("ouch")
			}
			if index-1 >= len(gh.backreferences) {
				panic("index is ahead of backreferences, are we sure we have recorded all backreferences from capture groups?!")
			}
			current_pattern := gh.backreferences[index-1]
			if strings.HasPrefix(string(gh.line[gh.line_cursor:]), current_pattern) {
				gh.matched_patterns = append(gh.matched_patterns, newMatchedPattern(string(gh.line[gh.line_cursor:gh.line_cursor+len(current_pattern)]), gh.patterns[gh.pattern_cursor]))
				gh.line_cursor += len(current_pattern)
			} else {
				gh.resetSearch()
			}
		} else if isCharacterGroupMatch(pattern) { // [anything with square brackets]
			var isPositive bool
			if gh.patterns[gh.pattern_cursor].pattern[1] != '^' {
				isPositive = true
			}
			part_ok = gh.matchCharacterGroupPattern(pattern, isPositive)
			if !part_ok {
				return false, nil
			}
		} else if isCaptureGroupMatch(pattern) { // (anything with parenthesis)
			//remove the parenthesis
			gh.patterns[gh.pattern_cursor].pattern = strings.TrimPrefix(gh.patterns[gh.pattern_cursor].pattern, "(")
			gh.patterns[gh.pattern_cursor].pattern = strings.TrimSuffix(gh.patterns[gh.pattern_cursor].pattern, ")")

			if gh.patterns[gh.pattern_cursor].sign == Alternation && gh.patterns[gh.pattern_cursor].pattern[0] != '(' { //if it is an Alternation in the capture group
				if ok := gh.matchCaptureGroupAlternation(); !ok {
					if gh.match_start {
						return false, nil
					} else {
						gh.resetSearch()
					}
				}
			} else { //not an alternation in the capture group
				current_pattern := gh.patterns[gh.pattern_cursor]
				//extract all the subpatterns from the group
				part_ok = gh.matchCaptureGroupSubPatterns(current_pattern.pattern)
				if !part_ok {
					if gh.match_start {
						return false, nil
					}
					gh.resetSearch()
				}
			}
		} else { //anything that doesn't fall in previous scenarios
			if len(gh.patterns[gh.pattern_cursor].pattern) == 1 {
				if part_ok = gh.matchCharacter(); !part_ok {
					//if we fail to match the first char of the pattern
					if gh.pattern_cursor == 0 {
						gh.pattern_cursor = -1
						gh.line_cursor++
						if gh.line_cursor >= len(gh.line) {
							return false, nil
						}
						continue
					} else {
						gh.pattern_cursor = -1
					}
				}
			} else if part_ok = gh.matchString(); !part_ok {
				return false, nil
			}
		}
		//don't edit this, it should work fine to control the flow
		if gh.line_cursor >= len(gh.line) && gh.pattern_cursor >= len(gh.patterns)-1 {
			continue
		} else if gh.line_cursor >= len(gh.line) {
			return false, nil
		}
	}
	if gh.match_end && gh.line_cursor < len(gh.line) {
		return false, nil
	}
	return true, nil
}

// match a single character, can match more than once wih quantifiers
func (gh *GrepHandler) matchCharacter() bool {
	//match more than once
	if gh.patterns[gh.pattern_cursor].sign == Plus {
		hasMatched := false
		for j := gh.line_cursor; j < len(gh.line); j++ {
			if gh.line[j] != gh.patterns[gh.pattern_cursor].pattern[0] {
				if hasMatched {
					gh.matched_patterns = append(gh.matched_patterns, newMatchedPattern(string(gh.line[gh.line_cursor:j]), gh.patterns[gh.pattern_cursor]))
					gh.line_cursor = j
					return true
				}
				break //will return false
			}
			hasMatched = true
		}
	} else { //match only once
		if gh.line[gh.line_cursor] == gh.patterns[gh.pattern_cursor].pattern[0] {
			gh.matched_patterns = append(gh.matched_patterns, newMatchedPattern(string(gh.line[gh.line_cursor:gh.line_cursor+1]), gh.patterns[gh.pattern_cursor]))
			gh.line_cursor++
			return true
		} else if gh.patterns[gh.pattern_cursor].sign == Optional {
			return true
		}
	}
	return false
}

// reset the pattern cursor, increment the search on the line
func (gh *GrepHandler) resetSearch() {
	gh.backreferences = make([]string, 0)
	gh.line_cursor_offset++
	gh.pattern_cursor = -1
	gh.line_cursor = gh.line_cursor_offset
}

// match the current pattern (its not one of the predefined ones, it's a string) with the line
func (gh *GrepHandler) matchString() bool {
	current_pattern := gh.patterns[gh.pattern_cursor]
	//if the pattern has a wildcard, we check all the bytes but the wildcard
	if patternHasAWildcard(current_pattern.pattern) {
		for temp_line_cursor, pattern_byte_cursor := gh.line_cursor, 0; temp_line_cursor < len(gh.line) && pattern_byte_cursor < len(current_pattern.pattern); temp_line_cursor, pattern_byte_cursor = temp_line_cursor+1, pattern_byte_cursor+1 {
			if current_pattern.pattern[pattern_byte_cursor] == '.' {
				continue
			} else if current_pattern.pattern[pattern_byte_cursor] != gh.line[temp_line_cursor] {
				if gh.match_start {
					return false
				}
				gh.resetSearch()
				return true
			}
		}
		//we matched the full pattern, increment the line cursor by the length of the pattern (won't work if used in combination with quantifiers)
		gh.matched_patterns = append(gh.matched_patterns, newMatchedPattern(string(gh.line[gh.line_cursor:gh.line_cursor+len(current_pattern.pattern)]), gh.patterns[gh.pattern_cursor]))
		gh.line_cursor += len(current_pattern.pattern)
		return true
	} else {
		//no wildcard, can use strings.HasPrefix to check if the pattern is contained within the subline
		match := strings.HasPrefix(string(gh.line[gh.line_cursor:]), current_pattern.pattern)
		if !match {
			if gh.pattern_cursor == 0 {
				if gh.match_start { //failure, we didn't match the beginning of the string when we HAD to
					return false
				} else {
					//we didn't match the beginning of the string, increment line cursor by 1 and restart from the 1st pattern
					gh.resetSearch()
					return true
				}
			} else {
				//we didn't match the beginning of the string, increment line cursor by 1 and restart from the 1st pattern
				gh.resetSearch()
				return true
			}
		} else { //it's a match so we increment the line cursor by the length of the pattern?
			gh.matched_patterns = append(gh.matched_patterns, newMatchedPattern(string(gh.line[gh.line_cursor:gh.line_cursor+len(current_pattern.pattern)]), gh.patterns[gh.pattern_cursor]))
			gh.line_cursor += len(current_pattern.pattern)
			return true
		}
	}
}

// (\w+ \d+) -> "\w+", " ", "\d+"
// (\w\d) -> "\w", "\d"
func (gh *GrepHandler) matchCaptureGroupSubPatterns(pattern string) bool {
	subgh := newGrepHandler(gh.line[gh.line_cursor:], pattern)
	if !subgh.ExtractPatterns() {
		panic("unable to extract capture group's subpatterns")
	}
	if gh.match_start || gh.pattern_cursor > 0 {
		subgh.match_start = true
	}
	//handle nested backreference, their index needs to be adjusted to the capture group
	var br_count int
	for x, pat := range subgh.patterns {
		if isBackReference(pat.pattern) {
			br_count++
			subgh.patterns[x].pattern = "\\" + strconv.Itoa(br_count)
		} else if isCharacterGroupMatch(pat.pattern) {
			if pat.pattern[1] == '^' {
				//is there a next pattern in subgh?
				if x+1 < len(subgh.patterns) {
					subgh.patterns[x].nextbyte = subgh.patterns[x+1].pattern[0]
				} else if gh.pattern_cursor+1 < len(gh.patterns) {
					var fb rune
					for _, b := range gh.patterns[gh.pattern_cursor+1].pattern {
						//!!!!!!!!!!! THIS NEEDS IMPROVEMENTS
						if b == '[' || b == '(' {
							continue
						} else {
							fb = b
							break
						}
					}
					subgh.patterns[x].nextbyte = byte(fb)
				}
			}
		}
	}
	if ok, _ := subgh.matchPatterns(); ok {
		if gh.match_start {
			gh.match_start = false
		}
		gh.line_cursor += subgh.line_cursor
		//grab the matched patterns
		mp := subgh.matched_patterns
		mps := make([]string, 0)
		for _, x := range mp {
			mps = append(mps, x.match)
		}
		backref := strings.Join(mps, "")
		gh.backreferences = append(gh.backreferences, backref)
		//handle nested backreferences
		gh.backreferences = append(gh.backreferences, subgh.backreferences...)
		gh.matched_patterns = append(gh.matched_patterns, newMatchedPattern(backref, gh.patterns[gh.pattern_cursor]))
		return true
	}
	return false
}

func (gh *GrepHandler) matchCaptureGroupAlternation() bool {
	//extract all the patterns
	subpatterns := strings.Split(gh.patterns[gh.pattern_cursor].pattern, "|")
	subline := gh.line[gh.line_cursor:]
	hasOneMatch := false
	pattern_found_index := -1
	var match string
	for x, pat := range subpatterns {
		match = ""
		if patternHasAWildcard(pat) {
			notAMatch := false
			for j, p := gh.line_cursor, 0; j < len(gh.line) && p < len(pat); j, p = j+1, p+1 {
				if pat[p] == '.' {
					match += string(gh.line[j])
				} else if pat[p] == gh.line[j] {
					match += string(gh.line[j])
				} else {
					notAMatch = true
					break
				}
			}
			if !notAMatch {
				hasOneMatch = true
				pattern_found_index = x
				break
			}
		} else if strings.HasPrefix(string(subline), pat) {
			hasOneMatch = true
			match += string(gh.line[gh.line_cursor : gh.line_cursor+len(subpatterns[x])])
			pattern_found_index = x
			break
		}
	}
	if hasOneMatch {
		gh.backreferences = append(gh.backreferences, match)
		gh.matched_patterns = append(gh.matched_patterns, newMatchedPattern(match, gh.patterns[gh.pattern_cursor]))
		gh.line_cursor += len(subpatterns[pattern_found_index])
		return true
	}
	return false
}

func (gh *GrepHandler) matchCharacterGroupPattern(pattern string, isPositiveMatch bool) bool {
	var part_ok bool
	charGroupPattern := getCharacterGroupPattern(pattern, isPositiveMatch)
	if gh.pattern_cursor == 0 {
		for ; gh.line_cursor < len(gh.line); gh.line_cursor++ {
			if isPositiveMatch {
				part_ok = gh.matchCharacterGroup(gh.line[gh.line_cursor:], charGroupPattern, true, gh.patterns[gh.pattern_cursor].sign)
			} else {
				part_ok = gh.matchCharacterGroup(gh.line[gh.line_cursor:], charGroupPattern, false, gh.patterns[gh.pattern_cursor].sign)
			}
			if part_ok {
				if gh.match_start {
					gh.match_start = false
				}
				return true
			} else if gh.match_start {
				return false
			}
		}
	} else {
		if isPositiveMatch {
			part_ok = gh.matchCharacterGroup(gh.line[gh.line_cursor:], charGroupPattern, true, gh.patterns[gh.pattern_cursor].sign)
		} else {
			part_ok = gh.matchCharacterGroup(gh.line[gh.line_cursor:], charGroupPattern, false, gh.patterns[gh.pattern_cursor].sign)
		}
		if part_ok {
			if gh.match_start {
				gh.match_start = false
			}
			return true
		} else if gh.match_start {
			return false
		} else {
			gh.resetSearch()
		}
		return true
	}
	return false
}

func (gh *GrepHandler) matchCharacterGroup(line, characterGroup []byte, isPositive bool, sign Sign) bool {
	hasMatched := false

	switch sign {
	case Plus: //match as many byte as we can
		for j := 0; j < len(line); j++ {
			if !isPositive {
				if contains(characterGroup, line[j]) {
					if hasMatched && !gh.match_start {
						gh.matched_patterns = append(gh.matched_patterns, newMatchedPattern(string(gh.line[gh.line_cursor:gh.line_cursor+j]), gh.patterns[gh.pattern_cursor]))
						gh.line_cursor += j
					}
					break
				}
				//it's a match
				hasMatched = true
				//grab the next pattern first byte, stop matching when it is found otherwise we run the risk to
				//greedy match until the line is over
				if line[j] == gh.patterns[gh.pattern_cursor].nextbyte {
					gh.matched_patterns = append(gh.matched_patterns, newMatchedPattern(string(gh.line[gh.line_cursor:gh.line_cursor+j]), gh.patterns[gh.pattern_cursor]))
					gh.line_cursor += j
					break
				}
				if gh.match_start {
					gh.match_start = false
				}
			} else {
				if !contains(characterGroup, line[j]) {
					if hasMatched && !gh.match_start {
						gh.matched_patterns = append(gh.matched_patterns, newMatchedPattern(string(gh.line[gh.line_cursor:gh.line_cursor+j]), gh.patterns[gh.pattern_cursor]))
						gh.line_cursor += j
					}
					break
				}
				//it's a match
				hasMatched = true
				if gh.match_start {
					gh.match_start = false
				}
			}
		}
	default: //match the first byte of the line
		if !isPositive {
			//it's a match
			if !contains(characterGroup, line[0]) {
				gh.matched_patterns = append(gh.matched_patterns, newMatchedPattern(string(line[0]), gh.patterns[gh.pattern_cursor]))
				hasMatched = true
			}
		} else {
			//it's a match
			if contains(characterGroup, line[0]) {
				gh.matched_patterns = append(gh.matched_patterns, newMatchedPattern(string(line[0]), gh.patterns[gh.pattern_cursor]))
				hasMatched = true
			}
		}
	}
	return hasMatched
}
