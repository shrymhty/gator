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
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/shrymhty/gator/internal/config"
	"github.com/shrymhty/gator/internal/database"
)

type state struct {
	db *database.Queries
	cfg *config.Config
	cmds *commands
}

type command struct {
	name string
	args []string
}

type commandHandler struct {
	fn func(*state, command) error
	desc string
}

type commands struct {
	// registered map[string]func(*state, command) error
	registered map[string]commandHandler
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
		registered: make(map[string]commandHandler),
	}

	s.cmds = cmds

	// fetch feeds
	go scrapeFeeds(s)

	// commands and their handlers
	cmds.register("login", handlerLogin, "login to CLI as a specific user")
	cmds.register("register", handleRegister, "create a new user")
	cmds.register("reset", handleReset, "reset the database")
	cmds.register("users", handleUsers, "get list of existing users")
	cmds.register("agg", handleAgg, "start the feed aggregator")
	cmds.register("addfeed", middlewareUserLoggedIn(HandleAddFeed), "add feed details to database for current user")
	cmds.register("feeds", handleFeeds, "retrive feed details")
	cmds.register("follow", middlewareUserLoggedIn(handleFollow), "follow the feed for current user")
	cmds.register("following", middlewareUserLoggedIn(handleFollowing), "fetching feeds for current user")
	cmds.register("unfollow", middlewareUserLoggedIn(handleUnfollow), "unfollow feed for current user")
	cmds.register("browse", middlewareUserLoggedIn(handleBrowse), "browse followed feeds for current user")
	cmds.register("help", handleHelp, "display the help menu")

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

func HandleAddFeed(s *state, cmd command, user database.User) error {
	if len(cmd.args) < 2 {
		return fmt.Errorf("Error: not enough arguments provided")
	}

	currentUser := s.cfg.CurrentUserName

	name := cmd.args[0]
	url := cmd.args[1]

	_, err := s.db.CreateFeed(context.Background(), database.CreateFeedParams{
		ID: uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name: name,
		Url: url,
		UserID: user.ID,
	}) 
	if err != nil {
		return fmt.Errorf("Error creating feed for user %s. Error: %w", currentUser, err)
	}

	_, err = createFeedFollow(s, user.ID, url)
	if err != nil {
		return fmt.Errorf("Error creating feed follow. Error: %w", err)
	}

	return nil
}

func handleFeeds(s *state, cmd command) error {
	if len(cmd.args) > 0 {
		return fmt.Errorf("Extra arguments passed.")
	}

	feeds, err := s.db.GetFeedsAndUserNames(context.Background())
	if err != nil {
		return fmt.Errorf("Error getting feeds details. Error: %w", err)
	}

	for _, feed := range feeds {
		fmt.Printf("* Name: %s\n  URL: %s\n  User: %s\n", feed.FeedName, feed.FeedUrl, feed.UserName)
	}

	return nil
}

func handleFollow(s *state, cmd command, user database.User) error {
	if len(cmd.args) != 1 {
		return fmt.Errorf("Expects only 1 argument. Received %d.", len(cmd.args))
	}

	url := cmd.args[0]

	feed, err := s.db.GetFeedByUrl(context.Background(), url)
	if err != nil {
		return fmt.Errorf("Error fetching feed details. Error: %w", err)
	}

	_, err = createFeedFollow(s, user.ID, url)
	if err != nil {
		return fmt.Errorf("Error creating feed follow. Error: %w", err)
	}

	fmt.Println("Record created successfully: ")
	fmt.Printf("* Feed: %s\n  User: %s\n", feed.Name, user.Name)

	return nil
}

func handleFollowing(s *state, cmd command, user database.User) error {
	if len(cmd.args) > 0 {
		return fmt.Errorf("Error executing command. Command expects 0 arguments")
	}

	feeds, err := s.db.GetFeedFollowsForUser(context.Background(), user.Name)
	if err != nil {
		return fmt.Errorf("Error fetching records. Error: %w", err)
	}

	fmt.Println("Feed Names:")
	for _, feed := range feeds {
		fmt.Println(feed.FeedName)
	}

	return nil
}

func handleUnfollow(s *state, cmd command, user database.User) error {
	if len(cmd.args) != 1 {
		return fmt.Errorf("Command expects only 1 argument. Recieved %d", len(cmd.args))
	}

	url := cmd.args[0]
	
	feed, err := s.db.GetFeedByUrl(context.Background(), url)
	if err != nil {
		return fmt.Errorf("Error fetching feed details. Error: %w", err)
	}

	err = s.db.DeleteFeedFollow(context.Background(), database.DeleteFeedFollowParams{
		UserID: user.ID,
		FeedID: feed.ID,
	})
	if err != nil {
		return fmt.Errorf("Error deleting feed follow record. Error: %w", err)
	}

	return nil
}

