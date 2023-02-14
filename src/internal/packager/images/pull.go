// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/moby/moby/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pterm/pterm"
)

// PullAll pulls all of the images in the provided tag map.
func (i *ImgConfig) PullAll() error {
	var (
		longer      string
		imgCount    = len(i.ImgList)
		imageMap    = map[string]v1.Image{}
		tagToImage  = map[name.Tag]v1.Image{}
		digestToTag = make(map[string]string)
	)

	// Give some additional user feedback on larger image sets
	if imgCount > 15 {
		longer = "This step may take a couple of minutes to complete."
	} else if imgCount > 5 {
		longer = "This step may take several seconds to complete."
	}

	spinner := message.NewProgressSpinner("Loading metadata for %d images. %s", imgCount, longer)
	defer spinner.Stop()

	if message.GetLogLevel() >= message.DebugLevel {
		logs.Warn.SetOutput(spinner)
		logs.Progress.SetOutput(spinner)
	}

	for idx, src := range i.ImgList {
		spinner.Updatef("Fetching image metadata (%d of %d): %s", idx+1, imgCount, src)

		img, err := i.PullImage(src, spinner)
		if err != nil {
			return fmt.Errorf("failed to pull image %s: %w", src, err)
		}
		imageMap[src] = img
	}

	// Create the ImagePath directory
	err := os.Mkdir(i.ImagesPath, 0755)
	if err != nil {
		return fmt.Errorf("failed to create image path %s: %w", i.ImagesPath, err)
	}

	for src, img := range imageMap {
		tag, err := name.NewTag(src, name.WeakValidation)
		if err != nil {
			return fmt.Errorf("failed to create tag for image %s: %w", src, err)
		}
		tagToImage[tag] = img
	}
	spinner.Updatef("Preparing image sources and cache for image pulling")

	spinner.Success()
	title := fmt.Sprintf("Pulling %d images (%s of %d)", imgCount, "0", imgCount)
	progressBar := message.NewProgressBar(int64(imgCount), title)

	for tag, img := range tagToImage {
		// Update the progress bar
		title = fmt.Sprintf("Pulling %d images (%d of %d)", imgCount, len(digestToTag)+1, imgCount)
		progressBar.Update(int64(len(digestToTag)), title)

		// Save the image
		err := crane.SaveOCI(img, i.ImagesPath)
		if err != nil {
			fmt.Errorf("error when trying to save the img (%s): %w", tag.Name(), err)
		}

		// Get the image digest
		// NOTE: This digest/tag map is used to set an annotation on the image index.json later
		imgDigest, _ := img.Digest()
		digestToTag[imgDigest.String()] = tag.String()
	}

	if err := addImageNameAnnotation(i.ImagesPath, digestToTag); err != nil {
		return fmt.Errorf("unable to format OCI layout: %w", err)
	}

	return nil
}

// PullImage returns a v1.Image either by loading a local tarball or the wider internet.
func (i *ImgConfig) PullImage(src string, spinner *message.Spinner) (img v1.Image, err error) {
	// Load image tarballs from the local filesystem.
	if strings.HasSuffix(src, ".tar") || strings.HasSuffix(src, ".tar.gz") || strings.HasSuffix(src, ".tgz") {
		spinner.Updatef("Reading image tarball: %s", src)
		return crane.Load(src, config.GetCraneOptions(true)...)
	}

	// If crane is unable to pull the image, try to load it from the local docker daemon.
	if _, err := crane.Manifest(src, config.GetCraneOptions(i.Insecure)...); err != nil {
		message.Debugf("crane unable to pull image %s: %s", src, err)
		spinner.Updatef("Falling back to docker for %s. This may take some time.", src)

		// Parse the image reference to get the image name.
		reference, err := name.ParseReference(src)
		if err != nil {
			return nil, fmt.Errorf("failed to parse image reference %s: %w", src, err)
		}

		// Attempt to connect to the local docker daemon.
		ctx := context.TODO()
		cli, err := client.NewClientWithOpts(client.FromEnv)
		if err != nil {
			return nil, fmt.Errorf("docker not available: %w", err)
		}
		cli.NegotiateAPIVersion(ctx)

		// Inspect the image to get the size.
		rawImg, _, err := cli.ImageInspectWithRaw(ctx, src)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect image %s via docker: %w", src, err)
		}

		// Warn the user if the image is large.
		if rawImg.Size > 750*1000*1000 {
			warn := pterm.DefaultParagraph.WithMaxWidth(80).Sprintf("%s is %s and may take a very long time to load via docker. "+
				"See https://docs.zarf.dev/docs/faq for suggestions on how to improve large local image loading operations.",
				src, utils.ByteFormat(float64(rawImg.Size), 2))
			spinner.Warnf(warn)
		}

		// Use unbuffered opener to avoid OOM Kill issues https://github.com/defenseunicorns/zarf/issues/1214.
		// This will also take for ever to load large images.
		if img, err = daemon.Image(reference, daemon.WithUnbufferedOpener()); err != nil {
			return nil, fmt.Errorf("failed to load image %s from docker daemon: %w", src, err)
		}

		// The pull from the docker daemon was successful, return the image.
		return img, err
	}

	// Manifest was found, so use crane to pull the image.
	if img, err = crane.Pull(src, config.GetCraneOptions(i.Insecure)...); err != nil {
		return nil, fmt.Errorf("failed to pull image %s: %w", src, err)
	}

	spinner.Updatef("Preparing image %s", src)
	imageCachePath := filepath.Join(config.GetAbsCachePath(), config.ZarfImageCacheDir)
	img = cache.Image(img, cache.NewFilesystemCache(imageCachePath))

	return img, nil
}

// IndexJSON represents the index.json file in an OCI layout.
type IndexJSON struct {
	SchemaVersion int `json:"schemaVersion"`
	Manifests     []struct {
		MediaType   string            `json:"mediaType"`
		Size        int               `json:"size"`
		Digest      string            `json:"digest"`
		Annotations map[string]string `json:"annotations"`
	} `json:"manifests"`
}

// addImageNameAnnotation adds an annotation to the index.json file so that the deploying code can figure out what the image tag <-> digest shasum will be.
func addImageNameAnnotation(ociPath string, digestToTag map[string]string) error {
	// Add an 'org.opencontainers.image.base.name' annotation so we can figure out what the image tag/digest shasum will be during deploy time
	indexJSON, err := os.Open(path.Join(ociPath, "index.json"))
	if err != nil {
		message.Errorf(err, "Unable to open %s/index.json", ociPath)
		return err
	}

	var index IndexJSON
	byteValue, _ := io.ReadAll(indexJSON)
	indexJSON.Close()
	_ = json.Unmarshal(byteValue, &index)
	for idx, manifest := range index.Manifests {
		if manifest.Annotations == nil {
			manifest.Annotations = make(map[string]string)
		}
		manifest.Annotations[ocispec.AnnotationBaseImageName] = digestToTag[manifest.Digest]
		index.Manifests[idx] = manifest
	}

	indexPath := filepath.Join(ociPath, "index.json")

	// Remove any file that might already exist
	_ = os.Remove(indexPath)

	// Create the index.json file and save the data to it
	indexJSON, err = os.Create(indexPath)
	if err != nil {
		return err
	}

	indexJSONBytes, err := json.Marshal(index)
	if err != nil {
		return err
	}

	_, err = indexJSON.Write(indexJSONBytes)
	if err != nil {
		return err
	}

	return indexJSON.Close()
}
