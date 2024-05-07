package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"

	arg "github.com/alexflint/go-arg"
	pdfcpu_api "github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/schollz/progressbar/v3"
	book "github.com/ygunayer/fh5dl/internal/book"
	"github.com/ztrue/tracerr"
	"golang.org/x/sync/errgroup"
)

type Args struct {
	Url               string `arg:"positional, required" help:"ID or URL of the PDF to download"`
	Concurrency       int    `arg:"-c" help:"(Optional) Number of concurrent downloads. Defaults to (number of CPUs available - 1)"`
	OutputFolder      string `arg:"-o" help:"(Optional) Output folder for the PDF. Defaults to the current working directory" default:"."`
	ImageOutputFolder string `arg:"--image-out" help:"(Optional) Output folder for downloaded images. Defaults to a temporary directory" default:""`
	Force             bool   `arg:"-f" help:"(Optional) Overwrite existing PDF file if it exists"`
}

func downloadImages(ctx context.Context, args *Args, images []book.PageImage) ([]book.DownloadedImage, error) {
	imageOutputRoot := ""
	if args.ImageOutputFolder != "" {
		realdir, err := filepath.Abs(args.ImageOutputFolder)
		if err != nil {
			return nil, tracerr.Wrap(err)
		}

		if _, err := os.Stat(realdir); os.IsNotExist(err) {
			err = os.MkdirAll(realdir, os.ModePerm)
			if err != nil {
				return nil, tracerr.Wrap(err)
			}
		}

		imageOutputRoot = realdir
	} else {
		tmpdir, err := os.MkdirTemp("", "fh5dl-")
		if err != nil {
			return nil, tracerr.Wrap(err)
		}

		imageOutputRoot = tmpdir
	}

	bar := progressbar.Default(int64(len(images)), "Downloading images")

	//_asd := make(chan int, 0)
	downloadedImages := make([]book.DownloadedImage, 0)
	mutex := sync.Mutex{}

	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(args.Concurrency)

	for _, image := range images {
		go func(image book.PageImage) {
			eg.Go(func() error {
				result, err := image.Download(ctx, imageOutputRoot)
				if err != nil {
					return tracerr.Wrap(err)
				}

				mutex.Lock()
				downloadedImages = append(downloadedImages, *result)
				mutex.Unlock()

				if err := bar.Add(1); err != nil {
					return tracerr.Wrap(err)
				}

				return nil
			})
		}(image)
	}

	if err := eg.Wait(); err != nil {
		return nil, tracerr.Wrap(err)
	}

	sort.Slice(downloadedImages, func(i, j int) bool {
		return downloadedImages[i].OverallOrder < downloadedImages[j].OverallOrder
	})

	if err := bar.Close(); err != nil {
		return nil, tracerr.Wrap(err)
	}

	return downloadedImages, nil
}

func die(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}

func main() {
	args := Args{}
	arg.MustParse(&args)

	realOutputFolder, err := filepath.Abs(args.OutputFolder)
	if err != nil {
		panic(tracerr.Wrap(err))
	}

	if args.Concurrency == 0 {
		args.Concurrency = runtime.NumCPU() - 1
	}

	b, err := book.Get(args.Url)
	if err != nil {
		die(tracerr.Wrap(err))
	}

	images := b.FindAllImages()
	println(fmt.Sprintf("Found book \"%s\" with %d pages and %d images", b.Title, len(b.Pages), len(images)))

	fullOutputPath := filepath.Join(realOutputFolder, b.Title+".pdf")
	fileStat, err := os.Stat(fullOutputPath)
	if err != nil {
		if !os.IsNotExist(err) {
			die(err)
		}
	}

	if fileStat != nil {
		if fileStat.IsDir() {
			die(fmt.Errorf("output path \"%s\" is a directory", fullOutputPath))
		}

		if !args.Force {
			die(fmt.Errorf("output file \"%s\" already exists. Use -f to overwrite", fullOutputPath))
		}
	}

	if len(images) < 1 {
		die(fmt.Errorf("book \"%s\" has no images", b.Title))
	}

	downloadedImages, err := downloadImages(context.Background(), &args, images)
	if err != nil {
		die(tracerr.Wrap(err))
	}

	println("All images downloaded. Generating PDF")

	imageFiles := make([]string, 0)
	for _, downloadedImage := range downloadedImages {
		imageFiles = append(imageFiles, downloadedImage.FullPath)
	}

	importConfig := pdfcpu.DefaultImportConfig()
	modelConfig := model.NewDefaultConfiguration()

	err = pdfcpu_api.ImportImagesFile(imageFiles, fullOutputPath, importConfig, modelConfig)
	if err != nil {
		die(tracerr.Wrap(err))
	}

	println(fmt.Sprintf("PDF saved to \"%s\"", fullOutputPath))
}
