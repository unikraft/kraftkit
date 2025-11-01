package handler

import (
	"context"
	"fmt"
	"io"

	"github.com/containerd/containerd/content"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"kraftkit.sh/config"
)

type ContainerdHandler struct{}

func NewContainerdHandler(ctx context.Context, address, namespace string, auths map[string]config.AuthConfig, opts ...any) (context.Context, *ContainerdHandler, error) {
	return ctx, nil, fmt.Errorf("containerd is not supported on openbsd")
}

// DigestInfo implements DigestResolver.
func (handle *ContainerdHandler) DigestInfo(ctx context.Context, dgst digest.Digest) (*content.Info, error) {
	return nil, fmt.Errorf("not implemented: oci.handler.ContainerdHandler.DigestInfo")
}

// PullDigest implements DigestPuller.
func (handle *ContainerdHandler) PullDigest(ctx context.Context, mediaType, fullref string, dgst digest.Digest, plat *ocispec.Platform, onProgress func(float64)) error {
	return fmt.Errorf("not implemented: oci.handler.ContainerdHandler.PullDigest")
}

// ReadDigest implements DigestReader.
func (handle *ContainerdHandler) ReadDigest(ctx context.Context, dgst digest.Digest) (io.ReadCloser, error) {
	return nil, fmt.Errorf("not implemented: oci.handler.ContainerdHandler.ReadDigest")
}

// ListDigest implements DigestResolver.
func (handle *ContainerdHandler) ListDigests(ctx context.Context) ([]digest.Digest, error) {
	return nil, fmt.Errorf("not implemented: oci.handler.ContainerdHandler.ListDigests")
}

// PushDescriptor implements DescriptorPusher.
func (handle *ContainerdHandler) PushDescriptor(ctx context.Context, ref string, target *ocispec.Descriptor, onProgress func(float64)) error {
	return fmt.Errorf("not implemented: oci.handler.ContainerdHandler.PushDescriptor")
}

// DeleteDigest implements DigestDeleter.
func (handle *ContainerdHandler) DeleteDigest(ctx context.Context, dgst digest.Digest) error {
	return fmt.Errorf("not implemented: oci.handler.ContainerdHandler.DeleteDigest")
}

// ListIndexes implements IndexLister.
func (handle *ContainerdHandler) ListIndexes(ctx context.Context) (map[string]*ocispec.Index, error) {
	return nil, fmt.Errorf("not implemented: oci.handler.ContainerdHandler.ListIndexes")
}

func (handle *ContainerdHandler) DeleteIndex(ctx context.Context, fullref string, deps bool) error {
	return fmt.Errorf("not implemented: oci.handler.ContainerdHandler.DeleteIndex")
}

// ResolveIndex implements IndexResolver.
func (handle *ContainerdHandler) ResolveIndex(ctx context.Context, fullref string) (*ocispec.Index, digest.Digest, error) {
	return nil, "", fmt.Errorf("not implemented: oci.handler.ContainerdHandler.ResolveIndex")
}

// SaveDescriptor implements DescriptorSaver.
func (handle *ContainerdHandler) SaveDescriptor(ctx context.Context, fullref string, desc ocispec.Descriptor, reader io.Reader, onProgress func(float64)) (err error) {
	return fmt.Errorf("not implemented: oci.handler.ContainerdHandler.SavedDescriptor")
}

// ResolveManifest implements ManifestResolver.
func (handle *ContainerdHandler) ResolveManifest(ctx context.Context, _ string, digest digest.Digest) (*ocispec.Manifest, *ocispec.Image, error) {
	return nil, nil, fmt.Errorf("not implemented: oci.handler.ContainerdHandler.ResolveManifest")
}

// ListManifests implements DigestResolver.
func (handle *ContainerdHandler) ListManifests(ctx context.Context) (manifests map[string]*ocispec.Manifest, err error) {
	return nil, fmt.Errorf("not implemented: oci.handler.ContainerdHandler.ListManifests")
}

func (handle *ContainerdHandler) DeleteManifest(ctx context.Context, fullref string, dgst digest.Digest) error {
	return fmt.Errorf("not implemented: oci.handler.ContainerdHandler.DeleteManifest")
}

// UnpackImage implements ImageUnpacker.
func (handle *ContainerdHandler) UnpackImage(ctx context.Context, ref string, dgst digest.Digest, dest string) (*ocispec.Image, error) {
	return nil, fmt.Errorf("not implemented: oci.handler.ContainerdHandler.UnpackImage")
}

// FinalizeImage implements ImageFinalizer.
func (handle *ContainerdHandler) FinalizeImage(ctx context.Context, image ocispec.Image) error {
	return fmt.Errorf("not implemented: oci.handler.ContainerdHandler.FinalizeImage")
}
