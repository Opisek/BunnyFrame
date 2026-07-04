package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"
)

type Bunny struct {
	Number      int
	Description string
	Image       []byte
}

type Post struct {
	Record struct {
		Text string `json:"text"`
	} `json:"record"`
	Embed struct {
		Images []struct {
			Fullsize string `json:"fullsize"`
		} `json:"images"`
	} `json:"embed"`
}

type Feed struct {
	Posts []struct {
		Post Post `json:"post"`
	} `json:"feed"`
}

var bunnyRegex = regexp.MustCompile(`bunny no\.(\d+)\s`)

func isBunnyPost(post Post) (bool, int) {
	if len(post.Embed.Images) == 0 {
		return false, 0
	}

	if submatches := bunnyRegex.FindStringSubmatch(post.Record.Text); len(submatches) == 2 {
		bunNumber, err := strconv.Atoi(submatches[1])
		if err != nil {
			return false, 0
		}

		return true, bunNumber
	}

	return false, 0
}

func getLatestBunny() (*Bunny, error) {
	resp, err := http.Get("https://public.api.bsky.app/xrpc/app.bsky.feed.getAuthorFeed?actor=did:plc:wf7nfy2us3h5gpa7zfettmzl&limit=10")
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	feed := Feed{}
	err = json.Unmarshal(body, &feed)

	for _, post := range feed.Posts {
		isBunny, bunnyNumber := isBunnyPost(post.Post)

		imageResp, err := http.Get(post.Post.Embed.Images[0].Fullsize)
		if err != nil {
			return nil, err
		}

		image, err := io.ReadAll(imageResp.Body)
		if err != nil {
			return nil, err
		}

		if isBunny {
			return &Bunny{
				Number:      bunnyNumber,
				Description: post.Post.Record.Text,
				Image:       image,
			}, nil
		}
	}

	fmt.Println(len(feed.Posts))
	fmt.Println(feed.Posts[0].Post.Record.Text)

	return nil, errors.New("no daily bunny found")
}

func main() {
	var latestBunny *Bunny
	var latestBunnyTimestamp time.Time

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !(r.Method == http.MethodGet || r.Method == http.MethodHead) {
			http.Error(w, "Unsupported operation", http.StatusBadRequest)
		}

		currentTimestamp := time.Now()

		if latestBunny != nil || currentTimestamp.Sub(latestBunnyTimestamp) > time.Hour {
			var err error
			latestBunny, err = getLatestBunny()
			if err != nil {
				fmt.Fprintln(os.Stdout, err)
				latestBunnyTimestamp = currentTimestamp
			} else {
			}
		}

		if latestBunny == nil {
			http.Error(w, "Could not get latest bunny", http.StatusInternalServerError)
		} else {
			w.Header().Add("Content-Type", "image/webp")
			w.Header().Add("Content-Length", strconv.Itoa(len(latestBunny.Image)))
			w.Header().Add("Content-Disposition", fmt.Sprintf("attachment; filename=bunny%d.webp", latestBunny.Number))

			if r.Method == http.MethodGet {
				w.Write(latestBunny.Image)
			}
		}
	})

	http.ListenAndServe(":8080", nil)
}
