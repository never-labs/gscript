package main

import (
	"fmt"
	"io"
	"sort"
)

type cliHelpTopic struct {
	Command string
	Usage   string
	Summary string
}

func runHelpCommand(args []string, outw, errw io.Writer) int {
	topics := cliHelpTopics()
	if len(args) > 1 {
		fmt.Fprintln(errw, "usage: leia help [command]")
		return 2
	}
	if len(args) == 1 {
		topic, ok := topics[args[0]]
		if !ok {
			fmt.Fprintf(errw, "leia help: unknown command %q\n", args[0])
			return 2
		}
		fmt.Fprintf(outw, "%s\n\n%s\n", topic.Usage, topic.Summary)
		return 0
	}

	fmt.Fprintln(outw, "usage: leia <command> [args]")
	fmt.Fprintln(outw)
	fmt.Fprintln(outw, "Commands:")
	commands := make([]string, 0, len(topics))
	for command := range topics {
		commands = append(commands, command)
	}
	sort.Strings(commands)
	for _, command := range commands {
		topic := topics[command]
		fmt.Fprintf(outw, "  %-12s %s\n", command, topic.Summary)
	}
	fmt.Fprintln(outw)
	fmt.Fprintln(outw, "Use `leia help <command>` for command-specific usage.")
	return 0
}
