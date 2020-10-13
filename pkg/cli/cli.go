// Merlin is a post-exploitation command and control framework.
// This file is part of Merlin.
// Copyright (C) 2019  Russel Van Tuyl

// Merlin is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// any later version.

// Merlin is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with Merlin.  If not, see <http://www.gnu.org/licenses/>.

package cli

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	// 3rd Party
	"github.com/chzyer/readline"
	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	uuid "github.com/satori/go.uuid"

	// Merlin
	merlin "github.com/Ne0nd0g/merlin/pkg"
	"github.com/Ne0nd0g/merlin/pkg/agents"
	agentAPI "github.com/Ne0nd0g/merlin/pkg/api/agents"
	listenerAPI "github.com/Ne0nd0g/merlin/pkg/api/listeners"
	"github.com/Ne0nd0g/merlin/pkg/api/messages"
	moduleAPI "github.com/Ne0nd0g/merlin/pkg/api/modules"
	"github.com/Ne0nd0g/merlin/pkg/banner"
	"github.com/Ne0nd0g/merlin/pkg/core"
	"github.com/Ne0nd0g/merlin/pkg/logging"
	"github.com/Ne0nd0g/merlin/pkg/modules"
	"github.com/Ne0nd0g/merlin/pkg/servers"
)

// Global Variables
var shellModule modules.Module
var shellAgent uuid.UUID
var shellListener listener
var shellListenerOptions map[string]string
var prompt *readline.Instance
var shellCompleter *readline.PrefixCompleter
var shellMenuContext = "main"

// MessageChannel is used to input user messages that are eventually written to STDOUT on the CLI application
var MessageChannel = make(chan messages.UserMessage)
var clientID = uuid.NewV4()

func handleMainShell(cmd []string) {
	switch cmd[0] {
	case "agent":
		if len(cmd) > 1 {
			menuAgent(cmd[1:])
		}
	case "banner":
		m := "\n"
		m += color.WhiteString(banner.MerlinBanner2)
		m += color.WhiteString("\r\n\t\t   Version: %s", merlin.Version)
		m += color.WhiteString("\r\n\t\t   Build: %s", merlin.Build)
		m += color.WhiteString("\r\n\t\t   Codename: Gandalf\n")
		MessageChannel <- messages.UserMessage{
			Level:   messages.Plain,
			Message: m,
			Time:    time.Now().UTC(),
			Error:   false,
		}
	case "help", "?":
		menuHelpMain()
	case "quit":
		if len(cmd) > 1 {
			if strings.ToLower(cmd[1]) == "-y" {
				exit()
			}
		}
		if confirm("Are you sure you want to exit the server?") {
			exit()
		}
	case "interact":
		if len(cmd) > 1 {
			i := []string{"interact"}
			i = append(i, cmd[1])
			menuAgent(i)
		}
	case "queue":
		if len(cmd) < 3 {
			MessageChannel <- messages.UserMessage{
				Level:   messages.Warn,
				Message: fmt.Sprintf("Invalid syntax."),
				Time:    time.Now().UTC(),
				Error:   false,
			}
		} else {
			if cmd[1] == "all" {
				cmd[1] = "ffffffff-ffff-ffff-ffff-ffffffffffff"
			}
			newID, err := uuid.FromString(cmd[1])
			if err != nil {
				MessageChannel <- messages.UserMessage{
					Level:   messages.Warn,
					Message: fmt.Sprintf("Invalid uuid: %s", cmd[1]),
					Time:    time.Now().UTC(),
					Error:   false,
				}
			} else {
				// Remove cmd[0:1] (queue uuid) and pass it along
				newCmd := make([]string, len(cmd)-2)
				copy(newCmd[0:], cmd[2:])
				handleAgentShell(newCmd, newID)
			}
		}
	case "listqueue":
		jobs := agents.ListQueue()
		MessageChannel <- messages.UserMessage{
			Level:   messages.Plain,
			Message: "Unassigned jobs: \n" + jobs,
			Time:    time.Now().UTC(),
			Error:   false,
		}
	case "clearqueue":
		agents.ClearQueue()
		MessageChannel <- messages.UserMessage{
			Level:   messages.Plain,
			Message: "Unassigned jobs removed",
			Time:    time.Now().UTC(),
			Error:   false,
		}
	case "listeners":
		shellMenuContext = "listenersmain"
		prompt.Config.AutoComplete = getCompleter("listenersmain")
		prompt.SetPrompt("\033[31mGandalf[\033[32mlisteners\033[31m]»\033[0m ")
	case "remove":
		if len(cmd) > 1 {
			i := []string{"remove"}
			i = append(i, cmd[1])
			menuAgent(i)
		}
	case "sessions":
		menuAgent([]string{"list"})
	case "set":
		if len(cmd) > 2 {
			switch strings.ToLower(cmd[1]) {
			case "verbose":
				if strings.ToLower(cmd[2]) == "true" {
					core.Verbose = true
					MessageChannel <- messages.UserMessage{
						Level:   messages.Success,
						Message: "Verbose output enabled",
						Time:    time.Now(),
						Error:   false,
					}
				} else if strings.ToLower(cmd[2]) == "false" {
					core.Verbose = false
					MessageChannel <- messages.UserMessage{
						Level:   messages.Success,
						Message: "Verbose output disabled",
						Time:    time.Now(),
						Error:   false,
					}
				}
			case "debug":
				if strings.ToLower(cmd[2]) == "true" {
					core.Debug = true
					MessageChannel <- messages.UserMessage{
						Level:   messages.Success,
						Message: "Debug output enabled",
						Time:    time.Now().UTC(),
						Error:   false,
					}
				} else if strings.ToLower(cmd[2]) == "false" {
					core.Debug = false
					MessageChannel <- messages.UserMessage{
						Level:   messages.Success,
						Message: "Debug output disabled",
						Time:    time.Now().UTC(),
						Error:   false,
					}
				}
			}
		}
	case "use":
		menuUse(cmd[1:])
	case "version":
		MessageChannel <- messages.UserMessage{
			Level:   messages.Plain,
			Message: color.BlueString("Merlin version: %s\n", merlin.Version),
			Time:    time.Now().UTC(),
			Error:   false,
		}
	case "":
	default:
		if len(cmd) > 1 {
			executeCommand(cmd[0], cmd[1:])
		} else {
			var x []string
			executeCommand(cmd[0], x)
		}
	}
}

