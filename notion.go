package main

import (
	"context"
	"fmt"
	"log"
	"os"

	notion "github.com/jomei/notionapi"
)

type taskSystemDBs struct {
	Root    string
	Issues  string
	Threads string
	Tasks   string
}

func getDBs(ctx context.Context, nc notion.Client) (dbs taskSystemDBs, err error) {
	dbs.Root = os.Getenv("NOTION_ROOT_PAGE")

	blkid := notion.BlockID(dbs.Root)
	s, err := nc.Block.GetChildren(ctx, blkid, nil)
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

	toggleChildren, err := nc.Block.GetChildren(ctx, toggle.GetID(), nil)
	if err != nil {
		return dbs, err
	}

	for _, c := range toggleChildren.Results {
		if c.GetType() == notion.BlockTypeChildDatabase {
			db, err := nc.Database.Get(ctx, notion.DatabaseID(c.GetID()))
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

func getOpenTasks(ctx context.Context, nc notion.Client, db string) (*notion.DatabaseQueryResponse, error) {
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
					// TODO: i would like this to be Equals: false, but the library we use doesn't support it
					// TODO: this is a bug in the library since it treats `false` an an uninitialized value
					// TODO: and therefore it fails the sanity check before sending the request
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

	return getTasks(ctx, nc, db, qr)
}

func getAllTasks(ctx context.Context, nc notion.Client, db string) (*notion.DatabaseQueryResponse, error) {
	qr := &notion.DatabaseQueryRequest{}

	return getTasks(ctx, nc, db, qr)
}

func getTasks(ctx context.Context, nc notion.Client, db string, qr *notion.DatabaseQueryRequest) (*notion.DatabaseQueryResponse, error) {
	res, err := nc.Database.Query(context.Background(), notion.DatabaseID(db), qr)
	if err != nil {
		fmt.Println("Error: ", err)
	}

	return res, nil
}

func parseTask(ctx context.Context, nc notion.Client, p *notion.Page) (t Task, err error) {
	t.ID = string(p.ID)
	t.Created = p.CreatedTime
	t.Updated = p.LastEditedTime

	// get the plaintext of the page
	t.Notes, err = getPagePlaintext(ctx, nc, t.ID)
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

func getPagePlaintext(ctx context.Context, nc notion.Client, id string) (string, error) {
	blkid := notion.BlockID(id)
	s, err := nc.Block.GetChildren(ctx, blkid, nil)
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
