package book

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ztrue/tracerr"
)

var idRegex = regexp.MustCompile(`^(\w+\/\w+)\/?`)
var startTrimPattern = regexp.MustCompile(`^[^\{]+`)
var endTrimPattern = regexp.MustCompile(`[^}]+$`)

type Book struct {
	Url   string
	Id    string
	Title string
	Pages []Page
}

type Page struct {
	Number       int
	ThumbnailUrl string
	ImageUrls    []string
}

type PageImage struct {
	PageNumber   int
	ImageNumber  int
	OverallOrder int
	Url          string
}

type DownloadedImage struct {
	PageNumber   int
	ImageNumber  int
	OverallOrder int
	Url          string
	FullPath     string
}

type htmlConfig struct {
	Pages []page `json:"fliphtml5_pages"`
	Meta  meta   `json:"meta"`
}

type meta struct {
	Title string `json:"title"`
}

type page struct {
	Images   []string `json:"n"`
	ThumbUrl string   `json:"t"`
}

func ParseId(idOrUrl string) (string, error) {
	trimmed := strings.TrimPrefix(idOrUrl, "https://online.fliphtml5.com/")
	trimmed = strings.TrimSuffix(trimmed, "http://online.fliphtml5.com/")

	matches := idRegex.FindStringSubmatch(trimmed)
	if matches == nil || len(matches) < 2 {
		return "", fmt.Errorf("invalid ID or URL: %s", trimmed)
	}

	return matches[1], nil
}

func downloadHtmlConfig(id string) (*htmlConfig, error) {
	response, err := http.Get(fmt.Sprintf("https://online.fliphtml5.com/%s/javascript/config.js", id))
	if err != nil {
		return nil, tracerr.Wrap(err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download book information: %s", response.Status)
	}

	jsConfigBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}

	jsConfig := string(jsConfigBytes)
	jsonConfig := startTrimPattern.ReplaceAllLiteralString(jsConfig, "")
	jsonConfig = endTrimPattern.ReplaceAllLiteralString(jsonConfig, "")

	var config htmlConfig
	err = json.Unmarshal([]byte(jsonConfig), &config)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}

	return &config, nil
}

func Get(idOrUrl string) (*Book, error) {
	id, err := ParseId(idOrUrl)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}

	htmlConfig, err := downloadHtmlConfig(id)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}

	pages := make([]Page, 0)
	for i, pageInfo := range htmlConfig.Pages {
		images := make([]string, 0)
		for _, imageUrl := range pageInfo.Images {
			images = append(images, fmt.Sprintf("https://online.fliphtml5.com/%s/files/large/%s", id, imageUrl))
		}

		pages = append(pages, Page{
			Number:       i + 1,
			ThumbnailUrl: pageInfo.ThumbUrl,
			ImageUrls:    images,
		})
	}

	return &Book{
		Url:   fmt.Sprintf("https://online.fliphtml5.com/%s/", id),
		Id:    id,
		Title: html.UnescapeString(htmlConfig.Meta.Title),
		Pages: pages,
	}, nil
}

func (b *Book) FindAllImages() []PageImage {
	images := make([]PageImage, 0)

	order := 1
	for i, page := range b.Pages {
		for j, imageUrl := range page.ImageUrls {
			images = append(images, PageImage{
				PageNumber:   i + 1,
				ImageNumber:  j + 1,
				OverallOrder: order,
				Url:          imageUrl,
			})

			order++
		}
	}

	return images
}

func (i *PageImage) Download(ctx context.Context, outputFolder string) (*DownloadedImage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, i.Url, nil)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download image: %s", res.Status)
	}

	fullPath := filepath.Join(outputFolder, fmt.Sprintf("%d-%d.jpg", i.PageNumber, i.ImageNumber))
	file, err := os.Create(fullPath)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}

	defer file.Close()

	_, err = io.Copy(file, res.Body)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}

	return &DownloadedImage{
		PageNumber:   i.PageNumber,
		ImageNumber:  i.ImageNumber,
		OverallOrder: i.OverallOrder,
		Url:          i.Url,
		FullPath:     fullPath,
	}, nil
}