func handleModuleShell(cmd []string) {
	switch cmd[0] {
	case "show":
		if len(cmd) > 1 {
			switch cmd[1] {
			case "info":
				shellModule.ShowInfo()
			case "options":
				shellModule.ShowOptions()
			}
		}
	case "info":
		shellModule.ShowInfo()
	case "set":
		if len(cmd) > 2 {
			if cmd[1] == "Agent" {
				s, err := shellModule.SetAgent(cmd[2])
				if err != nil {
					MessageChannel <- messages.UserMessage{
						Level:   messages.Warn,
						Message: err.Error(),
						Time:    time.Now().UTC(),
						Error:   true,
					}
				} else {
					MessageChannel <- messages.UserMessage{
						Level:   messages.Success,
						Message: s,
						Time:    time.Now().UTC(),
						Error:   false,
					}
				}
			} else {
				s, err := shellModule.SetOption(cmd[1], cmd[2:])
				if err != nil {
					MessageChannel <- messages.UserMessage{
						Level:   messages.Warn,
						Message: err.Error(),
						Time:    time.Now().UTC(),
						Error:   true,
					}
				} else {
					MessageChannel <- messages.UserMessage{
						Level:   messages.Success,
						Message: s,
						Time:    time.Now().UTC(),
						Error:   false,
					}
				}
			}
		}
	case "reload":
		menuSetModule(strings.TrimSuffix(strings.Join(shellModule.Path, "/"), ".json"))
	case "run":
		modMessages := moduleAPI.RunModule(shellModule)
		for _, message := range modMessages {
			MessageChannel <- message
		}
	case "back", "main":
		menuSetMain()
	case "quit":
		if len(cmd) > 1 {
			if strings.ToLower(cmd[1]) == "-y" {
				exit()
			}
		}
		if confirm("Are you sure you want to exit the server?") {
			exit()
		}
	case "unset":
		if len(cmd) >= 2 {
			s, err := shellModule.SetOption(cmd[1], nil)
			if err != nil {
				MessageChannel <- messages.UserMessage{
					Level:   messages.Warn,
					Message: err.Error(),
					Time:    time.Now().UTC(),
					Error:   true,
				}
			} else {
				MessageChannel <- messages.UserMessage{
					Level:   messages.Success,
					Message: s,
					Time:    time.Now().UTC(),
					Error:   false,
				}
			}
		}
	case "?", "help":
		menuHelpModule()
	default:
		if len(cmd) > 1 {
			executeCommand(cmd[0], cmd[1:])
		} else {
			var x []string
			executeCommand(cmd[0], x)
		}
	}
}

// Specify a custom uuid as curAgent if you don't want to use the global shellAgent
func handleAgentShell(cmd []string, curAgent uuid.UUID) {
	if uuid.Equal(uuid.Nil, curAgent) {
		curAgent = shellAgent
	}

	switch cmd[0] {
	case "back":
		menuSetMain()
	case "batchcommands":
		MessageChannel <- agentAPI.SetBatchCommands(curAgent, cmd)
	case "cd":
		MessageChannel <- agentAPI.CD(curAgent, cmd)
	case "clear", "c":
		err := agents.ClearJobs(curAgent)
		if err == nil {
			MessageChannel <- messages.UserMessage{
				Level:   messages.Success,
				Message: fmt.Sprintf("Cleared all queued commands"),
				Time:    time.Now().UTC(),
				Error:   true,
			}
		} else {
			MessageChannel <- messages.UserMessage{
				Level:   messages.Warn,
				Message: fmt.Sprintf("Error clearing queued commands: %s", err.Error()),
				Time:    time.Now().UTC(),
				Error:   true,
			}
		}
	case "download":
		MessageChannel <- agentAPI.Download(curAgent, cmd)
	case "exec":
		MessageChannel <- agentAPI.CMD(curAgent, cmd)
	case "exit":
		if len(cmd) > 1 {
			if strings.ToLower(cmd[1]) == "-y" {
				menuSetMain()
				MessageChannel <- agentAPI.Exit(curAgent, cmd)
			}
		} else {
			if confirm("Are you sure you want to exit the agent?") {
				menuSetMain()
				MessageChannel <- agentAPI.Exit(curAgent, cmd)
			}
		}
	case "?", "help":
		menuHelpAgent()
	case "inactivemultiplier":
		MessageChannel <- agentAPI.SetInactiveMultiplier(curAgent, cmd)
	case "inactivethreshold":
		MessageChannel <- agentAPI.SetInactiveThreshold(curAgent, cmd)
	case "info":
		agents.ShowInfo(curAgent)
	case "interact":
		if len(cmd) > 1 {
			i, errUUID := uuid.FromString(cmd[1])
			if errUUID != nil {
				MessageChannel <- messages.UserMessage{
					Level:   messages.Warn,
					Message: fmt.Sprintf("There was an error interacting with agent %s", cmd[1]),
					Time:    time.Now().UTC(),
					Error:   true,
				}
			} else {
				menuSetAgent(i)
			}
		}
	case "ipconfig", "ifconfig":
		MessageChannel <- agentAPI.Ifconfig(curAgent, cmd)
	case "ja3":
		MessageChannel <- agentAPI.SetJA3(curAgent, cmd)
	case "jobs":
		jobs, err := agents.ListJobs(curAgent)
		if err == nil {
			MessageChannel <- messages.UserMessage{
				Level:   messages.Success,
				Message: fmt.Sprintf("Queued commands:\n%s", strings.Join(jobs, "\n")),
				Time:    time.Now().UTC(),
				Error:   true,
			}
		} else {
			MessageChannel <- messages.UserMessage{
				Level:   messages.Warn,
				Message: fmt.Sprintf("Error retrieving queued commands: %s", err.Error()),
				Time:    time.Now().UTC(),
				Error:   true,
			}
		}
	case "kill":
		MessageChannel <- agentAPI.Kill(curAgent, cmd)
	case "killdate":
		MessageChannel <- agentAPI.SetKillDate(curAgent, cmd)
	case "ls":
		MessageChannel <- agentAPI.LS(curAgent, cmd)
	case "main":
		menuSetMain()
	case "maxretry":
		MessageChannel <- agentAPI.SetMaxRetry(curAgent, cmd)
	case "note":
		newNote := ""
		if len(cmd) > 1 {
			newNote = strings.Join(cmd[1:], " ")
		}
		err := agents.SetNote(curAgent, newNote)
		if err == nil {
			MessageChannel <- messages.UserMessage{
				Level:   messages.Success,
				Message: fmt.Sprintf("Note set to: %s", strings.Join(cmd[1:], " ")),
				Time:    time.Now().UTC(),
				Error:   true,
			}
		} else {
			MessageChannel <- messages.UserMessage{
				Level:   messages.Warn,
				Message: fmt.Sprintf("Error setting note: %s", err.Error()),
				Time:    time.Now().UTC(),
				Error:   true,
			}
		}
	case "padding":
		MessageChannel <- agentAPI.SetPadding(curAgent, cmd)
	case "ps":
		MessageChannel <- agentAPI.PS(curAgent, cmd)
	case "pwd":
		MessageChannel <- agentAPI.PWD(curAgent, cmd)
	case "quit":
		if len(cmd) > 1 {
			if strings.ToLower(cmd[1]) == "-y" {
				exit()
			}
		}
		if confirm("Are you sure you want to exit the server?") {
			exit()
		}
	case "sessions":
		menuAgent([]string{"list"})
	case "sdelete":
		MessageChannel <- agentAPI.SecureDelete(curAgent, cmd)
	case "shinject":
		MessageChannel <- agentAPI.ExecuteShellcode(curAgent, cmd)
	case "sleep":
		MessageChannel <- agentAPI.SetSleep(curAgent, cmd)
	case "status":
		status := agents.GetAgentStatus(curAgent)
		if status == "Active" {
			MessageChannel <- messages.UserMessage{
				Level:   messages.Plain,
				Message: color.GreenString("%s agent is active\n", curAgent),
				Time:    time.Now().UTC(),
				Error:   false,
			}
		} else if status == "Delayed" {
			MessageChannel <- messages.UserMessage{
				Level:   messages.Plain,
				Message: color.YellowString("%s agent is delayed\n", curAgent),
				Time:    time.Now().UTC(),
				Error:   false,
			}
		} else if status == "Dead" {
			MessageChannel <- messages.UserMessage{
				Level:   messages.Plain,
				Message: color.RedString("%s agent is dead\n", curAgent),
				Time:    time.Now().UTC(),
				Error:   false,
			}
		} else {
			MessageChannel <- messages.UserMessage{
				Level:   messages.Plain,
				Message: color.BlueString("%s agent is %s\n", curAgent, status),
				Time:    time.Now().UTC(),
				Error:   false,
			}
		}
	case "touch", "timestomp":
		MessageChannel <- agentAPI.Touch(curAgent, cmd)
	case "upload":
		MessageChannel <- agentAPI.Upload(curAgent, cmd)
	case "winexec":
		MessageChannel <- agentAPI.WinExec(curAgent, cmd)
	default:
		if len(cmd) > 1 {
			executeCommand(cmd[0], cmd[1:])
		} else {
			executeCommand(cmd[0], []string{})
		}
	}
}

