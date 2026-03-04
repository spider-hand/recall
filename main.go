package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Entry struct {
	Key string
	Cmd string
}

func loadEntries() []Entry {
	file, err := os.ReadFile("entries.json")
	if err != nil {
		return []Entry{}
	}

	var entries []Entry
	if err := json.Unmarshal(file, &entries); err != nil {
		return []Entry{}
	}
	return entries
}

func search(entries []Entry, query string) []Entry {
	query = strings.ToLower(query)

	var results []Entry

	for _, entry := range entries {
		if strings.Contains(strings.ToLower(entry.Key), query) {
			results = append(results, entry)
		}
	}
	return results
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: recall <query>")
		return
	}
	query := strings.Join(os.Args[1:], " ")

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
