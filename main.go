package main

import (
	"fmt"
	"os"
	"strings"

	api "github.com/quackduck/devzat/devzatapi"
)

const botName = "\x1b[1;36mgithub\x1b[0m"

const helpText = "**gh** — look up GitHub issues and pull requests\n" +
	"```\n" +
	"gh issue <owner/repo> <number>   Show an issue\n" +
	"gh pr    <owner/repo> <number>   Show a pull request\n" +
	"gh issues <owner/repo>           List open issues\n" +
	"gh prs   <owner/repo>            List open pull requests\n" +
	"gh help                          Show this help\n" +
	"```\n" +
	"Set `GITHUB_DEFAULT_REPO` to omit <owner/repo> (e.g. `gh pr 123`)."

const usage = `Usage:
  devzat-github <host:port>        Connect and serve the 'gh' command
  devzat-github --print <args...>  Format one lookup and print it (no server)

Configuration (environment):
  DEVZAT_ADDRESS   Server plugin-API address, e.g. localhost:5556 (arg overrides)
  DEVZAT_TOKEN     Plugin API token (mint with 'grant <you> github-bot' in chat)
  GITHUB_TOKEN     Optional; raises GitHub API limits and enables private repos
  GITHUB_DEFAULT_REPO  Optional owner/repo so 'gh pr 123' works without it

Example:
  devzat-github --print pr quackduck/devzat 5
`

func main() {
	args := os.Args[1:]
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Print(usage)
		return
	}
	if len(args) > 0 && args[0] == "--print" {
		runPrint(strings.Join(args[1:], " "))
		return
	}
	run()
}

func runPrint(args string) {
	client := newGHClient(os.Getenv("GITHUB_TOKEN"), os.Getenv("GITHUB_DEFAULT_REPO"))
	msg, err := handle(client, args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Println(msg)
}

func run() {
	address := os.Getenv("DEVZAT_ADDRESS")
	token := os.Getenv("DEVZAT_TOKEN")
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		address = os.Args[1]
	}
	if address == "" {
		fmt.Fprintln(os.Stderr, "no server address: pass it as the first argument or set DEVZAT_ADDRESS")
		os.Exit(1)
	}
	if token == "" {
		fmt.Fprintln(os.Stderr, "no token: set DEVZAT_TOKEN (mint one with 'grant <you> github-bot' in Devzat)")
		os.Exit(1)
	}

	client := newGHClient(os.Getenv("GITHUB_TOKEN"), os.Getenv("GITHUB_DEFAULT_REPO"))

	s, err := api.NewSession(address, token)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error creating session:", err)
		os.Exit(1)
	}
	defer s.Close()

	err = s.RegisterCmd("gh", "<issue|pr|issues|prs> <owner/repo> [number]", "Look up GitHub issues and pull requests", func(cmd api.CmdCall, e error) {
		if e != nil {
			return
		}
		msg, herr := handle(client, cmd.Args)
		if herr != nil {
			msg = "⚠️ " + herr.Error()
		}
		if msg == "" {
			return
		}
		if err := s.SendMessage(api.Message{Room: cmd.Room, From: botName, Data: msg}); err != nil {
			fmt.Fprintln(os.Stderr, "error sending message:", err)
		}
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error registering command:", err)
		os.Exit(1)
	}

	fmt.Printf("github plugin connected to %s\n", address)
	if err := <-s.ErrorChan; err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