// Shell is the exported function to start the command line interface
func Shell() {

	osSignalHandler()
	shellCompleter = getCompleter("main")

	printUserMessage()
	registerMessageChannel()
	getUserMessages()

	p, err := readline.NewEx(&readline.Config{
		Prompt:              "\033[31mGandalf»\033[0m ",
		HistoryFile:         "/tmp/readline.tmp",
		AutoComplete:        shellCompleter,
		InterruptPrompt:     "^C",
		EOFPrompt:           "exit",
		HistorySearchFold:   true,
		FuncFilterInputRune: filterInput,
	})

	if err != nil {
		MessageChannel <- messages.UserMessage{
			Level:   messages.Warn,
			Message: fmt.Sprintf("There was an error with the provided input: %s", err.Error()),
			Time:    time.Now().UTC(),
			Error:   true,
		}
	}
	prompt = p

	defer func() {
		err := prompt.Close()
		if err != nil {
			log.Fatal(err)
		}
	}()

	log.SetOutput(prompt.Stderr())

	for {
		line, err := prompt.Readline()
		if err == readline.ErrInterrupt {
			if confirm("Are you sure you want to exit the server?") {
				exit()
			}
		} else if err == io.EOF {
			exit()
		}

		line = strings.TrimSpace(line)
		cmd := strings.Fields(line)

		if len(cmd) > 0 {
			switch shellMenuContext {
			case "listener":
				menuListener(cmd)
			case "listenersmain":
				menuListeners(cmd)
			case "listenersetup":
				menuListenerSetup(cmd)
			case "main":
				handleMainShell(cmd)
			case "module":
				handleModuleShell(cmd)
			case "agent":
				handleAgentShell(cmd, uuid.Nil)
			}
		}
	}
}

func menuUse(cmd []string) {
	if len(cmd) > 0 {
		switch cmd[0] {
		case "module":
			if len(cmd) > 1 {
				menuSetModule(cmd[1])
			} else {
				MessageChannel <- messages.UserMessage{
					Level:   messages.Warn,
					Message: "Invalid module",
					Time:    time.Now().UTC(),
					Error:   false,
				}
			}
		case "":
		default:
			MessageChannel <- messages.UserMessage{
				Level:   messages.Note,
				Message: "Invalid 'use' command",
				Time:    time.Now().UTC(),
				Error:   false,
			}
		}
	} else {
		MessageChannel <- messages.UserMessage{
			Level:   messages.Note,
			Message: "Invalid 'use' command",
			Time:    time.Now().UTC(),
			Error:   false,
		}
	}
}

