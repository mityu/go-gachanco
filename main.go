package main

import (
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-pdf/fpdf"
)

const (
	KindFile = iota
	KindDir
)

const (
	A4WidthMM  = float64(210)
	A4HeightMM = float64(297)
)

type BuildOption struct {
	ExcludeInvalidFiles bool
	OverwritePDF        bool
}

type Resource struct {
	Outfile     string
	Infiles     []string
	InfilesKind int
	Option      BuildOption
}

func getUsage() string {
	return strings.Join([]string{
		"Usage: gachanco files|dirs",
		"        [<flags>] [-o <output file>] <target1> [,<target2>, [...]]",
		"",
		"    file(s)    Make PDF from specified files.",
		"    dir(s)     Make PDF from images in specified directories.",
		"",
		"    <flags>",
		"    --exclude-invalid-files",
		"        Exclude non-valid image files in targets instead of",
		"        giving error.",
		"    --overwrite-pdf    Overwrite PDF file even if it exists.",
	}, "\n")
}

func hasInStrings(l []string, s string) bool {
	for _, e := range l {
		if e == s {
			return true
		}
	}
	return false
}

func generateOutputPDFName(base string) string {
	// Convert "path/to/dir/" -> "path/to/dir"
	if _, name := filepath.Split(base); name == "" {
		base = filepath.Dir(base)
	}
	destPDFFile := base + ".pdf"
	if _, err := os.Stat(destPDFFile); err == nil {
		// destPDFFile is already exists. Add suffix.
		tmp := base + "-" + time.Now().Format("20060102150405")
		if _, err := os.Stat(tmp + ".pdf"); err == nil {
			// It seems that we need much more suffix.
			for i := 1; ; i++ {
				s := fmt.Sprintf("-%03d", i)
				if _, err := os.Stat(tmp + s + ".pdf"); err != nil {
					tmp += s
					break
				}
			}
		}
		destPDFFile = tmp + ".pdf"
	}
	return destPDFFile
}

func parseArgs(args []string) (Resource, error) {
	arglen := len(args)
	if arglen == 0 || hasInStrings([]string{"--help", "-h"}, args[0]) {
		fmt.Println(getUsage())
		return Resource{}, nil
	} else if arglen == 1 ||
		!hasInStrings([]string{"files", "file", "dirs", "dir"}, args[0]) {
		errmsg := "Error: Invalid argument\n" + getUsage()
		return Resource{}, errors.New(errmsg)
	}

	resource := Resource{}

	for i := 1; i < arglen; i++ {
		if args[i] == "-o" {
			i++
			if i == arglen {
				return Resource{}, errors.New(
					"Invalid argument: Nothing follows after \"-o\"")
			}
			resource.Outfile = args[i]
		} else if args[i] == "--exclude-invalid-files" {
			resource.Option.ExcludeInvalidFiles = true
		} else if args[i] == "--overwrite-pdf" {
			resource.Option.OverwritePDF = true
		} else {
			resource.Infiles = append(resource.Infiles, args[i])
		}
	}
	if strings.HasPrefix(args[0], "file") {
		resource.InfilesKind = KindFile
	} else {
		resource.InfilesKind = KindDir
	}
	return resource, nil
}

