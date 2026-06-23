package main

import "strings"

// boolFlags lists flags that don't take a value argument.
var boolFlags = map[string]bool{
	"--force": true, "-f": true,
	"--keep-presigned-params": true,
	"--dry-run":               true,
	"--yes":                   true, "-y": true,
}

// reorderArgs moves flag arguments (starting with "-") before positional arguments
// so that Go's flag package can parse them regardless of order.
func reorderArgs(args []string) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]

		// "--" marks end of flags; keep everything after as positional.
		if arg == "--" {
			positional = append(positional, args[i:]...)
			break
		}

		if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
			// Don't consume next arg if flag already has "=" value or is boolean.
			if strings.Contains(arg, "=") || boolFlags[arg] {
				continue
			}
			// Consume next arg as the flag's value if it exists and isn't a flag.
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flags = append(flags, args[i+1])
				i++
			}
		} else {
			positional = append(positional, arg)
		}
	}
	return append(flags, positional...)
}
