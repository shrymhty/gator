package main

import (
	"fmt"
	"os"

	"github.com/shrymhty/gator/internal/config"
)

type state struct {
	cfg *config.Config
}

// var supportedCommands map[string]command

type command struct {
	name string
	// description string
	args []string
}

type commands struct {
	registered map[string]func(*state, command) error
}

func main() {
	cfg, err := config.ReadConfig()
	if err != nil {
		fmt.Println("Error reading config:", err)
        return
	}
	
	s := &state{
		cfg: cfg,
	}

	cmds := &commands{
		registered: make(map[string]func(*state, command)error),
	}

	cmds.register("login", handlerLogin)

	if len(os.Args) < 2 {
		fmt.Println("Error: not enough arguments provided")
		os.Exit(1)
	}

	cmdName := os.Args[1]
	cmdArgs := os.Args[2:]

	cmd := command{
		name: cmdName,
		args: cmdArgs,
	}

	err = cmds.run(s, cmd)
	if err != nil {
		fmt.Println("Error: ", err)
		os.Exit(1)
	}
}

func handlerLogin(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("Error: Login handler expects a single argument, the username.")
	}
	err := s.cfg.SetUser(cmd.args[0])
	if err != nil {
		return err
	}
	
	fmt.Println("User has been set!")
	return nil
}

func (c *commands) run(s *state, cmd command) error {
	handler, ok := c.registered[cmd.name]
	if !ok {
		return fmt.Errorf("command %s does not exist", cmd.name)
	}

	return handler(s, cmd)
}

func (c *commands) register(name string, f func(*state, command) error) {
	if c.registered ==  nil {
		c.registered = make(map[string]func(*state, command) error)
	}
	c.registered[name] = f
}