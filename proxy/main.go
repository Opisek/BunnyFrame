package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/makeworld-the-better-one/dither/v2"
	"golang.org/x/image/bmp"
	"golang.org/x/image/webp"
)

type Bunny struct {
	Number      int
	Description string
	Image       image.Image
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
var rgbRegex = regexp.MustCompile(`\s*#?([0-9a-fA-F]{2})([0-9a-fA-F]{2})([0-9a-fA-F]{2})\s*`)

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

		if !isBunny {
			continue
		}

		imageResp, err := http.Get(post.Post.Embed.Images[0].Fullsize)
		if err != nil {
			return nil, err
		}

		img, err := webp.Decode(imageResp.Body)
		if err != nil {
			return nil, err
		}

		return &Bunny{
			Number:      bunnyNumber,
			Description: post.Post.Record.Text,
			Image:       img,
		}, nil
	}

	fmt.Println(len(feed.Posts))
	fmt.Println(feed.Posts[0].Post.Record.Text)

	return nil, errors.New("no daily bunny found")
}

func scaleImage(img image.Image, width int, height int) image.Image {
	scaledImg := imaging.Fit(img, width, height, imaging.CatmullRom)
	background := imaging.New(width, height, color.White)
	x := (width - scaledImg.Bounds().Dx()) / 2
	y := (height - scaledImg.Bounds().Dy()) / 2
	return imaging.Paste(background, scaledImg, image.Pt(x, y))
}

func parseColor(colorString string) (color.Color, error) {
	matches := rgbRegex.FindStringSubmatch(colorString)
	if matches == nil {
		return color.RGBA{}, fmt.Errorf("invalid color code: %s", colorString)
	}

	// We can ignore the errors in the following lines because the regex makes sure they don't occurr
	r, _ := strconv.ParseUint(matches[1], 16, 8)
	g, _ := strconv.ParseUint(matches[2], 16, 8)
	b, _ := strconv.ParseUint(matches[3], 16, 8)

	return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}, nil
}

func ditherImage(img image.Image, colors []color.Color) image.Image {
	d := dither.NewDitherer(colors)
	d.Matrix = dither.FloydSteinberg
	return d.Dither(img)
}

func main() {
	var latestBunny *Bunny
	var latestBunnyTimestamp time.Time

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !(r.Method == http.MethodGet || r.Method == http.MethodHead) {
			http.Error(w, "Unsupported operation", http.StatusBadRequest)
		}

		// Get the latest bunny
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
			return
		}

		img := latestBunny.Image

		// Scale the bunny
		rawWidth := r.URL.Query().Get("width")
		rawHeight := r.URL.Query().Get("height")
		if len(rawWidth) != 0 || len(rawHeight) != 0 {
			width, err := strconv.Atoi(rawWidth)
			if err != nil || width <= 0 {
				http.Error(w, "Could not scale bunny to the requested size", http.StatusBadRequest)
				return
			}

			height, err := strconv.Atoi(rawHeight)
			if err != nil || height <= 0 {
				http.Error(w, "Could not scale bunny to the requested size", http.StatusBadRequest)
				return
			}

			img = scaleImage(latestBunny.Image, 800, 400)
		}

		// Dither the bunny
		rawColors := r.URL.Query().Get("colors")
		if len(rawColors) != 0 {
			colorStrings := strings.Split(rawColors, ",")

			palette := make([]color.Color, len(colorStrings))

			for i, colorString := range colorStrings {
				var err error
				palette[i], err = parseColor(colorString)
				if err != nil {
					http.Error(w, "Could not dither bunny to requested colors", http.StatusBadRequest)
					return
				}
			}

			img = ditherImage(img, palette)
		}

		// Convert bunny to bmp
		buf := bytes.Buffer{}
		err := bmp.Encode(&buf, img)
		if err != nil {
			http.Error(w, "Could not convert bunny to bitmap", http.StatusInternalServerError)
			return
		}
		finishedBunny := buf.Bytes()

		// Send the bunny
		w.Header().Add("Content-Type", "image/bmp")
		w.Header().Add("Content-Length", strconv.Itoa(len(finishedBunny)))
		w.Header().Add("Content-Disposition", fmt.Sprintf("inline; filename=bunny%d.webp", latestBunny.Number))
		w.Header().Add("ETag", fmt.Sprintf("%d;%s;%s:%s", latestBunny.Number, rawWidth, rawHeight, rawColors))

		if r.Method == http.MethodGet {
			w.Write(finishedBunny)
		}
	})

	http.ListenAndServe(":8080", nil)
}
