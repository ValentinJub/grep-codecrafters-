package main

import (
	"reflect"
	"testing"
)

var testSimplePatternExtraction = []struct {
	description string
	pattern     string
	expected    []string
}{
	{
		description: "single character",
		pattern:     "d",
		expected:    []string{"d"},
	},
	{
		description: "single character",
		pattern:     "w",
		expected:    []string{"w"},
	},
	{
		description: "single character",
		pattern:     "q",
		expected:    []string{"q"},
	},
	{
		description: "multiple character classes",
		pattern:     "\\d\\w",
		expected:    []string{"\\d", "\\w"},
	},
	{
		description: "character classe and single char",
		pattern:     "\\dw",
		expected:    []string{"\\d", "w"},
	},
	{
		description: "two character classes and single char",
		pattern:     "\\d\\ww",
		expected:    []string{"\\d", "\\w", "w"},
	},
	{
		description: "two character classes and single char",
		pattern:     "\\d \\ww",
		expected:    []string{"\\d", " ", "\\w", "w"},
	},
	{
		description: "character group",
		pattern:     "[abc]",
		expected:    []string{"[abc]"},
	},
	{
		description: "end of string anchor",
		pattern:     "a$",
		expected:    []string{"a"},
	},
}

var testPatternWithQuantifierExtraction = []struct {
	description string
	pattern     string
	expected    []Pattern
}{
	{
		description: "VComplex capture group",
		pattern:     "((c.t|d.g) and (f..h|b..d)), \\2 with \\3, \\1",
		expected:    []Pattern{{pattern: "((c.t|d.g) and (f..h|b..d))", sign: Alternation}, {pattern: ", "}, {pattern: "\\2"}, {pattern: " with "}, {pattern: "\\3"}, {pattern: ", "}, {pattern: "\\1"}},
	},
	{
		description: "VComplex capture group",
		pattern:     "(([abc]+)-([def]+)) is \\1, not ([^xyz]+), \\2, or \\3",
		expected:    []Pattern{{pattern: "(([abc]+)-([def]+))"}, {pattern: " is "}, {pattern: "\\1"}, {pattern: ", not "}, {pattern: "([^xyz]+)"}, {pattern: ", "}, {pattern: "\\2"}, {pattern: ", or "}, {pattern: "\\3"}},
	},
	{
		description: "VComplex capture group",
		pattern:     "('(cat) and \\2') is the same as \\1",
		expected:    []Pattern{{pattern: "('(cat) and \\2')"}, {pattern: " is the same as "}, {pattern: "\\1"}},
	},
	{
		description: "Complex capture group",
		pattern:     "(\\w+ ca+t)",
		expected:    []Pattern{{pattern: "(\\w+ ca+t)"}},
	},
	{
		description: "Complex capture group",
		pattern:     "(\\w+ \\d+) is doing \\1 times",
		expected:    []Pattern{{pattern: "(\\w+ \\d+)"}, {pattern: " is doing "}, {pattern: "\\1"}, {pattern: " times"}},
	},
	{
		description: "Complex capture group",
		pattern:     "([abcd]+) is \\1, not [^xyz]+",
		expected:    []Pattern{{pattern: "([abcd]+)"}, {pattern: " is "}, {pattern: "\\1"}, {pattern: ", not "}, {pattern: "[^xyz]", sign: Plus}},
	},
	{
		description: "Complex capture group",
		pattern:     "(\\w\\w\\w\\w \\d\\d\\d) is doing \\1 times",
		expected:    []Pattern{{pattern: "(\\w\\w\\w\\w \\d\\d\\d)"}, {pattern: " is doing "}, {pattern: "\\1"}, {pattern: " times"}},
	},
	{
		description: "Negative character group",
		pattern:     "^[^xyz]",
		expected:    []Pattern{{pattern: "[^xyz]"}},
	},
	{
		description: "single backreference",
		pattern:     "(\\w+) and \\1",
		expected:    []Pattern{{pattern: "(\\w+)"}, {pattern: " and "}, {pattern: "\\1"}},
	},
	{
		description: "single backreference",
		pattern:     "(cat) and \\1",
		expected:    []Pattern{{pattern: "(cat)"}, {pattern: " and "}, {pattern: "\\1"}},
	},
	{
		description: "one alternation",
		pattern:     "a (cat|dog)",
		expected:    []Pattern{{pattern: "a "}, {pattern: "(cat|dog)", sign: Alternation}},
	},
	{
		description: "one alternation",
		pattern:     "(a|c)",
		expected:    []Pattern{{pattern: "(a|c)", sign: Alternation}},
	},
	{
		description: "one quantifier",
		pattern:     "a+bc",
		expected:    []Pattern{{pattern: "a", sign: Plus}, {pattern: "bc"}},
	},
	{
		description: "one quantifier",
		pattern:     "ab+cd",
		expected:    []Pattern{{pattern: "a"}, {pattern: "b", sign: Plus}, {pattern: "cd"}},
	},
	{
		description: "one quantifier",
		pattern:     "ab?cd",
		expected:    []Pattern{{pattern: "a"}, {pattern: "b", sign: Optional}, {pattern: "cd"}},
	},
}

