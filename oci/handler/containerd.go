// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	clog "github.com/containerd/containerd/log"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/nerdctl/pkg/imgutil/dockerconfigresolver"
	regtypes "github.com/docker/docker/api/types/registry"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"

	"kraftkit.sh/archive"
	"kraftkit.sh/log"
)

const (
	ContainerdGCLayerPrefix   = "containerd.io/gc.ref.content.l"
	ContainerdGCContentPrefix = "containerd.io/gc.ref.content"
)

type ContainerdHandler struct {
	client    *containerd.Client
	namespace string
	auths     map[string]regtypes.AuthConfig
}

// NewContainerdHandler creates a Resolver-compatible interface given the
// containerd address and namespace.
func NewContainerdHandler(ctx context.Context, address, namespace string, auths map[string]regtypes.AuthConfig, opts ...containerd.ClientOpt) (context.Context, *ContainerdHandler, error) {
	client, err := containerd.New(address, opts...)
	if err != nil {
		return nil, nil, err
	}

	if namespace == "" {
		namespace = "default"
	}

	// Swap containerd's internal logger with our io.Writer which can be replaced
	// with WithLogger
	clog.G(ctx).Logger.Out = log.G(ctx).Out
	clog.G(ctx).Logger.Formatter = log.G(ctx).Formatter
	clog.G(ctx).Logger.Level = log.G(ctx).Level

	return ctx, &ContainerdHandler{
		client:    client,
		namespace: namespace,
		auths:     auths,
	}, nil
}

// NewContainerdWithClient create a containerd Resolver-compatible with an
// existing containerd client connection.
func NewContainerdWithClient(ctx context.Context, client *containerd.Client) (context.Context, *ContainerdHandler, error) {
	if client == nil {
		return nil, nil, fmt.Errorf("no containerd client provided")
	}

	return ctx, &ContainerdHandler{client: client}, nil
}

// lease creates a lease which can be closed to enable asynchronous
// communication with containerd
func (handle *ContainerdHandler) lease(ctx context.Context) (context.Context, func(context.Context) error, error) {
	ctx, done, err := handle.client.WithLease(namespaces.WithNamespace(ctx, handle.namespace))
	if err != nil {
		return nil, nil, err
	}

	// Swap containerd's internal logger with KraftKit's
	ctx = clog.WithLogger(ctx, log.G(ctx).WithContext(ctx))

	return ctx, done, nil
}

// DigestExists implements DigestResolver.
func (handle *ContainerdHandler) DigestExists(ctx context.Context, dgst digest.Digest) (exists bool, err error) {
	ctx, done, err := handle.lease(ctx)
	if err != nil {
		return false, err
	}

	defer func() {
		err = combineErrors(err, done(ctx))
	}()

	if _, err := handle.client.ContentStore().Info(ctx, dgst); err != nil {
		return false, err
	}

	return true, nil
}

// ListIndexes implements DigestResolver.
func (handle *ContainerdHandler) ListIndexes(ctx context.Context) (indx []ocispec.Index, err error) {
	ctx, done, err := handle.lease(ctx)
	if err != nil {
		return nil, err
	}

	defer func() {
		err = combineErrors(err, done(ctx))
	}()

	all, err := handle.client.ImageService().List(
		namespaces.WithNamespace(ctx, handle.namespace),
	)
	if err != nil {
		return nil, err
	}

	for _, image := range all {
		found, err := handle.client.GetImage(
			namespaces.WithNamespace(ctx, handle.namespace),
			image.Name,
		)
		if err != nil {
			return nil, err
		}

		if found.Target().MediaType != ocispec.MediaTypeImageIndex {
			continue
		}

		manifest, err := images.Manifest(
			namespaces.WithNamespace(ctx, handle.namespace),
			handle.client.ContentStore(), found.Target(), nil,
		)
		if err != nil {
			return nil, err
		}
		manifests, err := images.Children(
			namespaces.WithNamespace(ctx, handle.namespace),
			handle.client.ContentStore(),
			found.Target(),
		)
		if err != nil {
			return nil, err
		}

		index := ocispec.Index{
			MediaType:    manifest.MediaType,
			ArtifactType: manifest.ArtifactType,
			Manifests:    manifests,
			Subject:      manifest.Subject,
			Annotations:  manifest.Annotations,
		}

		indx = append(indx, index)
	}

	return indx, nil
}

