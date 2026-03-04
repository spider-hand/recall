package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Entry struct {
	Key string `json:"key"`
	Cmd string `json:"cmd"`
}

func getDataFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "entries.json"
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

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: recall <query>")
		return
	}

	if os.Args[1] == "add" {
		addEntry()
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