var testGrep = []struct {
	description string
	pattern     string
	line        string
	expected    bool
}{
	{
		description: "complex nested backreference",
		line:        "cat and fish, cat with fish, cat and fish",
		pattern:     "((c.t|d.g) and (f..h|b..d)), \\2 with \\3, \\1",
		expected:    true,
	},
	{
		description: "complex nested backreference",
		line:        "abc-def is abc-def, not xyz, abc, or def",
		pattern:     "(([abc]+)-([def]+)) is \\1, not ([^xyz]+), \\2, or \\3",
		expected:    false,
	},
	{
		description: "complex nested backreference",
		line:        "abc-def is abc-def, not efg, abc, or def",
		pattern:     "(([abc]+)-([def]+)) is \\1, not ([^xyz]+), \\2, or \\3",
		expected:    true,
	},
	{
		description: "nested backreference",
		line:        "grep 101 is doing grep 101 times, and again grep 101 times",
		pattern:     "((\\w\\w\\w\\w) (\\d\\d\\d)) is doing \\2 \\3 times, and again \\1 times",
		expected:    true,
	},
	{
		description: "nested backreference",
		line:        "'cat and cat' is the same as 'cat and cat'",
		pattern:     "('(cat) and \\2') is the same as \\1",
		expected:    true,
	},
	{
		description: "complex single backreference",
		line:        "bugs here and bugs there",
		pattern:     "(b..s|c..e) here and \\1 there",
		expected:    true,
	},
	{
		description: "complex single backreference",
		line:        "this starts and ends with this",
		pattern:     "^(\\w+) starts and ends with \\1$",
		expected:    true,
	},
	{
		description: "single backreference",
		line:        "abcd is abcd, not efg",
		pattern:     "([abcd]+) is \\1, not [^xyz]+",
		expected:    true,
	},
	{
		description: "single backreference",
		line:        "grep 101 is doing grep 101 times",
		pattern:     "(\\w\\w\\w\\w \\d\\d\\d) is doing \\1 times",
		expected:    true,
	},
	{
		description: "single backreference",
		pattern:     "(cat) and \\1",
		line:        "cat and cat",
		expected:    true,
	},
	{
		description: "single backreference",
		pattern:     "(cat) and \\1",
		line:        "cat and dog",
		expected:    false,
	},
	{
		description: "alternation",
		pattern:     "a (cat|dog)",
		line:        "a dog",
		expected:    true,
	},
	{
		description: "alternation",
		pattern:     "a (cat|dog)",
		line:        "a cat",
		expected:    true,
	},
	{
		description: "alternation",
		pattern:     "a (cat|dog)",
		line:        "a rat",
		expected:    false,
	},
	{
		description: "wildcard character",
		pattern:     "c.t",
		line:        "car",
		expected:    false,
	},
	{
		description: "wildcard character",
		pattern:     "c.t",
		line:        "cut",
		expected:    true,
	},
	{
		description: "optional character",
		pattern:     "ca?t",
		line:        "act",
		expected:    true,
	},
	{
		description: "match one or more characters",
		pattern:     "ca+t",
		line:        "caaats",
		expected:    true,
	},
	{
		description: "single character match",
		pattern:     "d",
		line:        "dog",
		expected:    true,
	},
	{
		description: "Combining character classes",
		pattern:     "\\d+ apple",
		line:        "sally has 33 apples",
		expected:    true,
	},
	{
		description: "Combining character classes",
		pattern:     "\\d apple",
		line:        "sally has 3 apples",
		expected:    true,
	},
	{
		description: "Combining character classes",
		pattern:     "\\d \\w\\w\\ws",
		line:        "sally has 3 dogs",
		expected:    true,
	},
	{
		description: "Combining character classes",
		pattern:     "\\d \\w\\w\\ws",
		line:        "sally has 1 dog",
		expected:    false,
	},
	{
		description: "Negative character group",
		pattern:     "^[^xyz]",
		line:        "apple",
		expected:    true,
	},
	{
		description: "Negative character group",
		pattern:     "^[^xyz]",
		line:        "xpple",
		expected:    false,
	},
	{
		description: "Negative character group",
		pattern:     "[^xyz]",
		line:        "xxe",
		expected:    true,
	},
	{
		description: "Negative character group",
		pattern:     "[^xyz]",
		line:        "apple",
		expected:    true,
	},
	{
		description: "Negative character group",
		pattern:     "[^anb]",
		line:        "banana",
		expected:    false,
	},
	{
		description: "Positive Character Group",
		pattern:     "[abcd]",
		line:        "a",
		expected:    true,
	},
	{
		description: "start of string anchor",
		pattern:     "^log",
		line:        "log",
		expected:    true,
	},
	{
		description: "start of string anchor",
		pattern:     "^log",
		line:        "slog",
		expected:    false,
	},
	{
		description: "end of string anchor",
		pattern:     "log$",
		line:        "log",
		expected:    true,
	},
	{
		description: "end of string anchor",
		pattern:     "log$",
		line:        "logs",
		expected:    false,
	},
	{
		description: "match quantifiers",
		pattern:     "\\d+c",
		line:        "33c",
		expected:    true,
	},
}