func validateResource(resource *Resource) error {
	if len(resource.Infiles) == 0 {
		return errors.New("Invalid argument: No files or dirs is specified.")
	}

	if resource.Outfile == "" {
		resource.Outfile = generateOutputPDFName(resource.Infiles[0])
		fmt.Println(
			"No output file is specified. Auto generate output file:",
			resource.Outfile)
	} else if info, err := os.Stat(resource.Outfile); err == nil {
		if info.IsDir() {
			return errors.New(
				"Output file is a directory: " + resource.Outfile)
		} else if !resource.Option.OverwritePDF {
			return errors.New(
				"Output file already exists: " + resource.Outfile)
		}
	}

	if resource.InfilesKind == KindFile {
		errfiles := []string{}
		targetfiles := []string{}
		// TODO: add check for non-image files
		for _, fname := range resource.Infiles {
			if info, err := os.Stat(fname); err == nil && !info.IsDir() {
				targetfiles = append(targetfiles, fname)
			} else {
				errfiles = append(errfiles, fname)
			}
		}
		if !resource.Option.ExcludeInvalidFiles && len(errfiles) != 0 {
			return errors.New(
				"Invalid files:\n" + strings.Join(errfiles, "\n"))
		}
		resource.Infiles = targetfiles
	} else {
		errdirs := []string{}
		targetdirs := []string{}
		for _, dname := range resource.Infiles {
			if info, err := os.Stat(dname); err == nil && info.IsDir() {
				targetdirs = append(targetdirs, dname)
			} else {
				errdirs = append(errdirs, dname)
			}
		}
		if !resource.Option.ExcludeInvalidFiles && len(errdirs) != 0 {
			return errors.New(
				"Invalid dirs:\n" + strings.Join(errdirs, "\n"))
		}

		resource.Infiles = []string{}
		for _, dname := range targetdirs {
			entries, err := os.ReadDir(dname)
			if err != nil {
				return err
			}
			for _, e := range entries {
				// TODO: add check for non-image files
				if e.IsDir() {
					continue
				}
				resource.Infiles =
					append(resource.Infiles, filepath.Join(dname, e.Name()))
			}
		}
		resource.InfilesKind = KindFile
	}
	return nil
}

func BuildPDF(resource Resource) error {
	if err := validateResource(&resource); err != nil {
		return err
	}

	type ImgOpt struct {
		x float64
		y float64
		w float64
		h float64
		f string
		t string
	}

	filesCount := len(resource.Infiles)
	imgOpts := make([]ImgOpt, filesCount, filesCount)
	errChan := make(chan error, filesCount)
	var wg sync.WaitGroup
	for i, file := range resource.Infiles {
		wg.Add(1)
		go func(file string, dest *ImgOpt) {
			defer wg.Done()
			f, err := os.Open(file)
			if err != nil {
				errChan <- err
			}
			c, imgtype, err := image.DecodeConfig(f)
			if err != nil {
				if resource.Option.ExcludeInvalidFiles {
					fmt.Println(
						"Error happens while extracting metadata:", err, "\n",
						"    Excluded:", file)
				} else {
					errChan <- err
				}
				return
			}

			w, h := A4WidthMM, A4WidthMM
			scaleX := A4WidthMM / float64(c.Width)
			scaleY := A4HeightMM / float64(c.Height)

			if scaleX < scaleY {
				h = scaleX * float64(c.Height)
			} else if scaleY < scaleX {
				w = scaleY * float64(c.Width)
			}

			x := (A4WidthMM - w) / 2
			y := (A4HeightMM - h) / 2

			*dest = ImgOpt{
				x: x,
				y: y,
				w: w,
				h: h,
				t: imgtype,
				f: file,
			}
		}(file, &imgOpts[i])
	}
	wg.Wait()
	close(errChan)

	if !resource.Option.ExcludeInvalidFiles {
		for err := range errChan {
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return errors.New("Error happened while extracting metadata.")
			}
		}
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	for _, o := range imgOpts {
		if o.f == "" { // Skip errored file
			continue
		}
		pdf.AddPage()
		pdf.ImageOptions(o.f, o.x, o.y, o.w, o.h, false, fpdf.ImageOptions{
			ImageType:             o.t,
			ReadDpi:               true,
			AllowNegativePosition: false,
		}, 0, "")
	}
	if err := pdf.OutputFileAndClose(resource.Outfile); err != nil {
		return err
	}
	fmt.Println("Successfully generated:", resource.Outfile)
	return nil
}

func run() error {
	r, err := parseArgs(os.Args[1:])
	if err != nil {
		return err
	}
	return BuildPDF(r)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
