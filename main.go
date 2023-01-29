package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	notion "github.com/jomei/notionapi"
)

func usage() {
	s := fmt.Sprintf("usage: %s [options]", os.Args[0])
	fmt.Fprint(os.Stderr, s)
}

var (
	_nc *notion.Client
)

type taskSystemDBs struct {
	Root    string
	Issues  string
	Threads string
	Tasks   string
}

func main() {
	ctx := context.Background()

	if err := loadEnv(); err != nil {
		log.Fatalf("failed to load environment variables: %v", err)
	}

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

	// find the task that contains the substring "verizon" in the name
	var t Task
	for _, task := range ts {
		// using "strings.Contains" here to test
		if strings.Contains(task.Name, "brooklyn") {
			t = task
			break
		}
	}

	// marshal the task to JSON and print it
	b, err := json.Marshal(t)
	if err != nil {
		log.Fatalf("failed to marshal task: %v", err)
	}
	fmt.Printf("%s", b)
}

func loadEnv() error {
	err := godotenv.Load()
	if err != nil {
		log.Println("missing .env file, using environment variables")
	}

	if os.Getenv("NOTION_TOKEN") == "" {
		return fmt.Errorf("NOTION_TOKEN is not set")
	}
	_nc = notion.NewClient(notion.Token(os.Getenv("NOTION_TOKEN")))

	if os.Getenv("NOTION_ROOT_PAGE") == "" {
		return fmt.Errorf("NOTION_ROOT_PAGE is not set")
	}

	return nil
}

func getDBs(ctx context.Context) (dbs taskSystemDBs, err error) {
	dbs.Root = os.Getenv("NOTION_ROOT_PAGE")

	blkid := notion.BlockID(dbs.Root)
	s, err := _nc.Block.GetChildren(ctx, blkid, nil)
	if err != nil {
		return dbs, err
	}

	var toggle notion.Block
	for _, c := range s.Results {
		if c.GetType() == notion.BlockTypeToggle {
			toggle = c
			break
		}
	}
	if toggle == nil {
		return dbs, fmt.Errorf("toggle not found")
	}

	toggleChildren, err := _nc.Block.GetChildren(ctx, toggle.GetID(), nil)
	if err != nil {
		return dbs, err
	}

	for _, c := range toggleChildren.Results {
		if c.GetType() == notion.BlockTypeChildDatabase {
			db, err := _nc.Database.Get(ctx, notion.DatabaseID(c.GetID()))
			if err != nil {
				return dbs, err
			}
			switch db.Title[0].PlainText {
			case "issues":
				dbs.Issues = string(c.GetID())
			case "threads":
				dbs.Threads = string(c.GetID())
			case "tasks":
				dbs.Tasks = string(c.GetID())
			}
		}
	}

	if dbs.Issues == "" {
		return dbs, fmt.Errorf("issues database not found")
	}
	if dbs.Threads == "" {
		return dbs, fmt.Errorf("threads database not found")
	}
	if dbs.Tasks == "" {
		return dbs, fmt.Errorf("tasks database not found")
	}

	return dbs, nil
}

func getOpenTasks(ctx context.Context, db string) (*notion.DatabaseQueryResponse, error) {
	// here's the raw json for the filter:
	// {
	// 	"and": [
	// 		{
	// 			"property": "exited",
	// 			"checkbox": {
	// 				"equals": false
	// 			}
	// 		},
	// 		{
	// 			"and": [
	// 				{
	// 					"property": "thread/exited",
	// 					"checkbox": {
	// 						"does_not_contain": true
	// 					}
	// 				}
	// 			]
	// 		}
	// 	]
	// }

	qr := &notion.DatabaseQueryRequest{
		Filter: &notion.AndCompoundFilter{
			&notion.PropertyFilter{
				Property: "exited",
				Checkbox: &notion.CheckboxFilterCondition{
					DoesNotEqual: true,
				},
			},
			&notion.AndCompoundFilter{
				&notion.PropertyFilter{
					Property: "thread/exited",
					Rollup: &notion.RollupFilterCondition{
						None: &notion.RollupSubfilterCondition{
							Checkbox: &notion.CheckboxFilterCondition{
								Equals: true,
							},
						},
					},
				},
			},
		},
	}

	return getTasks(ctx, db, qr)
}

