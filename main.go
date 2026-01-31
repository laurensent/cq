package main

import (
	"fmt"
	"os"
	"strings"
)

// knownSubcommands lists cobra subcommand names and aliases.
var knownSubcommands = map[string]bool{
	"history": true, "h": true,
	"config": true,
	"models": true,
	"help":   true,
}

// flagsWithValue lists ask flags that consume the next argument as a value.
var flagsWithValue = map[string]bool{
	"-m": true, "--model": true,
}

// knownBoolFlags lists ask boolean flags that do not consume a value argument.
var knownBoolFlags = map[string]bool{
	"--raw": true, "--dry-run": true,
	"--think": true, "--search": true,
	"-h": true, "--help": true,
	"-v": true, "--version": true,
}

// passthrough collects unknown flags to forward to the claude CLI.
var passthrough []string

func main() {
	if first := firstPositionalArg(os.Args[1:]); first != "" && !knownSubcommands[first] {
		rootCmd.SetArgs(reorderArgs(os.Args[1:]))
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// firstPositionalArg returns the first non-flag argument, skipping flag values.
func firstPositionalArg(args []string) string {
	skip := false
	for _, arg := range args {
		if skip {
			skip = false
			continue
		}
		if flagsWithValue[arg] {
			skip = true
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		return arg
	}
	return ""
}

// isKnownBoolFlagWithValue checks if arg is a known bool flag with =value (e.g. --think=false).
func isKnownBoolFlagWithValue(arg string) bool {
	if i := strings.Index(arg, "="); i > 0 {
		return knownBoolFlags[arg[:i]]
	}
	return false
}

// reorderArgs extracts known ask flags, passthrough claude flags, and
// positional arguments (prompt words) from the argument list.
// ask flags go before "--", positional args go after.
// Unknown flags are collected in the passthrough global for forwarding to claude.
func reorderArgs(args []string) []string {
	var flags []string
	var positional []string
	passthrough = nil

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// ask flag with value (-m opus)
		if flagsWithValue[arg] {
			flags = append(flags, arg)
			if i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
			continue
		}

		// ask bool flag (--raw, --dry-run, --think, --search, etc.)
		if knownBoolFlags[arg] || isKnownBoolFlagWithValue(arg) {
			flags = append(flags, arg)
			continue
		}

		// Unknown flag → passthrough to claude
		if strings.HasPrefix(arg, "-") {
			passthrough = append(passthrough, arg)
			// --flag=value is self-contained
			if strings.Contains(arg, "=") {
				continue
			}
			// Consume next arg as value if it doesn't look like a flag
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				passthrough = append(passthrough, args[i])
			}
			continue
		}

		// Positional arg (prompt word)
		positional = append(positional, arg)
	}

	if len(positional) == 0 {
		return flags
	}
	out := make([]string, 0, len(flags)+1+len(positional))
	out = append(out, flags...)
	out = append(out, "--")
	out = append(out, positional...)
	return out
}