// ListManifests implements DigestResolver.
func (handle *ContainerdHandler) ListManifests(ctx context.Context) (manifests []ocispec.Manifest, err error) {
	ctx, done, err := handle.lease(ctx)
	if err != nil {
		return nil, err
	}

	defer func() {
		err = combineErrors(err, done(ctx))
	}()

	all, err := handle.client.ImageService().List(ctx)
	if err != nil {
		return nil, err
	}

	for _, image := range all {
		found, err := handle.client.GetImage(ctx, image.Name)
		if err != nil {
			return nil, err
		}

		manifest, err := images.Manifest(ctx, handle.client.ContentStore(), found.Target(), nil)
		if err != nil {
			continue
		}

		manifests = append(manifests, manifest)
	}

	return manifests, nil
}

// TODO(craciunoiuc): Saving only saves the index and not the manifests themselves
// SaveDigest implements DigestSaver.
func (handle *ContainerdHandler) SaveDigest(ctx context.Context, ref string, desc ocispec.Descriptor, reader io.Reader, onProgress func(float64)) (err error) {
	log.G(ctx).Errorf("oci: Packaging not supported for containerd with index manifests. Saving will fail.")

	ctx, done, err := handle.lease(ctx)
	if err != nil {
		return err
	}

	defer func() {
		err = combineErrors(err, done(ctx))
	}()

	writer, err := content.OpenWriter(
		ctx,
		handle.client.ContentStore(),
		content.WithDescriptor(desc),
		content.WithRef(desc.Digest.String()),
	)
	if err != nil {
		return err
	}

	log.G(ctx).WithFields(logrus.Fields{
		"mediaType": desc.MediaType,
		"digest":    desc.Digest.String(),
	}).Tracef("oci: copying")

	var tee io.Reader
	var cache bytes.Buffer
	if desc.MediaType == ocispec.MediaTypeImageManifest ||
		desc.MediaType == ocispec.MediaTypeImageIndex {
		tee = io.TeeReader(reader, &cache)
	} else {
		tee = reader
	}

	if err := content.Copy(ctx, writer, tee, desc.Size, desc.Digest); err != nil {
		return err
	}

	// Write the image and the various parentage tags
	is := handle.client.ImageService()

	switch desc.MediaType {
	// case ociimages.MediaTypeDockerSchema2Manifest,
	// 			ocispec.MediaTypeImageManifest,
	// 			ociimages.MediaTypeDockerSchema2ManifestList:
	case ocispec.MediaTypeImageIndex,
		ocispec.MediaTypeImageManifest:
		// ref, ok := desc.Annotations[ociimages.AnnotationImageName]
		// if !ok {
		// 	return fmt.Errorf("cannot push image layer without image annotation")
		// }

		log.G(ctx).WithFields(logrus.Fields{
			"ref": ref,
		}).Trace("oci: indexing")

		manifest := ocispec.Manifest{}
		if err := json.NewDecoder(&cache).Decode(&manifest); err != nil {
			return err
		}

		// Add garbage prevention tags
		labels := map[string]string{}
		for i, l := range manifest.Layers {
			labels[fmt.Sprintf("%s.%d", ContainerdGCLayerPrefix, i)] = l.Digest.String()
		}

		labels[fmt.Sprintf("%s.%d", ContainerdGCLayerPrefix, len(manifest.Layers))] = manifest.Config.Digest.String()

		var image images.Image
		existingImage, err := is.Get(namespaces.WithNamespace(ctx, handle.namespace), ref)

		if err != nil || existingImage.Target.Digest.String() == "" {
			log.G(ctx).Trace("oci: creating new image")
			image = images.Image{
				Name:      ref,
				Labels:    nil,
				Target:    desc,
				CreatedAt: time.Now(),
				UpdatedAt: time.Time{},
			}
			_, err = is.Create(namespaces.WithNamespace(ctx, handle.namespace), image)
		} else {
			log.G(ctx).Trace("oci: updating existing image")
			image = images.Image{
				Name:      ref,
				Labels:    nil,
				Target:    desc,
				UpdatedAt: time.Time{},
			}
			_, err = is.Update(namespaces.WithNamespace(ctx, handle.namespace), image)
		}
		if err != nil {
			return err
		}

		updatedFields := make([]string, 0)

		for k, v := range labels {
			log.G(ctx).WithFields(logrus.Fields{
				k:     v,
				"ref": image.Target.Digest,
			}).Trace("oci: labelling")

			updatedFields = append(updatedFields, fmt.Sprintf("labels.%s", k))
		}

		if _, err := handle.client.ContentStore().Update(ctx, content.Info{
			Digest: digest.Digest(desc.Digest),
			Labels: labels,
		}, updatedFields...); err != nil {
			return err
		}
	}

	return nil
}

