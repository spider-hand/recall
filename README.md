# recall

A CLI tool to find commands by describing what you want to do when you forget them

## Motivation

Even after years of working as a software engineer, I still find myself forgetting commands and searching for them again and again.

You could search your shell history, grep through notes, or ask ChatGPT, but sometimes that might feel like overkill.

I thought it might be helpful to have something that provides a similar UX to asking ChatGPT, but directly in the terminal.

## Installation

### Github Release

Download the latest binary from [releases](https://github.com/spider-hand/recall/releases).

### Brew

```sh
brew install recall
```

### Chocolatey

```sh
choco install recall
```

## Usage

### Basic Usage

#### Search

Search commands by describing what you want to do:

```sh
recall <query>
```

Example:

```sh
recall delete branch
```

Output example:

```sh
delete local branch
git branch -d <branch>
```

If multiple results are found, they are listed:

```sh
1) delete local branch
   git branch -d <branch>

2) delete remote branch
   git push origin --delete <branch>
   git fetch -p
```

#### Add

```sh
recall --add
```

Example:

```sh
description: delete local branch

> git branch -d <branch>
```

You can enter multiple commands. Press Enter on an empty line to finish.

Example:

```sh
description: undo last commit

> git reset --soft HEAD~1
> git reset
```

### Other Actions

```
--add               Add a new entry
--edit <query>      Edit an entry
--delete <query>    Delete an entry
--list              List all entries
--version           Show version
--help              Show help message
```

## License

[MIT](./LICENSE)
