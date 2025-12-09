package main

import (
    "flag"
    "fmt"
    "log"
    "os"
    "strings"

    "github.com/OhMyDitzzy/go-payload-dumper/internal/dumper"
)

func main() {
    payloadPath := flag.String("payload", "", "payload file path or URL (can be a zip file)")
    outDir := flag.String("out", "output", "output directory")
    diff := flag.Bool("diff", false, "extract differential OTA")
    oldDir := flag.String("old", "old", "directory with original images for differential OTA")
    images := flag.String("images", "", "comma-separated list of images to extract")
    flag.Parse()

    if *payloadPath == "" {
        fmt.Println("Usage: go-payload-dumper -payload <path> [options]")
        flag.PrintDefaults()
        os.Exit(1)
    }

    if err := os.MkdirAll(*outDir, 0755); err != nil {
        log.Fatalf("Failed to create output directory: %v", err)
    }

    d, err := dumper.New(*payloadPath, *outDir, *oldDir, *diff)
    if err != nil {
        log.Fatalf("Failed to initialize dumper: %v", err)
    }
    defer d.Close()

    var imageList []string
    if *images != "" {
        imageList = strings.Split(*images, ",")
    }

    if err := d.Extract(imageList); err != nil {
        log.Fatalf("Failed to extract payload: %v", err)
    }

    fmt.Println("Extraction completed successfully!")
}