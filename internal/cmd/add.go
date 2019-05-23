package cmd

import (
	"fmt"
	nurl "net/url"
	fp "path/filepath"
	"strings"
	"time"

	"github.com/go-shiori/go-readability"
	"github.com/go-shiori/shiori/internal/model"
	"github.com/spf13/cobra"
)

func addCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add url",
		Short: "Bookmark the specified URL",
		Args:  cobra.ExactArgs(1),
		Run:   addHandler,
	}

	cmd.Flags().StringP("title", "i", "", "Custom title for this bookmark.")
	cmd.Flags().StringP("excerpt", "e", "", "Custom excerpt for this bookmark.")
	cmd.Flags().StringSliceP("tags", "t", []string{}, "Comma-separated tags for this bookmark.")
	cmd.Flags().BoolP("offline", "o", false, "Save bookmark without fetching data from internet.")

	return cmd
}

func addHandler(cmd *cobra.Command, args []string) {
	// Read flag and arguments
	url := args[0]
	title, _ := cmd.Flags().GetString("title")
	excerpt, _ := cmd.Flags().GetString("excerpt")
	tags, _ := cmd.Flags().GetStringSlice("tags")
	offline, _ := cmd.Flags().GetBool("offline")

	// Clean up URL by removing its fragment and UTM parameters
	tmp, err := nurl.Parse(url)
	if err != nil || tmp.Scheme == "" || tmp.Hostname() == "" {
		cError.Println("URL is not valid")
		return
	}

	tmp.Fragment = ""
	clearUTMParams(tmp)

	// Create bookmark item
	book := model.Bookmark{
		URL:     tmp.String(),
		Title:   normalizeSpace(title),
		Excerpt: normalizeSpace(excerpt),
	}

	// Create bookmark ID
	book.ID, err = DB.CreateNewID("bookmark")
	if err != nil {
		cError.Printf("Failed to create ID: %v\n", err)
		return
	}

	// Set bookmark tags
	book.Tags = make([]model.Tag, len(tags))
	for i, tag := range tags {
		book.Tags[i].Name = strings.TrimSpace(tag)
	}

	// If it's not offline mode, fetch data from internet
	var imageURL string

	if !offline {
		func() {
			cInfo.Println("Downloading article...")

			resp, err := httpClient.Get(url)
			if err != nil {
				cError.Printf("Failed to download article: %v\n", err)
				return
			}
			defer resp.Body.Close()

			article, err := readability.FromReader(resp.Body, url)
			if err != nil {
				cError.Printf("Failed to parse article: %v\n", err)
				return
			}

			book.Author = article.Byline
			book.Content = article.TextContent
			book.HTML = article.Content

			// If title and excerpt doesnt have submitted value, use from article
			if book.Title == "" {
				book.Title = article.Title
			}

			if book.Excerpt == "" {
				book.Excerpt = article.Excerpt
			}

			// Get image URL
			if article.Image != "" {
				imageURL = article.Image
			} else if article.Favicon != "" {
				imageURL = article.Favicon
			}
		}()
	}

	// Make sure title is not empty
	if book.Title == "" {
		book.Title = book.URL
	}

	// Save bookmark to database
	_, err = DB.SaveBookmarks(book)
	if err != nil {
		cError.Printf("Failed to save bookmark: %v\n", err)
		return
	}

	// Save article image to local disk
	if imageURL != "" {
		imgPath := fp.Join(DataDir, "thumb", fmt.Sprintf("%d", book.ID))

		err = downloadFile(imageURL, imgPath, time.Minute)
		if err != nil {
			cError.Printf("Failed to download image: %v\n", err)
		}
	}

	// Print added bookmark
	fmt.Println()
	printBookmarks(book)
}