// ResolveImage implements ImageResolver.
func (handle *ContainerdHandler) ResolveImage(ctx context.Context, fullref, platform string) (imgspec ocispec.Image, err error) {
	ctx, done, err := handle.lease(ctx)
	if err != nil {
		return ocispec.Image{}, err
	}

	defer func() {
		err = combineErrors(err, done(ctx))
	}()

	image, err := handle.client.GetImage(
		namespaces.WithNamespace(ctx, handle.namespace),
		fullref,
	)
	if err != nil {
		return ocispec.Image{}, err
	}

	if image.Target().MediaType == ocispec.MediaTypeImageIndex {
		manifests, err := images.Children(
			namespaces.WithNamespace(ctx, handle.namespace),
			handle.client.ContentStore(),
			image.Target(),
		)
		if err != nil {
			return ocispec.Image{}, err
		}

		// Split on ':'
		parsed := strings.SplitN(fullref, ":", 2)
		var base string
		if len(parsed) == 2 {
			base = parsed[0]
		} else if len(parsed) == 1 {
			base = fullref
		} else {
			return ocispec.Image{}, fmt.Errorf("invalid image reference: %s", fullref)
		}

		for _, manifest := range manifests {
			if manifest.MediaType == ocispec.MediaTypeImageManifest &&
				(fmt.Sprintf("%s/%s", manifest.Platform.OS, manifest.Platform.Architecture) == platform ||
					len(platform) == 0 && len(manifests) == 1) {

				image, err := handle.client.GetImage(
					namespaces.WithNamespace(ctx, handle.namespace),
					base+":"+manifest.Digest.String(),
				)
				if err != nil {
					return ocispec.Image{}, err
				}

				return image.Spec(ctx)
			}
		}

		return ocispec.Image{}, fmt.Errorf("no matching platform found")
	} else {
		return image.Spec(ctx)
	}
}