func menuAgent(cmd []string) {
	switch cmd[0] {
	case "list":
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Agent GUID", "Note", "Platform", "Host", "Transport", "Status",
			"User", "Process", "Last checkin"})
		table.SetAlignment(tablewriter.ALIGN_CENTER)
		for k, v := range agents.Agents {
			// Convert proto (i.e. h2 or hq) to user friendly string
			var proto string
			switch v.Proto {
			case "http":
				proto = "HTTP/1.1 clear-text"
			case "https":
				proto = "HTTP/1.1 over TLS"
			case "h2c":
				proto = "HTTP/2 clear-text"
			case "h2":
				proto = "HTTP/2 over TLS"
			case "http3":
				proto = "HTTP/3 (HTTP/2 over QUIC)"
			default:
				proto = fmt.Sprintf("Unknown: %s", v.Proto)
			}

			var proc string
			if v.Platform == "windows" {
				proc = v.Process[strings.LastIndex(v.Process, "\\")+1:]
			} else {
				proc = v.Process[strings.LastIndex(v.Process, "/")+1:]
			}

			lastTime := time.Since(v.StatusCheckIn) // int64 nanosecond
			lastTimeStr := fmt.Sprintf("%d:%d:%d ago",
				int(lastTime.Hours()),
				int(lastTime.Minutes())%60,
				int(lastTime.Seconds())%60)
			table.Append([]string{k.String(), v.Note, v.Platform + "/" + v.Architecture,
				v.HostName, proto, agents.GetAgentStatus(k), v.UserName,
				fmt.Sprintf("%s(%d)", proc, v.Pid), lastTimeStr})
		}
		fmt.Println()
		table.Render()
		fmt.Println()
	case "interact":
		if len(cmd) > 1 {
			i, errUUID := uuid.FromString(cmd[1])
			if errUUID != nil {
				MessageChannel <- messages.UserMessage{
					Level:   messages.Warn,
					Message: fmt.Sprintf("There was an error interacting with agent %s", cmd[1]),
					Time:    time.Now().UTC(),
					Error:   true,
				}
			} else {
				menuSetAgent(i)
			}
		}
	case "remove":
		if len(cmd) > 1 {
			i, errUUID := uuid.FromString(cmd[1])
			if errUUID != nil {
				MessageChannel <- messages.UserMessage{
					Level:   messages.Warn,
					Message: fmt.Sprintf("There was an error interacting with agent %s", cmd[1]),
					Time:    time.Now().UTC(),
					Error:   true,
				}
			} else {
				errRemove := agents.RemoveAgent(i)
				if errRemove != nil {
					MessageChannel <- messages.UserMessage{
						Level:   messages.Warn,
						Message: errRemove.Error(),
						Time:    time.Now().UTC(),
						Error:   true,
					}
				} else {
					m := fmt.Sprintf("Agent %s was removed from the server at %s",
						cmd[1], time.Now().UTC().Format(time.RFC3339))
					MessageChannel <- messages.UserMessage{
						Level:   messages.Info,
						Message: m,
						Time:    time.Now().UTC(),
						Error:   false,
					}
				}
			}
		}
	}
}

func menuSetAgent(agentID uuid.UUID) {
	for k := range agents.Agents {
		if agentID == agents.Agents[k].ID {
			shellAgent = agentID
			prompt.Config.AutoComplete = getCompleter("agent")
			prompt.SetPrompt("\033[31mGandalf[\033[32magent\033[31m][\033[33m" + shellAgent.String() + "\033[31m]»\033[0m ")
			shellMenuContext = "agent"
		}
	}
}

// menuListener handles all the logic for interacting with an instantiated listener
func menuListener(cmd []string) {
	switch strings.ToLower(cmd[0]) {
	case "back":
		shellMenuContext = "listenersmain"
		prompt.Config.AutoComplete = getCompleter("listenersmain")
		prompt.SetPrompt("\033[31mGandalf[\033[32mlisteners\033[31m]»\033[0m ")
	case "delete":
		if confirm(fmt.Sprintf("Are you sure you want to delete the %s listener?", shellListener.name)) {
			um := listenerAPI.Remove(shellListener.name)
			if !um.Error {
				shellListener = listener{}
				shellListenerOptions = nil
				shellMenuContext = "listenersmain"
				prompt.Config.AutoComplete = getCompleter("listenersmain")
				prompt.SetPrompt("\033[31mGandalf[\033[32mlisteners\033[31m]»\033[0m ")
			} else {
				MessageChannel <- um
			}
		}
	case "quit":
		if len(cmd) > 1 {
			if strings.ToLower(cmd[1]) == "-y" {
				exit()
			}
		}
		if confirm("Are you sure you want to exit the server?") {
			exit()
		}
	case "help":
		menuHelpListener()
	case "info", "show":
		um, options := listenerAPI.GetListenerConfiguredOptions(shellListener.id)
		if um.Error {
			MessageChannel <- um
			break
		}
		statusMessage := listenerAPI.GetListenerStatus(shellListener.id)
		if statusMessage.Error {
			MessageChannel <- statusMessage
			break
		}
		if options != nil {
			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"Name", "Value"})
			table.SetAlignment(tablewriter.ALIGN_LEFT)
			table.SetRowLine(true)
			table.SetBorder(true)

			for k, v := range options {
				table.Append([]string{k, v})
			}
			table.Append([]string{"Status", shellListener.status})
			table.Render()
		}
	case "main":
		menuSetMain()
	case "restart":
		MessageChannel <- listenerAPI.Restart(shellListener.id)
		um, options := listenerAPI.GetListenerConfiguredOptions(shellListener.id)
		if um.Error {
			MessageChannel <- um
			break
		}
		prompt.SetPrompt("\033[31mGandalf[\033[32mlisteners\033[31m][\033[33m" + options["Name"] + "\033[31m]»\033[0m ")
	case "set":
		MessageChannel <- listenerAPI.SetOption(shellListener.id, cmd)
	case "start":
		MessageChannel <- listenerAPI.Start(shellListener.name)
	case "status":
		MessageChannel <- listenerAPI.GetListenerStatus(shellListener.id)
	case "stop":
		MessageChannel <- listenerAPI.Stop(shellListener.name)
	default:
		if len(cmd) > 1 {
			executeCommand(cmd[0], cmd[1:])
		} else {
			var x []string
			executeCommand(cmd[0], x)
		}
	}
}

