package main

import (
	"context"
	"encoding/json"
	"log"
	"math/rand"
	"time"

	gpt "github.com/PullRequestInc/go-gpt3"
	notion "github.com/jomei/notionapi"
)

type server struct {
	Notion     notion.Client
	NotionRoot string
	Gpt        gpt.Client
}

func (s *server) run(ctx context.Context) error {
	dbs, err := getDBs(ctx, s.Notion)
	if err != nil {
		log.Fatalf("failed to get databases: %v", err)
	}

	// get tasks
	tasks, err := getOpenTasks(ctx, s.Notion, dbs.Tasks)
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
		task, err := parseTask(ctx, s.Notion, &t)
		if err != nil {
			log.Fatalf("failed to parse task: %v", err)
		}

		// add the task to the list
		ts = append(ts, task)
	}

	// map a random unique 4 charecter pseudorandom string to each task
	// this is used to identify the task in the GPT-3 output
	m := make(map[string]Task)
	for _, t := range ts {
		// generate a random string
		randStr := randomString(4)

		// check if the string is already in the map
		// if it is, generate a new string
		for {
			if _, ok := m[randStr]; ok {
				randStr = randomString(4)
			} else {
				break
			}
		}

		// add the task to the map
		m[randStr] = t
	}

	// cast all into a `gptPromptInputTask` struct
	var pts []gptPrioritizeInputTask
	for k, v := range m {
		pts = append(pts, gptPrioritizeInputTask{
			ID:    k,
			Name:  v.Name,
			Notes: v.Notes,
		})
	}

	// prioritize tasks with GPT-3
	out, err := gptPrioritize(ctx, s.Gpt, gptPrioritizeInput{
		DailyFocus: "",
		Tasks:      pts,
	})
	if err != nil {
		log.Fatalf("failed to prioritize tasks: %v", err)
	}

	// create a new task with duration list from the GPT-3 output
	var tds []TaskWithDuration
	for _, t := range out.Tasks {
		tds = append(tds, TaskWithDuration{
			Task:     m[t.ID],
			Duration: time.Duration(t.Minutes) * time.Minute,
		})
	}

	// drop all parent tasks from the list
	// this is because we only want to focus on the subtasks
	var tds2 []TaskWithDuration
	for _, t := range tds {
		if t.Subitems == nil {
			tds2 = append(tds2, t)
		}
	}
	tds = tds2

	// unmarshal and log the result
	data, err := json.MarshalIndent(tds, "", "  ")
	if err != nil {
		log.Fatalf("failed to marshal tasks: %v", err)
	}
	log.Printf("GPT-3 output: %s", string(data))

	// now, we're going to schedule these tasks in google calendar
	// we're going to schedule them in the order they were returned from GPT-3
	// this is because GPT-3 prioritizes the tasks in the order they should be done
	// so, we're going to schedule them in the same order

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

type TaskWithDuration struct {
	Task

	Duration time.Duration `json:"duration"`
}

func randomString(n int) string {
	var letter = []rune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

	b := make([]rune, n)
	for i := range b {
		b[i] = letter[rand.Intn(len(letter))]
	}
	return string(b)
}