func createFeedFollow(s *state, userID uuid.UUID, url string) (database.CreateFeedFollowRow, error) {
	feed, err := s.db.GetFeedByUrl(context.Background(), url)
    if err != nil {
        return database.CreateFeedFollowRow{}, fmt.Errorf("could not find feed: %w", err)
    }

    feedFollow, err := s.db.CreateFeedFollow(context.Background(), database.CreateFeedFollowParams{
        ID:        uuid.New(),
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
        UserID:    userID,
        FeedID:    feed.ID,
    })
    if err != nil {
        return database.CreateFeedFollowRow{}, fmt.Errorf("could not create feed follow: %w", err)
    }

    return feedFollow, nil
}

func (c *commands) run(s *state, cmd command) error {
	handler, ok := c.registered[cmd.name]
	if !ok {
		return fmt.Errorf("command %s does not exist", cmd.name)
	}

	return handler.fn(s, cmd)
}

func (c *commands) register(name string, f func(*state, command) error, description string) {
	if c.registered ==  nil {
		c.registered = make(map[string]commandHandler)
	}
	cmdHdlr := commandHandler{
		fn: f,
		desc: description,
	}
	c.registered[name] = cmdHdlr

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
	if len(cmd.args) != 1 {
		return fmt.Errorf("usage: agg <time_between_reqs> (e.g., 1m, 1h)")
	}

	time_between_reqs, err := time.ParseDuration(cmd.args[0])
	if err != nil {
        return fmt.Errorf("invalid duration format: %w", err)
    }

	fmt.Printf("Collecting feeds every %s\n", time_between_reqs)
	scrapeFeeds(s)

	ticker := time.NewTicker(time_between_reqs)
	for ; ; <-ticker.C {
		scrapeFeeds(s)
	}
}

// middleware
func middlewareUserLoggedIn(handler func(s *state, cmd command, user database.User) error) func (*state, command) error {
	return func (s *state, cmd command) error {
		user, err := s.db.GetUser(context.Background(), s.cfg.CurrentUserName)
		if err != nil {
			return fmt.Errorf("Error getting user details. Error: %w", err)
		}
		return handler(s, cmd, user)
	} 
}

func scrapeFeeds(s *state) {

	for {
		feed, err := s.db.GetNextFeedToFetch(context.Background())
		if err != nil {
			fmt.Printf("Error fetching next feed details.\n")
		}

		err = s.db.MarkFeedFetched(context.Background(), database.MarkFeedFetchedParams{
			LastFetchedAt: sql.NullTime{Time: time.Now(), Valid: true},
			ID: feed.ID,
		})
		if err != nil {
			fmt.Printf("Error marking feed as fetched.\n")
		}

		feedDetails, err := fetchFeed(context.Background(), feed.Url)
		if err != nil {
			fmt.Printf("Error fetching feed by url.")
		}

		for _, f := range feedDetails.Channel.Item {
			pubDate := parsePubDate(f.PubDate)

			_, err := s.db.CreatePost(context.Background(), database.CreatePostParams{
				ID: uuid.New(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
				Title: f.Title,
				Url: f.Link,
				Description: f.Description,
				PublishedAt: pubDate,
				FeedID: feed.ID,
			})

			if err != nil {
				if strings.Contains(err.Error(), "pq: duplicate key value violates unique constraint") {
                continue
            }
            fmt.Printf("Error saving post %s: %v\n", f.Title, err)
			}
		}
	}
}

func parsePubDate(date string) time.Time {
	formats := []string{
        time.RFC1123Z,
        time.RFC1123,
        time.RFC3339,
        "Mon, 02 Jan 2006 15:04:05 MST",
        "2006-01-02T15:04:05Z07:00",
    }

	for _, f := range formats {
        if t, err := time.Parse(f, date); err == nil {
            return t
        }
    }

    return time.Now()
}

func handleBrowse(s *state, cmd command, user database.User) error {
	limit := 2
	if len(cmd.args) == 1 {
		parsedLimit, err := strconv.Atoi(cmd.args[0])
		if err != nil {
			return fmt.Errorf("Invalid limit: %w", err)
		}
		limit = parsedLimit
	}

	posts, err := s.db.GetPostsByUser(context.Background(), database.GetPostsByUserParams{
		ID: user.ID,
		Limit: int32(limit),
	})

	if err != nil {
		return fmt.Errorf("Error fetching posts. Error: %w", err)
	}

	for _, post := range posts {
		fmt.Println(post)
	}

	return nil	
}

func handleHelp(s *state, cmd command) error {
	fmt.Printf("Usage: ./gator <command> [<args>]\n\n")
	fmt.Println("Commands:")
	for name, handler := range s.cmds.registered {
		fmt.Printf("  %-10s : %s\n", name, handler.desc)
	}
	return nil
}