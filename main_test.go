package main

import (
	"reflect"
	"testing"
)

func TestTokenize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "lowercase conversion",
			input: "Hello World",
			want:  []string{"hello", "world"},
		},
		{
			name:  "word split",
			input: "git push origin",
			want:  []string{"git", "push", "origin"},
		},
		{
			name:  "multiple spaces",
			input: "git   push    origin",
			want:  []string{"git", "push", "origin"},
		},
		{
			name:  "empty string",
			input: "",
			want:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tokenize(tt.input)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("tokenize(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestComputeDocFreq(t *testing.T) {
	tests := []struct {
		name    string
		entries []Entry
		want    map[string]int
	}{
		{
			name: "count token per document",
			entries: []Entry{
				{Description: "delete branch"},
				{Description: "delete local branch"},
			},
			want: map[string]int{
				"delete": 2,
				"branch": 2,
				"local":  1,
			},
		},
		{
			name: "repeated token in same document counts once",
			entries: []Entry{
				{Description: "git git git"},
			},
			want: map[string]int{
				"git": 1,
			},
		},
		{
			name:    "empty entries",
			entries: []Entry{},
			want:    map[string]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeDocFreq(tt.entries)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("computeDocFreq() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBM25Scoring(t *testing.T) {
	tests := []struct {
		name        string
		entries     []Entry
		query       string
		higherEntry string
		lowerEntry  string
	}{
		{
			name: "more token matches scores higher",
			entries: []Entry{
				{Description: "delete local branch"},
				{Description: "create branch"},
			},
			query:       "delete branch",
			higherEntry: "delete local branch",
			lowerEntry:  "create branch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryTokens := tokenize(tt.query)
			docFreq := computeDocFreq(tt.entries)
			totalDoc := len(tt.entries)

			totalDocLen := 0
			for _, entry := range tt.entries {
				totalDocLen += len(tokenize(entry.Description))
			}
			avgDocLen := float64(totalDocLen) / float64(totalDoc)

			var higherScore, lowerScore float64
			for _, entry := range tt.entries {
				score := computeBM25Score(entry, queryTokens, docFreq, totalDoc, avgDocLen)
				if entry.Description == tt.higherEntry {
					higherScore = score
				}
				if entry.Description == tt.lowerEntry {
					lowerScore = score
				}
			}

			if higherScore <= lowerScore {
				t.Errorf("expected %q (score: %f) to score higher than %q (score: %f)",
					tt.higherEntry, higherScore, tt.lowerEntry, lowerScore)
			}
		})
	}
}

func TestCheckPhraseMatch(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		query string
		want  bool
	}{
		{
			name:  "exact phrase match",
			text:  "delete branch",
			query: "delete branch",
			want:  true,
		},
		{
			name:  "phrase contained in text",
			text:  "delete local branch",
			query: "local branch",
			want:  true,
		},
		{
			name:  "same tokens different order",
			text:  "branch delete",
			query: "delete branch",
			want:  false,
		},
		{
			name:  "case insensitive",
			text:  "Delete Branch",
			query: "delete branch",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkPhraseMatch(tt.text, tt.query)
			if got != tt.want {
				t.Errorf("checkPhraseMatch(%q, %q) = %v, want %v", tt.text, tt.query, got, tt.want)
			}
		})
	}
}

func TestExpandSynonyms(t *testing.T) {
	tests := []struct {
		name         string
		tokens       []string
		wantContains []string
	}{
		{
			name:         "delete expands to remove",
			tokens:       []string{"delete"},
			wantContains: []string{"delete", "remove"},
		},
		{
			name:         "remove expands to delete",
			tokens:       []string{"remove"},
			wantContains: []string{"remove", "delete"},
		},
		{
			name:         "non-synonym token unchanged",
			tokens:       []string{"branch"},
			wantContains: []string{"branch"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandSynonyms(tt.tokens)
			gotMap := make(map[string]bool)
			for _, token := range got {
				gotMap[token] = true
			}

			for _, want := range tt.wantContains {
				if !gotMap[want] {
					t.Errorf("expandSynonyms(%v) = %v, expected to contain %q", tt.tokens, got, want)
				}
			}
		})
	}
}

func TestSearch(t *testing.T) {
	t.Run("PhraseMatchBonus", func(t *testing.T) {
		entries := []Entry{
			{Description: "delete branch"},
			{Description: "branch delete"},
		}
		results := search(entries, "delete branch")

		if len(results) < 2 {
			t.Fatalf("expected at least 2 results, got %d", len(results))
		}
		if results[0].Description != "delete branch" {
			t.Errorf("expected first result to be %q, got %q", "delete branch", results[0].Description)
		}
		if results[1].Description != "branch delete" {
			t.Errorf("expected second result to be %q, got %q", "branch delete", results[1].Description)
		}
	})

	t.Run("SynonymSearch", func(t *testing.T) {
		tests := []struct {
			name        string
			entries     []Entry
			query       string
			wantMatches []string
		}{
			{
				name: "delete matches remove entry",
				entries: []Entry{
					{Description: "remove branch"},
				},
				query:       "delete",
				wantMatches: []string{"remove branch"},
			},
			{
				name: "remove matches delete entry",
				entries: []Entry{
					{Description: "delete branch"},
				},
				query:       "remove",
				wantMatches: []string{"delete branch"},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				results := search(tt.entries, tt.query)

				if len(results) != len(tt.wantMatches) {
					t.Fatalf("expected %d results, got %d", len(tt.wantMatches), len(results))
				}

				for i, want := range tt.wantMatches {
					if results[i].Description != want {
						t.Errorf("expected result[%d] = %q, got %q", i, want, results[i].Description)
					}
				}
			})
		}
	})

	t.Run("ResultLimit", func(t *testing.T) {
		entries := []Entry{
			{Description: "git command one"},
			{Description: "git command two"},
			{Description: "git command three"},
			{Description: "git command four"},
			{Description: "git command five"},
		}
		results := search(entries, "git")

		if len(results) > 3 {
			t.Errorf("expected max 3 results, got %d", len(results))
		}
	})
}
