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
	Description string   `json:"description"`
	Commands    []string `json:"commands"`
}

type ScoredEntry struct {
	Entry Entry
	Score float64
}

func getDataFilePath() string {

	if envPath := os.Getenv("DATA_DIR"); envPath != "" {
		return envPath + "/entries.json"
	}

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
	description, _ := reader.ReadString('\n')
	description = strings.TrimSpace(description)

	fmt.Println("commands (empty line to finish):")
	var commands []string
	for {
		fmt.Print("> ")
		cmd, _ := reader.ReadString('\n')
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			break
		}
		commands = append(commands, cmd)
	}

	if len(commands) == 0 {
		fmt.Println("no commands added")
		return
	}

	entries := loadEntries()

	entry := Entry{Description: description, Commands: commands}

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

func editEntry(query string) {

	entries := loadEntries()

	results := search(entries, query)

	if len(results) == 0 {
		fmt.Println("no results")
		return
	}

	printEntries(results)

	fmt.Print("select entry to edit: ")

	var choice int
	fmt.Scanln(&choice)

	if choice < 1 || choice > len(results) {
		fmt.Println("invalid selection")
		return
	}

	target := results[choice-1]

	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("description (%s): ", target.Description)
	newDescription, _ := reader.ReadString('\n')
	newDescription = strings.TrimSpace(newDescription)

	if newDescription == "" {
		newDescription = target.Description
	}

	fmt.Println("current commands:")
	for i, cmd := range target.Commands {
		fmt.Printf("  %d) %s\n", i+1, cmd)
	}
	fmt.Println("new commands (empty line to finish):")
	var newCommands []string
	for {
		fmt.Print("> ")
		cmd, _ := reader.ReadString('\n')
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			break
		}
		newCommands = append(newCommands, cmd)
	}

	if len(newCommands) == 0 {
		newCommands = target.Commands
	}

	for i, entry := range entries {
		if entry.Description == target.Description {
			entries[i].Description = newDescription
			entries[i].Commands = newCommands
			break
		}
	}

	saveEntries(entries)

	fmt.Println("updated")
}

func deleteEntries(query string) {

	entries := loadEntries()

	results := search(entries, query)

	if len(results) == 0 {
		fmt.Println("no results")
		return
	}

	fmt.Println()

	printEntries(results)

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
		if entry.Description != target.Description {
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

	printEntries(entries)
}

// tokenize converts a text into lowercase tokens split by whitespace
func tokenize(text string) []string {
	text = strings.ToLower(text)
	return strings.Fields(text)
}

var synonymMap = map[string][]string{
	// delete/remove
	"remove": {"delete"},
	"delete": {"remove"},
	// create/add/new/make
	"create": {"add", "new", "make"},
	"add":    {"create", "new", "make"},
	"new":    {"create", "add", "make"},
	"make":   {"create", "add", "new"},
	// update/edit/modify/change
	"update": {"edit", "modify", "change"},
	"edit":   {"update", "modify", "change"},
	"modify": {"update", "edit", "change"},
	"change": {"update", "edit", "modify"},
	// list/show/display/view
	"list":    {"show", "display", "view"},
	"show":    {"list", "display", "view"},
	"display": {"list", "show", "view"},
	"view":    {"list", "show", "display"},
	// start/run/execute/launch
	"start":   {"run", "execute", "launch"},
	"run":     {"start", "execute", "launch"},
	"execute": {"start", "run", "launch"},
	"launch":  {"start", "run", "execute"},
	// stop/kill/terminate/end
	"stop":      {"kill", "terminate", "end"},
	"kill":      {"stop", "terminate", "end"},
	"terminate": {"stop", "kill", "end"},
	"end":       {"stop", "kill", "terminate"},
	// install/setup/configure
	"install":   {"setup", "configure"},
	"setup":     {"install", "configure"},
	"configure": {"install", "setup"},
}

// expandSynonyms expands query tokens by adding synonyms from synonymMap.
// This improves search recall so "remove" also matches entries with "delete".
func expandSynonyms(tokens []string) []string {
	expanded := make(map[string]bool)
	for _, token := range tokens {
		expanded[token] = true
		if synonyms, ok := synonymMap[token]; ok {
			for _, syn := range synonyms {
				expanded[syn] = true
			}
		}
	}
	result := make([]string, 0, len(expanded))
	for token := range expanded {
		result = append(result, token)
	}
	return result
}

// computeDocumentFrequency calculates how many documents contain each token.
// Each token is counted once per entry.
func computeDocFreq(entries []Entry) map[string]int {
	docFreq := make(map[string]int)

	for _, entry := range entries {

		seenTokens := make(map[string]bool)

		tokens := tokenize(entry.Description)

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

	entryTokens := tokenize(entry.Description)

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
	queryTokens = expandSynonyms(queryTokens)

	docFreq := computeDocFreq(entries)

	totalDoc := len(entries)

	totalDocLen := 0

	for _, entry := range entries {
		totalDocLen += len(tokenize(entry.Description))
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

		if checkPhraseMatch(entry.Description, query) {
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

func printEntries(entries []Entry) {
	for i, entry := range entries {
		fmt.Printf("%d) %s\n", i+1, entry.Description)
		indent := len(fmt.Sprintf("%d) ", i+1))
		for _, cmd := range entry.Commands {
			fmt.Printf("%*s%s\n", indent, "", cmd)
		}
		fmt.Println()
	}
}

var version = "dev"

func main() {
	args := os.Args[1:]

	if len(args) == 0 || args[0] == "--help" {
		fmt.Println("Usage:")
		fmt.Println("  recall <query>   Search commands by description")
		fmt.Println("  recall --add            Add a new entry")
		fmt.Println("  recall --edit <query>   Edit an entry")
		fmt.Println("  recall --delete <query> Delete an entry")
		fmt.Println("  recall --list           List all entries")
		fmt.Println("  recall --version        Show version")
		fmt.Println("  recall --help           Show help message")
		return
	}

	if args[0] == "--version" {
		fmt.Println("recall", version)
		return
	}

	if args[0] == "--add" {
		addEntry()
		return
	}

	if args[0] == "--edit" {
		if len(args) < 2 {
			fmt.Println("usage: recall --edit <query>")
			return
		}

		query := strings.Join(args[1:], " ")
		editEntry(query)
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
		for _, cmd := range results[0].Commands {
			fmt.Println(cmd)
		}
		return
	}

	printEntries(results)
}