func getPatternStringFromPatterns(patterns []Pattern) []string {
	out := make([]string, len(patterns))
	for x, p := range patterns {
		out[x] = p.pattern
	}
	return out
}

func TestSimplePatternExtraction(t *testing.T) {
	for _, tp := range testSimplePatternExtraction {
		t.Run(tp.description, func(t *testing.T) {
			gh := newGrepHandler([]byte("balls"), tp.pattern)
			ok := gh.ExtractPatterns()
			if !ok {
				t.Fatal("no pattern found")
			}
			patterns := getPatternStringFromPatterns(gh.patterns)
			if !reflect.DeepEqual(patterns, tp.expected) {
				t.Fatalf("no matching pattern: got %v expected: %v", gh.patterns, tp.expected)
			}
		})
	}
}

func TestPatternWithQuantifierExtraction(t *testing.T) {
	for _, tp := range testPatternWithQuantifierExtraction {
		t.Run(tp.description, func(t *testing.T) {
			gh := newGrepHandler([]byte("balls"), tp.pattern)
			ok := gh.ExtractPatterns()
			if !ok {
				t.Fatal("no pattern found")
			} else if !reflect.DeepEqual(gh.patterns, tp.expected) {
				t.Fatalf("no matching pattern: got %v expected: %v", gh.patterns, tp.expected)
			}
		})
	}
}

func TestGreps(t *testing.T) {
	for _, tp := range testGrep {
		t.Run(tp.description, func(t *testing.T) {
			gh := newGrepHandler([]byte(tp.line), tp.pattern)
			ok := gh.ExtractPatterns()
			if !ok {
				t.Fatal("no pattern found")
			} else {
				actual, err := gh.matchPatterns()
				if err != nil {
					t.Fatalf("error returned: %s", err)
				} else if actual != tp.expected {
					t.Fatalf("failed to match %s with pattern: %s", tp.line, tp.pattern)
				}
			}
		})
	}
}