// menuListeners handles all the logic for the root Listeners menu
func menuListeners(cmd []string) {
	switch strings.ToLower(cmd[0]) {
	case "quit":
		if len(cmd) > 1 {
			if strings.ToLower(cmd[1]) == "-y" {
				exit()
			}
		}
		if confirm("Are you sure you want to exit the server?") {
			exit()
		}
	case "delete":
		if len(cmd) >= 2 {
			name := strings.Join(cmd[1:], " ")
			um := listenerAPI.Exists(name)
			if um.Error {
				MessageChannel <- um
				return
			}
			if confirm(fmt.Sprintf("Are you sure you want to delete the %s listener?", name)) {
				removeMessage := listenerAPI.Remove(name)
				MessageChannel <- removeMessage
				if removeMessage.Error {
					return
				}
				shellListener = listener{}
				shellListenerOptions = nil
				shellMenuContext = "listenersmain"
				prompt.Config.AutoComplete = getCompleter("listenersmain")
				prompt.SetPrompt("\033[31mGandalf[\033[32mlisteners\033[31m]»\033[0m ")
			}
		}
	case "help":
		menuHelpListenersMain()
	case "info":
		if len(cmd) >= 2 {
			name := strings.Join(cmd[1:], " ")
			um := listenerAPI.Exists(name)
			if um.Error {
				MessageChannel <- um
				return
			}
			r, id := listenerAPI.GetListenerByName(name)
			if r.Error {
				MessageChannel <- r
				return
			}
			if id == uuid.Nil {
				MessageChannel <- messages.UserMessage{
					Level:   messages.Warn,
					Message: "a nil Listener UUID was returned",
					Time:    time.Time{},
					Error:   true,
				}
			}
			oMessage, options := listenerAPI.GetListenerConfiguredOptions(id)
			if oMessage.Error {
				MessageChannel <- oMessage
				return
			}
			if options != nil {
				table := tablewriter.NewWriter(os.Stdout)
				table.SetHeader([]string{"Name", "Value"})
				table.SetAlignment(tablewriter.ALIGN_LEFT)
				table.SetRowLine(true)
				table.SetBorder(true)

				for k, v := range options {
					table.Append([]string{k, v})
				}
				table.Render()
			}
		}
	case "interact":
		if len(cmd) >= 2 {
			name := strings.Join(cmd[1:], " ")
			r, id := listenerAPI.GetListenerByName(name)
			if r.Error {
				MessageChannel <- r
				return
			}
			if id == uuid.Nil {
				return
			}

			status := listenerAPI.GetListenerStatus(id).Message
			shellListener = listener{
				id:     id,
				name:   name,
				status: status,
			}
			shellMenuContext = "listener"
			prompt.Config.AutoComplete = getCompleter("listener")
			prompt.SetPrompt("\033[31mGandalf[\033[32mlisteners\033[31m][\033[33m" + name + "\033[31m]»\033[0m ")
		} else {
			MessageChannel <- messages.UserMessage{
				Level:   messages.Note,
				Message: "you must select a listener to interact with",
				Time:    time.Now().UTC(),
				Error:   false,
			}
		}
	case "list":
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Name", "Interface", "Port", "Protocol", "Status", "Description"})
		table.SetAlignment(tablewriter.ALIGN_CENTER)
		listeners := listenerAPI.GetListeners()
		for _, v := range listeners {
			table.Append([]string{
				v.Name,
				v.Server.GetInterface(),
				fmt.Sprintf("%d", v.Server.GetPort()),
				servers.GetProtocol(v.Server.GetProtocol()),
				servers.GetStateString(v.Server.Status()),
				v.Description})
		}
		fmt.Println()
		table.Render()
		fmt.Println()
	case "main", "back":
		menuSetMain()
	case "start":
		if len(cmd) >= 2 {
			name := strings.Join(cmd[1:], " ")
			MessageChannel <- listenerAPI.Start(name)
		}
	case "stop":
		if len(cmd) >= 2 {
			name := strings.Join(cmd[1:], " ")
			MessageChannel <- listenerAPI.Stop(name)
		}
	case "use":
		if len(cmd) >= 2 {
			types := listenerAPI.GetListenerTypes()
			for _, v := range types {
				if strings.ToLower(cmd[1]) == v {
					shellListenerOptions = listenerAPI.GetListenerOptions(cmd[1])
					shellListenerOptions["Protocol"] = strings.ToLower(cmd[1])
					shellMenuContext = "listenersetup"
					prompt.Config.AutoComplete = getCompleter("listenersetup")
					prompt.SetPrompt("\033[31mGandalf[\033[32mlisteners\033[31m][\033[33m" + strings.ToLower(cmd[1]) + "\033[31m]»\033[0m ")
				}
			}
		}
	default:
		if len(cmd) > 1 {
			executeCommand(cmd[0], cmd[1:])
		} else {
			var x []string
			executeCommand(cmd[0], x)
		}
	}
}

// menuListenerSetup handles all of the logic for setting up a Listener
func menuListenerSetup(cmd []string) {
	switch strings.ToLower(cmd[0]) {
	case "back":
		shellMenuContext = "listenersmain"
		prompt.Config.AutoComplete = getCompleter("listenersmain")
		prompt.SetPrompt("\033[31mGandalf[\033[32mlisteners\033[31m]»\033[0m ")
	case "quit":
		if len(cmd) > 1 {
			if strings.ToLower(cmd[1]) == "-y" {
				exit()
			}
		}
		if confirm("Are you sure you want to exit the server?") {
			exit()
		}
	case "help":
		menuHelpListenerSetup()
	case "info", "show":
		if shellListenerOptions != nil {
			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"Name", "Value"})
			table.SetAlignment(tablewriter.ALIGN_LEFT)
			table.SetRowLine(true)
			table.SetBorder(true)

			for k, v := range shellListenerOptions {
				table.Append([]string{k, v})
			}
			table.Render()
		}
	case "main":
		menuSetMain()
	case "set":
		if len(cmd) >= 2 {
			for k := range shellListenerOptions {
				if cmd[1] == k {
					shellListenerOptions[k] = strings.Join(cmd[2:], " ")
					m := fmt.Sprintf("set %s to: %s", k, strings.Join(cmd[2:], " "))
					MessageChannel <- messages.UserMessage{
						Level:   messages.Success,
						Message: m,
						Time:    time.Now().UTC(),
						Error:   false,
					}
				}
			}
		}
	case "start", "run", "execute":
		um, id := listenerAPI.NewListener(shellListenerOptions)
		MessageChannel <- um
		if um.Error {
			return
		}
		if id == uuid.Nil {
			MessageChannel <- messages.UserMessage{
				Level:   messages.Warn,
				Message: "a nil Listener UUID was returned",
				Time:    time.Time{},
				Error:   true,
			}
			return
		}

		shellListener = listener{id: id, name: shellListenerOptions["Name"]}
		startMessage := listenerAPI.Start(shellListener.name)
		MessageChannel <- startMessage
		um, options := listenerAPI.GetListenerConfiguredOptions(shellListener.id)
		if um.Error {
			MessageChannel <- um
			break
		}
		shellMenuContext = "listener"
		prompt.Config.AutoComplete = getCompleter("listener")
		prompt.SetPrompt("\033[31mGandalf[\033[32mlisteners\033[31m][\033[33m" + options["Name"] + "\033[31m]»\033[0m ")
	case "stop":
		MessageChannel <- listenerAPI.Stop(shellListener.name)
	default:
		if len(cmd) > 1 {
			executeCommand(cmd[0], cmd[1:])
		} else {
			var x []string
			executeCommand(cmd[0], x)
		}
	}
}