func getAllTasks(ctx context.Context, db string) (*notion.DatabaseQueryResponse, error) {
	qr := &notion.DatabaseQueryRequest{}

	return getTasks(ctx, db, qr)
}

func getTasks(ctx context.Context, db string, qr *notion.DatabaseQueryRequest) (*notion.DatabaseQueryResponse, error) {
	res, err := _nc.Database.Query(context.Background(), notion.DatabaseID(db), qr)
	if err != nil {
		fmt.Println("Error: ", err)
	}

	return res, nil
}

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

func parseTask(ctx context.Context, p *notion.Page) (t Task, err error) {
	t.ID = string(p.ID)
	t.Created = p.CreatedTime
	t.Updated = p.LastEditedTime

	// get the plaintext of the page
	t.Notes, err = getPagePlaintext(ctx, t.ID)
	if err != nil {
		return t, err
	}

	for s, p := range p.Properties {
		switch s {
		case "name":
			v, ok := p.(*notion.TitleProperty)
			if !ok {
				return t, fmt.Errorf("name property is not a title")
			}
			t.Name = richToPlainText(v.Title)
		case "parent":
			v, ok := p.(*notion.RelationProperty)
			if !ok {
				return t, fmt.Errorf("parent property is not a relation")
			}
			if len(v.Relation) == 0 {
				// since this field is optional, we can just skip it
				continue
			}
			t.Parent = string(v.Relation[0].ID)
		case "subitems":
			v, ok := p.(*notion.RelationProperty)
			if !ok {
				return t, fmt.Errorf("children property is not a relation")
			}
			for _, c := range v.Relation {
				t.Subitems = append(t.Subitems, string(c.ID))
			}
		case "exited":
			v, ok := p.(*notion.CheckboxProperty)
			if !ok {
				return t, fmt.Errorf("exited property is not a checkbox")
			}
			t.Exited = v.Checkbox
		}
	}

	return t, nil
}

func richToPlainText(r []notion.RichText) (s string) {
	for _, t := range r {
		s += t.PlainText
	}
	return s
}

func getPagePlaintext(ctx context.Context, id string) (string, error) {
	blkid := notion.BlockID(id)
	s, err := _nc.Block.GetChildren(ctx, blkid, nil)
	if err != nil {
		return "", err
	}

	if len(s.Results) == 0 {
		return "", nil
	}

	var t string
	for i, b := range s.Results {
		cm, err := blockToCommonMark(b)
		if err != nil {
			// skip unsupported block types, but log the error
			log.Println(err)
			continue
		}
		t += cm
		if i < len(s.Results)-1 {
			t += "\n"
		}
	}

	return t, nil
}

// TODO: make this more sophisticated and extract to a separate package
func blockToCommonMark(b notion.Block) (string, error) {
	switch b := b.(type) {
	case *notion.ParagraphBlock:
		return richToPlainText(b.Paragraph.RichText), nil
	case *notion.Heading1Block:
		return "# " + richToPlainText(b.Heading1.RichText), nil
	case *notion.Heading2Block:
		return "## " + richToPlainText(b.Heading2.RichText), nil
	case *notion.Heading3Block:
		return "### " + richToPlainText(b.Heading3.RichText), nil
	case *notion.BulletedListItemBlock:
		return "* " + richToPlainText(b.BulletedListItem.RichText), nil
	// case *notion.NumberedListItemBlock:
	// 	return richToPlainText(b.(*notion.NumberedListItemBlock).NumberedListItem.RichText), nil
	// case *notion.ToDoBlock:
	// 	return richToPlainText(b.(*notion.ToDoBlock).ToDo.RichText), nil
	default:
		return "", fmt.Errorf("unsupported block type: %T", b)
	}
}
