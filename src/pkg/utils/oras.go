// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions.
package utils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	zarfconfig "github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/google/go-containerregistry/pkg/name"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/errcode"
)

const (
	ZarfLayerMediaTypeTarZstd = "application/vnd.zarf.layer.v1.tar+zstd"
	ZarfLayerMediaTypeTarGzip = "application/vnd.zarf.layer.v1.tar+gzip"
	ZarfLayerMediaTypeYaml    = "application/vnd.zarf.layer.v1.yaml"
	ZarfLayerMediaTypeJson    = "application/vnd.zarf.layer.v1.json"
	ZarfLayerMediaTypeTxt     = "application/vnd.zarf.layer.v1.txt"
	ZarfLayerMediaTypeUnknown = "application/vnd.zarf.layer.v1.unknown"
)

// ParseZarfLayerMediaType returns the Zarf layer media type for the given filename.
func ParseZarfLayerMediaType(filename string) string {
	// since we are controlling the filenames, we can just use the extension
	switch filepath.Ext(filename) {
	case ".zst":
		return ZarfLayerMediaTypeTarZstd
	case ".gz":
		return ZarfLayerMediaTypeTarGzip
	case ".yaml":
		return ZarfLayerMediaTypeYaml
	case ".json":
		return ZarfLayerMediaTypeJson
	case ".txt":
		return ZarfLayerMediaTypeTxt
	default:
		return ZarfLayerMediaTypeUnknown
	}
}

// OrasCtxWithScopes returns a context with the given scopes.
//
// This is needed for pushing to Docker Hub.
func OrasCtxWithScopes(fullname string) context.Context {
	// For pushing to Docker Hub, we need to set the scope to the repository with pull+push actions, otherwise a 401 is returned
	scopes := []string{
		fmt.Sprintf("repository:%s:pull,push", fullname),
	}
	return auth.WithScopes(context.Background(), scopes...)
}

// OrasAuthClient returns an auth client for the given reference.
//
// The credentials are pulled using Docker's default credential store.
func OrasAuthClient(ref name.Reference) (*auth.Client, error) {
	// load default Docker config file
	cfg, err := config.Load(config.Dir())
	if err != nil {
		return &auth.Client{}, err
	}
	if !cfg.ContainsAuth() {
		return &auth.Client{}, errors.New("no docker config file found, run 'docker login'")
	}

	configs := []*configfile.ConfigFile{cfg}

	var key = ref.Context().RegistryStr()
	if key == "registry-1.docker.io" || key == "docker.io" {
		// Docker stores its credentials under the following key, otherwise credentials use the registry URL
		key = "https://index.docker.io/v1/"
	}

	authConf, err := configs[0].GetCredentialsStore(key).Get(key)
	if err != nil {
		return &auth.Client{}, fmt.Errorf("unable to get credentials for %s: %w", key, err)
	}

	cred := auth.Credential{
		Username:     authConf.Username,
		Password:     authConf.Password,
		AccessToken:  authConf.RegistryToken,
		RefreshToken: authConf.IdentityToken,
	}

	return &auth.Client{
		Credential: auth.StaticCredential(ref.Context().RegistryStr(), cred),
		Cache:      auth.NewCache(),
		// Gitlab auth fails if ForceAttemptOAuth2 is set to true
		// ForceAttemptOAuth2: true,
	}, nil
}

// isManifestUnsupported returns true if the error is an unsupported artifact manifest error.
//
// This function was copied verbatim from https://github.com/oras-project/oras/blob/main/cmd/oras/push.go
func IsManifestUnsupported(err error) bool {
	var errResp *errcode.ErrorResponse
	if !errors.As(err, &errResp) || errResp.StatusCode != http.StatusBadRequest {
		return false
	}

	var errCode errcode.Error
	if !errors.As(errResp, &errCode) {
		return false
	}

	// As of November 2022, ECR is known to return UNSUPPORTED error when
	// putting an OCI artifact manifest.
	switch errCode.Code {
	case errcode.ErrorCodeManifestInvalid, errcode.ErrorCodeUnsupported:
		return true
	}
	return false
}

