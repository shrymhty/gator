package main

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/shrymhty/gator/internal/config"
	"github.com/shrymhty/gator/internal/database"
)

type state struct {
	db *database.Queries
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

type RSSfeed struct {
	Channel struct {
		Title string `xml:"title"`
		Link string `xml:"link"`
		Description string `xml:"description"`
		Item []RSSItem `xml:"item"`
	} `xml:"channel"`
}

type RSSItem struct {
	Title string `xml:"title"`
	Link string `xml:"link"`
	Description string `xml:"description"`
	PubDate string `xml:"pubDate"`
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

	// database connection
	db, err := sql.Open("postgres", s.cfg.DbUrl)
	if err != nil {
		fmt.Println("Error connecting to database")
	}

	dbQueries := database.New(db)
	s.db = dbQueries

	cmds := &commands{
		registered: make(map[string]func(*state, command)error),
	}

	// commands and their handlers
	cmds.register("login", handlerLogin)
	cmds.register("register", handleRegister)
	cmds.register("reset", handleReset)
	cmds.register("users", handleUsers)
	cmds.register("agg", handleAgg)

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

	name := cmd.args[0]
	_, err := s.db.GetUser(context.Background(), name)
	if err != nil {
		return fmt.Errorf("User doesn't exist. Cannot login to user. Error: %w", err)
	}

	err = s.cfg.SetUser(name)
	if err != nil {
		return err
	}
	
	fmt.Println("User has been set!")
	return nil
}

func handleRegister(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("Error: Register handler expects a single argument, the username.")
	}

	name := cmd.args[0]

	_, err := s.db.GetUser(context.Background(), name)
	if err == nil {
		return fmt.Errorf("User already exists in the database")
	}
	if err != sql.ErrNoRows {
		return fmt.Errorf("database error: %w", err)
	}

	// Create the user
	user, err := s.db.CreateUser(context.Background(), database.CreateUserParams{
		ID: uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name: name,
	})
	if err != nil {
		return fmt.Errorf("Could not create user %s. Error: %w", name, err)
	}

	err = s.cfg.SetUser(name)
	if err != nil {
		return fmt.Errorf("Error setting user %s to config. Error: %w", name, err)
	}

	fmt.Printf("User %s created sucessfully!\n", name)
	fmt.Printf("Created user:\n%v\n", user)

	return nil
}

func handleReset(s *state, cmd command) error {
	if len(cmd.args) > 0 {
		return fmt.Errorf("Extra arguments passed.")
	}

	err := s.db.DeleteUsers(context.Background())
	if err != nil {
		return fmt.Errorf("Error deleting user data. Error: %w", err)
	}

	return nil
}

func handleUsers(s *state, cmd command) error {
	if len(cmd.args) > 0 {
		return fmt.Errorf("Extra arguments passed.")
	}

	users, err := s.db.GetUsers(context.Background())
	if err != nil {
		return fmt.Errorf("Error geting user names. Error: %w", err)
	}

	for _, user := range users {
		name := user.Name
		if s.cfg.CurrentUserName == user.Name {
			name = name + " (current)"
		}
		fmt.Println("* " + name)
	}
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

func fetchFeed(ctx context.Context, feedUrl string) (*RSSfeed, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", feedUrl, nil)
	if err != nil {
		return &RSSfeed{}, fmt.Errorf("Error creating request, Error: %w", err)
	}

	req.Header.Set("User-Agent", "gator")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return &RSSfeed{}, fmt.Errorf("Error completing request, Error: %w", err)
	}

	defer resp.Body.Close()

	var RSSresponse RSSfeed
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return &RSSfeed{}, fmt.Errorf("Error reading response. Error: %w", err)
	}
	err = xml.Unmarshal(data, &RSSresponse)
	if err != nil {
		return &RSSfeed{}, fmt.Errorf("Cannot unmarshal data. Error: %w", err)
	}

	RSSresponse.Channel.Title = html.UnescapeString(RSSresponse.Channel.Title)
	RSSresponse.Channel.Description = html.UnescapeString(RSSresponse.Channel.Description)
	for i := range RSSresponse.Channel.Item {
        RSSresponse.Channel.Item[i].Title = html.UnescapeString(RSSresponse.Channel.Item[i].Title)
        RSSresponse.Channel.Item[i].Description = html.UnescapeString(RSSresponse.Channel.Item[i].Description)
    }

	return &RSSresponse, nil
}

func handleAgg(s *state, cmd command) error {
	ctx := context.Background()
	url := "https://www.wagslane.dev/index.xml"
	result, err := fetchFeed(ctx, url)
	if err != nil {
		return fmt.Errorf("Error fetching RSS Feed. Error: %w", err)
	}

	fmt.Println(result)
	return nil
}