func menuSetModule(cmd string) {
	if len(cmd) > 0 {
		mPath := path.Join(core.CurrentDir, "data", "modules", cmd+".json")
		um, m := moduleAPI.GetModule(mPath)
		if um.Error {
			MessageChannel <- um
			return
		}
		if m.Name != "" {
			shellModule = m
			prompt.Config.AutoComplete = getCompleter("module")
			prompt.SetPrompt("\033[31mGandalf[\033[32mmodule\033[31m][\033[33m" + shellModule.Name + "\033[31m]»\033[0m ")
			shellMenuContext = "module"
		}
	}
}

func menuSetMain() {
	prompt.Config.AutoComplete = getCompleter("main")
	prompt.SetPrompt("\033[31mGandalf»\033[0m ")
	shellMenuContext = "main"
}

func getCompleter(completer string) *readline.PrefixCompleter {

	// Main Menu Completer
	var main = readline.NewPrefixCompleter(
		readline.PcItem("agent",
			readline.PcItem("list"),
			readline.PcItem("interact",
				readline.PcItemDynamic(agents.GetAgentList()),
			),
		),
		readline.PcItem("banner"),
		readline.PcItem("clearqueue"),
		readline.PcItem("help"),
		readline.PcItem("interact",
			readline.PcItemDynamic(agents.GetAgentList()),
		),
		readline.PcItem("listeners"),
		readline.PcItem("listqueue"),
		readline.PcItem("queue",
			readline.PcItemDynamic(agents.GetAgentList()),
		),
		readline.PcItem("remove",
			readline.PcItemDynamic(agents.GetAgentList()),
		),
		readline.PcItem("sessions"),
		readline.PcItem("use",
			readline.PcItem("module",
				readline.PcItemDynamic(moduleAPI.GetModuleListCompleter()),
			),
		),
		readline.PcItem("version"),
	)

	// Module Menu
	var module = readline.NewPrefixCompleter(
		readline.PcItem("back"),
		readline.PcItem("help"),
		readline.PcItem("info"),
		readline.PcItem("main"),
		readline.PcItem("reload"),
		readline.PcItem("run"),
		readline.PcItem("show",
			readline.PcItem("options"),
			readline.PcItem("info"),
		),
		readline.PcItem("set",
			readline.PcItem("Agent",
				readline.PcItem("all"),
				readline.PcItemDynamic(agents.GetAgentList()),
			),
			readline.PcItemDynamic(shellModule.GetOptionsList()),
		),
		readline.PcItem("unset",
			readline.PcItemDynamic(shellModule.GetOptionsList()),
		),
	)

	// Agent Menu
	var agent = readline.NewPrefixCompleter(
		readline.PcItem("back"),
		readline.PcItem("batchcommands"),
		readline.PcItem("cd"),
		readline.PcItem("clear"),
		readline.PcItem("download"),
		readline.PcItem("exec"),
		readline.PcItem("exit"),
		readline.PcItem("help"),
		readline.PcItem("ifconfig"),
		readline.PcItem("inactivemultiplier"),
		readline.PcItem("inactivethreshold"),
		readline.PcItem("info"),
		readline.PcItem("interact",
			readline.PcItemDynamic(agents.GetAgentList()),
		),
		readline.PcItem("ipconfig"),
		readline.PcItem("ja3"),
		readline.PcItem("kill"),
		readline.PcItem("killdate"),
		readline.PcItem("jobs"),
		readline.PcItem("ls"),
		readline.PcItem("main"),
		readline.PcItem("maxretry"),
		readline.PcItem("note"),
		readline.PcItem("padding"),
		readline.PcItem("ps"),
		readline.PcItem("pwd"),
		readline.PcItem("quit"),
		readline.PcItem("sessions"),
		readline.PcItem("sdelete"),
		readline.PcItem("shinject",
			readline.PcItem("self"),
			readline.PcItem("remote"),
			readline.PcItem("RtlCreateUserThread"),
		),
		readline.PcItem("sleep"),
		readline.PcItem("status"),
		readline.PcItem("timestomp"),
		readline.PcItem("touch"),
		readline.PcItem("upload"),
		readline.PcItem("winexec"),
	)

	// Listener Menu (a specific listener)
	var listener = readline.NewPrefixCompleter(
		readline.PcItem("back"),
		readline.PcItem("delete"),
		readline.PcItem("help"),
		readline.PcItem("info"),
		readline.PcItem("main"),
		readline.PcItem("remove"),
		readline.PcItem("restart"),
		readline.PcItem("set",
			readline.PcItemDynamic(listenerAPI.GetListenerOptionsCompleter(shellListenerOptions["Protocol"])),
		),
		readline.PcItem("show"),
		readline.PcItem("start"),
		readline.PcItem("status"),
		readline.PcItem("stop"),
	)

	// Listeners Main Menu (the root menu)
	var listenersmain = readline.NewPrefixCompleter(
		readline.PcItem("back"),
		readline.PcItem("delete",
			readline.PcItemDynamic(listenerAPI.GetListenerNamesCompleter()),
		),
		readline.PcItem("help"),
		readline.PcItem("info",
			readline.PcItemDynamic(listenerAPI.GetListenerNamesCompleter()),
		),
		readline.PcItem("interact",
			readline.PcItemDynamic(listenerAPI.GetListenerNamesCompleter()),
		),
		readline.PcItem("list"),
		readline.PcItem("main"),
		readline.PcItem("start",
			readline.PcItemDynamic(listenerAPI.GetListenerNamesCompleter()),
		),
		readline.PcItem("stop",
			readline.PcItemDynamic(listenerAPI.GetListenerNamesCompleter()),
		),
		readline.PcItem("use",
			readline.PcItemDynamic(listenerAPI.GetListenerTypesCompleter()),
		),
	)

	// Listener Setup Menu
	var listenersetup = readline.NewPrefixCompleter(
		readline.PcItem("back"),
		readline.PcItem("execute"),
		readline.PcItem("help"),
		readline.PcItem("info"),
		readline.PcItem("main"),
		readline.PcItem("run"),
		readline.PcItem("set",
			readline.PcItemDynamic(listenerAPI.GetListenerOptionsCompleter(shellListenerOptions["Protocol"])),
		),
		readline.PcItem("show"),
		readline.PcItem("start"),
		readline.PcItem("stop"),
	)

	switch completer {
	case "agent":
		return agent
	case "listener":
		return listener
	case "listenersmain":
		return listenersmain
	case "listenersetup":
		return listenersetup
	case "main":
		return main
	case "module":
		return module
	default:
		return main
	}
}