// PullOCIZarfPackageOpts are the options for pulling a Zarf package from a registry.
type PullOCIZarfPackageOpts struct {
	remote.Repository
	Ref     name.Reference
	Outdir  string
	Spinner *message.Spinner
	// 👇 used for pulling a single component out of a Zarf package stored in a registry
	ComponentDesired string
}

// PullOCIZarfPackage downloads a Zarf package w/ the given reference to the specified output directory.
func PullOCIZarfPackage(pullOpts PullOCIZarfPackageOpts) error {
	spinner := pullOpts.Spinner
	ref := pullOpts.Ref
	_ = os.Mkdir(pullOpts.Outdir, 0755)
	ctx := OrasCtxWithScopes(ref.Context().RepositoryStr())
	repo, err := remote.NewRepository(fmt.Sprintf("%s/%s", ref.Context().RegistryStr(), ref.Context().RepositoryStr()))
	if err != nil {
		return err
	}
	repo.PlainHTTP = pullOpts.PlainHTTP

	authClient, err := OrasAuthClient(ref)
	if err != nil {
		return err
	}
	repo.Client = authClient

	// get the manifest descriptor
	// ref.Identifier() can be a tag or a digest
	descriptor, err := repo.Resolve(ctx, ref.Identifier())
	if err != nil {
		return err
	}

	// get the manifest itself
	pulled, err := content.FetchAll(ctx, repo, descriptor)
	if err != nil {
		return err
	}
	manifest := ocispec.Manifest{}
	artifact := ocispec.Artifact{}
	var layers []ocispec.Descriptor
	// if the manifest is an artifact, unmarshal it as an artifact
	// otherwise, unmarshal it as a manifest
	if descriptor.MediaType == ocispec.MediaTypeArtifactManifest {
		if err = json.Unmarshal(pulled, &artifact); err != nil {
			return err
		}
		layers = artifact.Blobs
	} else {
		if err = json.Unmarshal(pulled, &manifest); err != nil {
			return err
		}
		layers = manifest.Layers
	}

	// if a component is specified, only pull that component and the zarf.yaml
	if pullOpts.ComponentDesired != "" {
		componentAndZarfYaml := []ocispec.Descriptor{}
		for _, layer := range layers {
			if filepath.Base(layer.Annotations[ocispec.AnnotationTitle]) == pullOpts.ComponentDesired+".tar.zst" || layer.Annotations[ocispec.AnnotationTitle] == zarfconfig.ZarfYAML {
				componentAndZarfYaml = append(componentAndZarfYaml, layer)
			}
		}
		layers = componentAndZarfYaml
	}

	// get the layers
	for _, layer := range layers {
		path := filepath.Join(pullOpts.Outdir, layer.Annotations[ocispec.AnnotationTitle])
		// if the file exists and the size matches, skip it
		info, err := os.Stat(path)
		if err == nil && info.Size() == layer.Size {
			message.SuccessF("%s %s", layer.Digest.Hex()[:12], layer.Annotations[ocispec.AnnotationTitle])
			continue
		}
		spinner.Updatef("%s %s", layer.Digest.Hex()[:12], layer.Annotations[ocispec.AnnotationTitle])

		layerContent, err := content.FetchAll(ctx, repo, layer)
		if err != nil {
			return err
		}

		parent := filepath.Dir(path)
		if parent != "." {
			_ = os.MkdirAll(parent, 0755)
		}

		file, err := os.Create(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = file.Write(layerContent)
		if err != nil {
			return err
		}
		message.SuccessF("%s %s", layer.Digest.Hex()[:12], layer.Annotations[ocispec.AnnotationTitle])
	}

	return nil
}
