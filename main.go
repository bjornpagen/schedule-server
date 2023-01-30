package main

import (
	"context"
	"fmt"
	"log"
	"os"

	gpt "github.com/PullRequestInc/go-gpt3"
	"github.com/joho/godotenv"
	notion "github.com/jomei/notionapi"
)

func usage() {
	s := fmt.Sprintf("usage: %s [options]", os.Args[0])
	fmt.Fprint(os.Stderr, s)
}

func main() {
	if err := loadEnv(); err != nil {
		log.Fatalf("failed to load environment variables: %v", err)
	}

	ctx := context.Background()
	s := server{
		Notion:     *notion.NewClient(notion.Token(os.Getenv("NOTION_TOKEN"))),
		NotionRoot: os.Getenv("NOTION_ROOT_PAGE"),
		Gpt:        gpt.NewClient(os.Getenv("OPENAI_API_KEY")),
	}

	if err := s.run(ctx); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
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

	if os.Getenv("OPENAI_API_KEY") == "" {
		return fmt.Errorf("OPENAI_API_KEY is not set")
	}

	return nil
}