func menuHelpMain() {
	MessageChannel <- messages.UserMessage{
		Level:   messages.Plain,
		Message: color.YellowString("Merlin C2 Server (version %s)\n", merlin.Version),
		Time:    time.Now().UTC(),
		Error:   false,
	}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetBorder(false)
	table.SetCaption(true, "Main Menu Help")
	table.SetHeader([]string{"Command", "Description", "Options"})

	data := [][]string{
		{"agent", "Interact with agents or list agents", "interact, list"},
		{"banner", "Print the Merlin banner", ""},
		{"listeners", "Move to the listeners menu", ""},
		{"interact", "Interact with an agent", ""},
		{"quit", "Exit and close the Merlin server", ""},
		{"remove", "Remove or delete a DEAD agent from the server"},
		{"sessions", "List all agents session information.", ""},
		{"use", "Use a function of Merlin", "module"},
		{"version", "Print the Merlin server version", ""},
	}

	table.AppendBulk(data)
	fmt.Println()
	table.Render()
	fmt.Println()
	MessageChannel <- messages.UserMessage{
		Level:   messages.Info,
		Message: "Visit the wiki for additional information https://merlin-c2.readthedocs.io/en/latest/server/menu/main.html",
		Time:    time.Now().UTC(),
		Error:   false,
	}
}

// The help menu while in the modules menu
func menuHelpModule() {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetBorder(false)
	table.SetCaption(true, "Module Menu Help")
	table.SetHeader([]string{"Command", "Description", "Options"})

	data := [][]string{
		{"back", "Return to the main menu", ""},
		{"info", "Show information about a module"},
		{"main", "Return to the main menu", ""},
		{"reload", "Reloads the module to a fresh clean state"},
		{"run", "Run or execute the module", ""},
		{"set", "Set the value for one of the module's options", "<option name> <option value>"},
		{"show", "Show information about a module or its options", "info, options"},
		{"unset", "Clear a module option to empty", "<option name>"},
	}

	table.AppendBulk(data)
	fmt.Println()
	table.Render()
	fmt.Println()
	MessageChannel <- messages.UserMessage{
		Level:   messages.Info,
		Message: "Visit the wiki for additional information https://merlin-c2.readthedocs.io/en/latest/server/menu/modules.html",
		Time:    time.Now().UTC(),
		Error:   false,
	}
}

// The help menu while in the agent menu
func menuHelpAgent() {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetBorder(false)
	table.SetCaption(true, "Agent Help Menu")
	table.SetHeader([]string{"Command", "Description", "Examples"})

	data := [][]string{
		{"back", "Return to the main menu", ""},
		{"batchcommands", "Tell an agent to run all queued jobs on checkin", ""},
		{"cd", "Change directories", "cd ../../ OR cd c:\\\\Users"},
		{"clear", "Clear all queued commands", ""},
		{"clearqueue", "Clear all queued commands that have not been sent to an agent", ""},
		{"download", "Download a file from the agent", "download <remote_file>"},
		{"exec", "Execute a command on the agent", "exec ping -c 3 8.8.8.8"},
		{"exit", "Instruct the agent to die or quit", ""},
		{"help", "Display this message", ""},
		{"inactivemultiplier", "Multiply sleep values by this number each time threshold is reached", "inactivemultiplier 10"},
		{"inactivethreshold", "Go inactive if operator is idle for this many check ins", "inactivethreshold 3"},
		{"info", "Display all information about the agent", ""},
		{"interact", "Interact with an agent", ""},
		{"ipconfig", "Display network adapter(s) information", ""},
		{"ja3", "Change agent's TLS fingerprint", "github.com/Ne0nd0g/ja3transport"},
		{"jobs", "List queued commands", ""},
		{"kill", "Kill a process", "kill <pid>"},
		{"killdate", "Set agent's killdate (UNIX epoch timestamp)", "killdate 1609480800"},
		{"listqueue", "Lists all jobs that have yet to be sent to an agent", ""},
		{"ls", "List directory contents", "ls /etc OR ls C:\\\\Users"},
		{"main", "Return to the main menu", ""},
		{"maxretry", "Set number of failed check in attempts before the agent exits", "maxretry 30"},
		{"note", "Set a custom note for this agent", "note Help This callback dead"},
		{"padding", "Set maximum number of random bytes to pad messages", "padding 4096"},
		{"ps", "Display running processes", ""},
		{"pwd", "Display the current working directory", ""},
		{"queue", "Manually send a job to a client (that may not be registered yet)", "queue 2b112337-3476-4776-86fa-250b50ac8cfc ipconfig"},
		{"quit", "Shutdown and close the server", ""},
		{"sessions", "List all agents session information.", ""},
		{"sdelete", "Secure delete a file", "sdelete C:\\\\Merlin.exe"},
		{"shinject", "Execute shellcode", "self, remote <pid>, RtlCreateUserThread <pid>"},
		{"sleep", "<min> <max> (in seconds)", "sleep 15 30"},
		{"status", "Print the current status of the agent", ""},
		{"touch", "<source> <destination>", "touch \"C:\\\\old file.txt\" C:\\\\Merlin.exe"},
		{"upload", "Upload a file to the agent", "upload <local_file> <remote_file>"},
		{"winexec", "Execute a program using Windows API calls. Does not provide stdout. Parent spoofing optional.", "winexec [-ppid 500] ping -c 3 8.8.8.8"},
	}

	table.AppendBulk(data)
	fmt.Println()
	table.Render()
	fmt.Println()
	MessageChannel <- messages.UserMessage{
		Level:   messages.Info,
		Message: "Visit the wiki for additional information https://merlin-c2.readthedocs.io/en/latest/server/menu/agents.html",
		Time:    time.Now().UTC(),
		Error:   false,
	}
}

