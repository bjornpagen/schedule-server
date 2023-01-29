package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
)

func usage() {
	s := fmt.Sprintf("usage: %s [options]", os.Args[0])
	fmt.Fprint(os.Stderr, s)
}

func main() {
	ctx := context.Background()

	if err := loadEnv(); err != nil {
		log.Fatalf("failed to load environment variables: %v", err)
	}

	setNotionClient(os.Getenv("NOTION_TOKEN"))

	dbs, err := getDBs(ctx)
	if err != nil {
		log.Fatalf("failed to get databases: %v", err)
	}

	// get tasks
	tasks, err := getOpenTasks(ctx, dbs.Tasks)
	if err != nil {
		log.Fatal(err)
	}

	// check if there are any tasks
	if len(tasks.Results) == 0 {
		log.Fatalf("failed to get open tasks: %v", err)
	}

	// parse all the tasks
	var ts []Task
	for _, t := range tasks.Results {
		// parse the task
		task, err := parseTask(ctx, &t)
		if err != nil {
			log.Fatalf("failed to parse task: %v", err)
		}

		// add the task to the list
		ts = append(ts, task)
	}

	// cast all into a `gptPromptInputTask` struct
	var pts []gptPrioritizeInputTask
	for _, t := range ts {
		pts = append(pts, gptPrioritizeInputTask{
			ID:    t.ID,
			Name:  t.Name,
			Notes: t.Notes,
		})
	}

	// print the tasks
	b, err := json.Marshal(pts)
	if err != nil {
		log.Fatalf("failed to marshal tasks: %v", err)
	}
	fmt.Println(string(b))
}

func loadEnv() error {
	err := godotenv.Load()
	if err != nil {
		log.Println("missing .env file, using environment variables")
	}

	if os.Getenv("NOTION_TOKEN") == "" {
		return fmt.Errorf("NOTION_TOKEN is not set")
	}

	if os.Getenv("NOTION_ROOT_PAGE") == "" {
		return fmt.Errorf("NOTION_ROOT_PAGE is not set")
	}

	return nil
}

// here is a `Task` struct that represents a single todo list item
type Task struct {
	ID       string    `json:"id"`       // notion page id
	Name     string    `json:"name"`     // notion page title
	Notes    string    `json:"notes"`    // commonmark of the page
	Parent   string    `json:"parent"`   // notion parent page id
	Subitems []string  `json:"subitems"` // notion subitem page ids
	Exited   bool      `json:"exited"`   // notion exited checkbox
	Created  time.Time `json:"created"`  // notion created time
	Updated  time.Time `json:"updated"`  // notion updated time
}