// FetchImage implements ImageFetcher.
func (handle *ContainerdHandler) FetchImage(ctx context.Context, ref, plat string, onProgress func(float64)) (err error) {
	ctx, done, err := handle.lease(ctx)
	if err != nil {
		return err
	}

	defer func() {
		err = combineErrors(err, done(ctx))
	}()

	progress := make(chan struct{})
	ongoing := newJobs(ref)

	go func() {
		handle.reportProgress(ctx, ongoing, handle.client.ContentStore(), onProgress)
		close(progress)
	}()

	resolver, err := dockerconfigresolver.New(
		ctx,
		strings.Split(ref, "/")[0],
		dockerconfigresolver.WithSkipVerifyCerts(true),
		dockerconfigresolver.WithAuthCreds(func(domain string) (string, string, error) {
			auth, ok := handle.auths[domain]
			if !ok {
				return "", "", nil
			}

			return auth.Username, auth.Password, nil
		}),
	)
	if err != nil {
		return err
	}

	ropts := []containerd.RemoteOpt{
		containerd.WithImageHandler(images.HandlerFunc(func(_ context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
			if desc.MediaType != images.MediaTypeDockerSchema1Manifest {
				ongoing.Add(desc)
			}

			return nil, nil
		})),
		containerd.WithResolver(resolver),
	}

	if plat != "" {
		ropts = append(ropts, containerd.WithPlatform(plat))
	}

	// Fetch the image
	_, err = handle.client.Fetch(ctx, ref, ropts...)
	if err != nil {
		return err
	}

	<-progress

	image, err := handle.client.GetImage(
		namespaces.WithNamespace(ctx, handle.namespace),
		ref,
	)
	if err != nil {
		return err
	}
	manifests, err := images.Children(
		namespaces.WithNamespace(ctx, handle.namespace),
		handle.client.ContentStore(),
		image.Target(),
	)
	if err != nil {
		return err
	}

	// Split on ':'
	parsed := strings.SplitN(ref, ":", 2)
	var base string
	if len(parsed) == 2 {
		base = parsed[0]
	} else if len(parsed) == 1 {
		base = ref
	} else {
		return fmt.Errorf("invalid image reference: %s", ref)
	}

	for _, manifest := range manifests {
		if manifest.MediaType == ocispec.MediaTypeImageManifest &&
			(fmt.Sprintf("%s/%s", manifest.Platform.OS, manifest.Platform.Architecture) == plat ||
				len(plat) == 0 && len(manifests) == 1) {

			// Fetch the image
			_, err = handle.client.Fetch(
				namespaces.WithNamespace(ctx, handle.namespace),
				base+":"+manifest.Digest.String(),
				ropts...,
			)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// statusInfo holds the status info for an upload or download
type statusInfo struct {
	Ref       string
	Status    string
	Offset    int64
	Total     int64
	StartedAt time.Time
	UpdatedAt time.Time
}

// reportProgress uses the internal containerd store and live status monitoring
// in order to determine the overall progress of downloading multiple layers for
// a singlely specified OCI image which reports progress by invoking the
// `onProgress` callback method.
func (handle *ContainerdHandler) reportProgress(ctx context.Context, ongoing *jobs, cs content.Store, onProgress func(progress float64)) {
	// TODO: This implementation is based on containerd's `ctr content fetch`
	// implementation which can be found at:
	//
	//   https://github.com/containerd/containerd/blob/8ec1fc7/cmd/ctr/commands/content/fetch.go#L219-L336
	//
	// However, this implementation does not work correctly.  It seems that the
	// `active.Offset` attribute is reporting too little or `active.Total` is too
	// high and so the progress monitor shows progress download of 1-5% only.  In
	// KraftKit's progress meter (ui.paraprogress) and once this download is
	// complete (and the correct signal/channels are updated) the meter will jump
	// to 100% (as is desired by the package).  However, ultimately, this does not
	// feedback to the user the right information.  This implementation should be
	// more carefully inspected to determine why Offset and Total are incorrect.

	var (
		ticker   = time.NewTicker(100 * time.Millisecond)
		start    = time.Now()
		statuses = map[string]statusInfo{}
		done     bool
	)
	defer ticker.Stop()

outer:
	for {
		select {
		case <-ticker.C:
			resolved := "resolved"
			if !ongoing.IsResolved() {
				resolved = "resolving"
			}

			statuses[ongoing.name] = statusInfo{
				Ref:    ongoing.name,
				Status: resolved,
			}

			keys := []string{ongoing.name}

			// now, update the items in jobs that are not in active
			for _, j := range ongoing.Jobs() {
				key := remotes.MakeRefKey(ctx, j)

				activeSeen := map[string]struct{}{}
				if !done {
					activeStatuses, err := cs.ListStatuses(ctx, fmt.Sprintf("ref==%s", key))
					if err != nil {
						log.G(ctx).WithError(err).Error("active check failed")
						continue
					}

					// update status of active entries!
					for _, active := range activeStatuses {
						statuses[active.Ref] = statusInfo{
							Ref:       active.Ref,
							Status:    "downloading",
							Offset:    active.Offset,
							Total:     active.Total,
							StartedAt: active.StartedAt,
							UpdatedAt: active.UpdatedAt,
						}
						activeSeen[active.Ref] = struct{}{}
					}
				}

				keys = append(keys, key)
				if _, ok := activeSeen[key]; ok {
					continue
				}

				status, ok := statuses[key]
				if !done && (!ok || status.Status == "downloading") {
					info, err := cs.Info(ctx, j.Digest)
					if err != nil {
						if !errdefs.IsNotFound(err) {
							log.G(ctx).WithError(err).Error("failed to get content info")
							continue outer
						} else {
							statuses[key] = statusInfo{
								Ref:    key,
								Status: "waiting",
							}
						}
					} else if info.CreatedAt.After(start) {
						statuses[key] = statusInfo{
							Ref:       key,
							Status:    "done",
							UpdatedAt: info.CreatedAt,
							Total:     0,
						}
					} else {
						statuses[key] = statusInfo{
							Ref:    key,
							Status: "exists",
							Total:  0,
						}
					}
				} else if done {
					if ok {
						if status.Status != "done" && status.Status != "exists" {
							status.Status = "done"
							statuses[key] = status
						}
					} else {
						statuses[key] = statusInfo{
							Ref:    key,
							Status: "done",
							Total:  0,
						}
					}
				}
			}

			var offset int64 = 0
			var total int64 = 0
			var exists int = 0
			for _, status := range statuses {
				if status.Status == "exists" ||
					status.Status == "resolved" ||
					status.Status == "done" {
					exists++
				}
				if status.Total > 0 {
					offset += status.Offset
					total += status.Total
				}
			}

			if onProgress != nil {
				onProgress(float64(offset) / float64(total))
			}

			if done || (len(statuses) == exists) {
				return
			}
		case <-ctx.Done():
			done = true // allow ui to update once more
		}
	}
}

// PushImage implements ImagePusher.
func (handle *ContainerdHandler) PushImage(ctx context.Context, ref string, target *ocispec.Descriptor) error {
	img, err := handle.client.ImageService().Get(
		namespaces.WithNamespace(ctx, handle.namespace),
		ref,
	)
	if err != nil {
		return err
	}
	manifests, err := images.Children(
		namespaces.WithNamespace(ctx, handle.namespace),
		handle.client.ContentStore(),
		img.Target,
	)
	if err != nil {
		return err
	}

	resolver, err := dockerconfigresolver.New(
		ctx,
		strings.Split(ref, "/")[0],
		dockerconfigresolver.WithSkipVerifyCerts(true),
		dockerconfigresolver.WithAuthCreds(func(domain string) (string, string, error) {
			auth, ok := handle.auths[domain]
			if !ok {
				return "", "", nil
			}

			return auth.Username, auth.Password, nil
		}),
	)
	if err != nil {
		return err
	}

	// Split on ':'
	parsed := strings.SplitN(ref, ":", 2)
	var base string
	if len(parsed) == 2 {
		base = parsed[0]
	} else if len(parsed) == 1 {
		base = ref
	} else {
		return fmt.Errorf("invalid image reference: %s", ref)
	}

	for _, manifest := range manifests {
		image, err := handle.client.GetImage(
			namespaces.WithNamespace(ctx, handle.namespace),
			base+":"+manifest.Digest.String(),
		)
		if err != nil {
			return err
		}

		if err := handle.client.Push(
			namespaces.WithNamespace(ctx, handle.namespace),
			base+":"+manifest.Digest.String(),
			image.Target(),
			containerd.WithResolver(resolver),
		); err != nil {
			return err
		}
	}

	return handle.client.Push(
		namespaces.WithNamespace(ctx, handle.namespace),
		ref,
		*target,
		containerd.WithResolver(resolver),
	)
}

// UnpackImage implements ImageUnpacker.
func (handle *ContainerdHandler) UnpackImage(ctx context.Context, ref string, platform string, dest string) (err error) {
	ctx, done, err := handle.lease(ctx)
	if err != nil {
		return err
	}

	defer func() {
		err = combineErrors(err, done(ctx))
	}()

	img, err := handle.client.ImageService().Get(
		namespaces.WithNamespace(ctx, handle.namespace),
		ref,
	)
	if err != nil {
		return err
	}
	manifests, err := images.Children(
		namespaces.WithNamespace(ctx, handle.namespace),
		handle.client.ContentStore(),
		img.Target,
	)
	if err != nil {
		return err
	}

	// Split on ':'
	parsed := strings.SplitN(ref, ":", 2)
	var base string
	if len(parsed) == 2 {
		base = parsed[0]
	} else if len(parsed) == 1 {
		base = ref
	} else {
		return fmt.Errorf("invalid image reference: %s", ref)
	}

	var i containerd.Image
	for _, manifest := range manifests {
		if manifest.MediaType == ocispec.MediaTypeImageManifest &&
			(fmt.Sprintf("%s/%s", manifest.Platform.OS, manifest.Platform.Architecture) == platform ||
				len(platform) == 0 && len(manifests) == 1) {

			imgBase, err := handle.client.ImageService().Get(
				namespaces.WithNamespace(ctx, handle.namespace),
				base+":"+manifest.Digest.String(),
			)
			if err != nil {
				return err
			}
			i = containerd.NewImage(handle.client, imgBase)

			break
		}
	}

	if i == nil {
		return fmt.Errorf("no matching platform found")
	}

	// TODO: We need to pass the architecture, platform and any desired KConfig
	// values via the platform specifier:
	// i := containerd.NewImageWithPlatform(
	// 	handle.client,
	// 	img,
	// 	platforms.Only(ocispec.Platform{
	// 	// TODO!
	// 	})),
	// )

	if err = i.Unpack(ctx, containerd.DefaultSnapshotter); err != nil {
		return err
	}

	isUnpacked, err := i.IsUnpacked(ctx, containerd.DefaultSnapshotter)
	if err != nil {
		return err
	}

	if !isUnpacked {
		return fmt.Errorf("empty image")
	}

	// TODO(nderjung): This is where we could used media-types to extract the
	// right files.

	layers, err := i.RootFS(ctx)
	if err != nil {
		return err
	}

	for _, layer := range layers {
		log.G(ctx).WithField("digest", layer.String()).Trace("extract layer")

		ra, err := i.ContentStore().ReaderAt(ctx, ocispec.Descriptor{Digest: layer})
		if err != nil {
			return err
		}

		if err := archive.Untar(content.NewReader(ra), dest); err != nil {
			return err
		}

		ra.Close()
	}

	return nil
}

// FinalizeImage implements ImageFinalizer.
func (handle *ContainerdHandler) FinalizeImage(ctx context.Context, image ocispec.Image) error {
	return fmt.Errorf("not implemented: oci.handler.ContainerdHandler.FinalizeImage")
}

// combineErrors is a helper for handling multiple potential errors, combining
// them as necessary. It is meant to be used in a deferred function.
func combineErrors(original, additional error) error {
	switch {
	case additional == nil:
		return original
	case original != nil:
		return fmt.Errorf("%w. Additionally: %w", original, additional)
	default:
		return additional
	}
}