// The help menu for the main or root Listeners menu
func menuHelpListenersMain() {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetBorder(false)
	table.SetCaption(true, "Listeners Help Menu")
	table.SetHeader([]string{"Command", "Description", "Options"})

	data := [][]string{
		{"back", "Return to the main menu", ""},
		{"delete", "Delete a named listener", "delete <listener_name>"},
		{"info", "Display all information about a listener", "info <listener_name>"},
		{"interact", "Interact with a named agent to modify it", "interact <listener_name>"},
		{"list", "List all created listeners", ""},
		{"main", "Return to the main menu", ""},
		{"start", "Start a named listener", "start <listener_name>"},
		{"stop", "Stop a named listener", "stop <listener_name>"},
		{"use", "Create a new listener by protocol type", "use [http,https,http2,http3,h2c]"},
	}

	table.AppendBulk(data)
	fmt.Println()
	table.Render()
	fmt.Println()
	MessageChannel <- messages.UserMessage{
		Level:   messages.Info,
		Message: "Visit the wiki for additional information https://merlin-c2.readthedocs.io/en/latest/server/menu/listeners.html",
		Time:    time.Now().UTC(),
		Error:   false,
	}
}

// The help menu for the main or root Listeners menu
func menuHelpListenerSetup() {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetBorder(false)
	table.SetCaption(true, "Listener Setup Help Menu")
	table.SetHeader([]string{"Command", "Description", "Options"})

	data := [][]string{
		{"back", "Return to the listeners menu", ""},
		{"execute", "Create and start the listener (alias)", ""},
		{"info", "Display all configurable information about a listener", ""},
		{"main", "Return to the main menu", ""},
		{"run", "Create and start the listener (alias)", ""},
		{"set", "Set a configurable option", "set <option_name>"},
		{"show", "Display all configurable information about a listener", ""},
		{"start", "Create and start the listener", ""},
		{"stop", "Stop the listener", ""},
	}

	table.AppendBulk(data)
	fmt.Println()
	table.Render()
	fmt.Println()
	MessageChannel <- messages.UserMessage{
		Level:   messages.Info,
		Message: "Visit the wiki for additional information https://merlin-c2.readthedocs.io/en/latest/server/menu/listeners.html",
		Time:    time.Now().UTC(),
		Error:   false,
	}
}

// The help menu for a specific, instantiated, listener
func menuHelpListener() {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetBorder(false)
	table.SetCaption(true, "Listener Help Menu")
	table.SetHeader([]string{"Command", "Description", "Options"})

	data := [][]string{
		{"back", "Return to the listeners menu", ""},
		{"delete", "Delete this listener", "delete <listener_name>"},
		{"info", "Display all configurable information the current listener", ""},
		{"main", "Return to the main menu", ""},
		{"restart", "Restart this listener", ""},
		{"set", "Set a configurable option", "set <option_name>"},
		{"show", "Display all configurable information about a listener", ""},
		{"start", "Start this listener", ""},
		{"status", "Get the server's current status", ""},
		{"stop", "Stop the listener", ""},
	}

	table.AppendBulk(data)
	fmt.Println()
	table.Render()
	fmt.Println()
	MessageChannel <- messages.UserMessage{
		Level:   messages.Info,
		Message: "Visit the wiki for additional information https://merlin-c2.readthedocs.io/en/latest/server/menu/listeners.html",
		Time:    time.Now().UTC(),
		Error:   false,
	}
}

func filterInput(r rune) (rune, bool) {
	switch r {
	// block CtrlZ feature
	case readline.CharCtrlZ:
		return r, false
	}
	return r, true
}

// confirm reads in a string and returns true if the string is y or yes but does not provide the prompt question
func confirm(question string) bool {
	reader := bufio.NewReader(os.Stdin)
	MessageChannel <- messages.UserMessage{
		Level:   messages.Plain,
		Message: color.RedString(fmt.Sprintf("%s [Yes/No]: ", question)),
		Time:    time.Now().UTC(),
		Error:   false,
	}
	response, err := reader.ReadString('\n')
	if err != nil {
		MessageChannel <- messages.UserMessage{
			Level:   messages.Warn,
			Message: fmt.Sprintf("There was an error reading the input:\r\n%s", err.Error()),
			Time:    time.Now().UTC(),
			Error:   true,
		}
	}
	response = strings.ToLower(response)
	response = strings.Trim(response, "\r\n")
	yes := []string{"y", "yes", "-y", "-Y"}

	for _, match := range yes {
		if response == match {
			return true
		}
	}
	return false
}

// exit will prompt the user to confirm if they want to exit
func exit() {
	color.Red("[!]Quitting...")
	logging.Server("Shutting down Merlin due to user input")
	os.Exit(0)
}

// Prevent the server from falling over just from an accidental Ctrl-C
func osSignalHandler() {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		if confirm("Are you sure you want to exit the server?") {
			exit()
		}
	}()
}

func executeCommand(name string, arg []string) {
	MessageChannel <- messages.UserMessage{
		Level:   messages.Info,
		Message: "Unknown command",
		Time:    time.Time{},
		Error:   false,
	}
}

func registerMessageChannel() {
	um := messages.Register(clientID)
	if um.Error {
		MessageChannel <- um
		return
	}
	if core.Debug {
		MessageChannel <- um
	}
}

func getUserMessages() {
	go func() {
		for {
			MessageChannel <- messages.GetMessageForClient(clientID)
		}
	}()
}

// printUserMessage is used to print all messages to STDOUT for command line clients
func printUserMessage() {
	go func() {
		for {
			m := <-MessageChannel
			switch m.Level {
			case messages.Info:
				fmt.Println(color.CyanString("\n[i] %s", m.Message))
			case messages.Note:
				fmt.Println(color.YellowString("\n[-] %s", m.Message))
			case messages.Warn:
				fmt.Println(color.RedString("\n[!] %s", m.Message))
			case messages.Debug:
				if core.Debug {
					fmt.Println(color.RedString("\n[DEBUG] %s", m.Message))
				}
			case messages.Success:
				fmt.Println(color.GreenString("\n[+] %s", m.Message))
			case messages.Plain:
				fmt.Println("\n" + m.Message)
			default:
				fmt.Println(color.RedString("\n[_-_] Invalid message level: %d\r\n%s", m.Level, m.Message))
			}
		}
	}()
}

type listener struct {
	id     uuid.UUID // Listener unique identifier
	name   string    // Listener unique name
	status string    // Listener server status
}
