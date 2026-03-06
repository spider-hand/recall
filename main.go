package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
)

type Entry struct {
	Key string `json:"key"`
	Cmd string `json:"cmd"`
}

type ScoredEntry struct {
	Entry Entry
	Score float64
}

func getDataFilePath() string {

	localFile := "entries.json"

	if _, err := os.Stat(localFile); err == nil {
		return localFile
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return localFile
	}

	dir := home + "/.recall"

	os.MkdirAll(dir, 0755)

	return dir + "/entries.json"
}

func loadEntries() []Entry {
	file, err := os.ReadFile(getDataFilePath())
	if err != nil {
		return []Entry{}
	}

	var entries []Entry
	if err := json.Unmarshal(file, &entries); err != nil {
		return []Entry{}
	}
	return entries
}

func addEntry() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("description: ")
	key, _ := reader.ReadString('\n')
	key = strings.TrimSpace(key)

	fmt.Print("command: ")
	cmd, _ := reader.ReadString('\n')
	cmd = strings.TrimSpace(cmd)

	entries := loadEntries()

	entry := Entry{Key: key, Cmd: cmd}

	entries = append(entries, entry)

	saveEntries(entries)
}

func saveEntries(entries []Entry) {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(getDataFilePath(), data, 0644)
}

func deleteEntries(query string) {

	entries := loadEntries()

	results := search(entries, query)

	if len(results) == 0 {
		fmt.Println("no results")
		return
	}

	fmt.Println()

	for i, entry := range results {
		fmt.Printf("%d) %s\n", i+1, entry.Key)
		fmt.Printf("%*s%s\n\n", len(fmt.Sprintf("%d) ", i+1)), "", entry.Cmd)
	}

	fmt.Print("select entry to delete: ")

	var choice int
	fmt.Scanln(&choice)

	if choice < 1 || choice > len(results) {
		fmt.Println("invalid selection")
		return
	}

	target := results[choice-1]

	var updated []Entry

	for _, entry := range entries {
		if entry.Key != target.Key || entry.Cmd != target.Cmd {
			updated = append(updated, entry)
		}
	}

	saveEntries(updated)

	fmt.Println("deleted")
}

func listEntries() {

	entries := loadEntries()

	if len(entries) == 0 {
		fmt.Println("no entries")
		return
	}

	for i, entry := range entries {
		fmt.Printf("%d) %s\n", i+1, entry.Key)
		fmt.Printf("%*s%s\n\n", len(fmt.Sprintf("%d) ", i+1)), "", entry.Cmd)
	}
}

// tokenize converts a text into lowercase tokens split by whitespace
func tokenize(text string) []string {
	text = strings.ToLower(text)
	return strings.Fields(text)
}

// computeDocumentFrequency calculates how many documents contain each token.
// Each token is counted once per entry.
func computeDocFreq(entries []Entry) map[string]int {
	docFreq := make(map[string]int)

	for _, entry := range entries {

		seenTokens := make(map[string]bool)

		tokens := tokenize(entry.Key)

		for _, token := range tokens {
			if !seenTokens[token] {
				docFreq[token]++
				seenTokens[token] = true
			}
		}
	}

	return docFreq
}

// computeBM25Score computes the BM25 relevance score between the query tokens
// and the description of a single entry.
//
// BM25 is a ranking function widely used in search engines.
func computeBM25Score(
	entry Entry,
	queryTokens []string,
	docFreq map[string]int,
	totalDoc int,
	avgDocLen float64,
) float64 {

	k1 := 1.5
	b := 0.75

	entryTokens := tokenize(entry.Key)

	docLen := float64(len(entryTokens))

	tokenFreq := make(map[string]int)

	for _, token := range entryTokens {
		tokenFreq[token]++
	}

	score := 0.0

	for _, token := range queryTokens {

		freq := tokenFreq[token]
		if freq == 0 {
			continue
		}

		tokenDocFreq := docFreq[token]

		idf := math.Log(
			(float64(totalDoc)-float64(tokenDocFreq)+0.5)/
				(float64(tokenDocFreq)+0.5) + 1,
		)

		numerator := float64(freq) * (k1 + 1)

		denominator := float64(freq) +
			k1*(1-b+b*(docLen/avgDocLen))

		score += idf * (numerator / denominator)
	}

	return score
}

// checkPhraseMatch checks if the query is a substring of the text.
// Since BM25 doesn't consider the order of tokens,
// this function is used to give a higher score to entries that contain the exact query as a phrase.
func checkPhraseMatch(text string, query string) bool {
	text = strings.ToLower(text)
	query = strings.ToLower(query)

	return strings.Contains(text, query)
}

// search ranks entries by BM25 relevance to the query and returns them sorted
// from highest score to lowest.
func search(entries []Entry, query string) []Entry {

	queryTokens := tokenize(query)

	docFreq := computeDocFreq(entries)

	totalDoc := len(entries)

	totalDocLen := 0

	for _, entry := range entries {
		totalDocLen += len(tokenize(entry.Key))
	}

	avgDocLen := float64(totalDocLen) / float64(totalDoc)

	var scoredEntries []ScoredEntry

	for _, entry := range entries {

		score := computeBM25Score(
			entry,
			queryTokens,
			docFreq,
			totalDoc,
			avgDocLen,
		)

		if checkPhraseMatch(entry.Key, query) {
			score += 1.5
		}

		if score > 0 {
			scoredEntries = append(scoredEntries, ScoredEntry{
				Entry: entry,
				Score: score,
			})
		}
	}

	sort.Slice(scoredEntries, func(i, j int) bool {
		return scoredEntries[i].Score > scoredEntries[j].Score
	})

	var results []Entry

	for _, scoredEntry := range scoredEntries {
		results = append(results, scoredEntry.Entry)
	}

	const maxResults = 3

	if len(results) > maxResults {
		results = results[:maxResults]
	}

	return results
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: recall <query>")
		return
	}

	args := os.Args[1:]

	if args[0] == "--add" {
		addEntry()
		return
	}

	if args[0] == "--delete" {
		if len(args) < 2 {
			fmt.Println("usage: recall --delete <query>")
			return
		}

		query := strings.Join(args[1:], " ")
		deleteEntries(query)
		return
	}

	if args[0] == "--list" {
		listEntries()
		return
	}

	query := strings.Join(args, " ")

	entries := loadEntries()
	results := search(entries, query)

	if len(results) == 0 {
		fmt.Println("no results")
		return
	}

	if len(results) == 1 {
		fmt.Println(results[0].Cmd)
		return
	}

	for i, entry := range results {
		fmt.Printf("%d) %s\n", i+1, entry.Cmd)
	}
}
