package oci

import (
	"context"
	"os"
	"path/filepath"

	regtypes "github.com/docker/docker/api/types/registry"
	"github.com/genuinetools/reg/repoutils"
	"github.com/juju/errors"
	"github.com/sirupsen/logrus"
	"kraftkit.sh/config"
	"kraftkit.sh/log"
	"kraftkit.sh/oci/handler"
)

type OCIManagerOption func(context.Context, *ociManager) error

// WithDetectHandler uses internal KraftKit configuration to determine which
// underlying OCI handler implementation should be used. Ultimately, this is
// done by checking whether set configuration can ultimately invoke a relative
// client to enable the handler.
func WithDetectHandler() OCIManagerOption {
	return func(ctx context.Context, manager *ociManager) error {
		if contAddr := config.G[config.KraftKit](ctx).ContainerdAddr; len(contAddr) > 0 {
			namespace := DefaultNamespace
			if n := os.Getenv("CONTAINERD_NAMESPACE"); n != "" {
				namespace = n
			}

			log.G(ctx).WithFields(logrus.Fields{
				"addr":      contAddr,
				"namespace": namespace,
			}).Trace("using oci containerd handler")

			manager.handle = func(ctx context.Context) (context.Context, handler.Handler, error) {
				return handler.NewContainerdHandler(ctx, contAddr, namespace)
			}

			return nil
		}

		// Fall-back to using a simpler directory/tarball-based OCI handler
		ociDir := filepath.Join(config.G[config.KraftKit](ctx).RuntimeDir, "oci")

		log.G(ctx).WithFields(logrus.Fields{
			"path": ociDir,
		}).Trace("using oci directory handler")

		manager.handle = func(ctx context.Context) (context.Context, handler.Handler, error) {
			handle, err := handler.NewDirectoryHandler(ociDir)
			if err != nil {
				return nil, nil, err
			}

			return ctx, handle, nil
		}

		return nil
	}
}

// WithContainerd forces the use of a containerd handler by providing an address
// to the containerd daemon (whether UNIX socket or TCP socket) as well as the
// default namespace to operate within.
func WithContainerd(ctx context.Context, addr, namespace string) OCIManagerOption {
	return func(ctx context.Context, manager *ociManager) error {
		if n := os.Getenv("CONTAINERD_NAMESPACE"); n != "" {
			namespace = n
		} else if namespace == "" {
			namespace = DefaultNamespace
		}

		log.G(ctx).WithFields(logrus.Fields{
			"addr":      addr,
			"namespace": namespace,
		}).Trace("using containerd handler")

		manager.handle = func(ctx context.Context) (context.Context, handler.Handler, error) {
			return handler.NewContainerdHandler(ctx, addr, namespace)
		}

		return nil
	}
}

// WithDefaultRegistries sets the list of KraftKit-set registries which is
// defined through its configuration.
func WithDefaultRegistries() OCIManagerOption {
	return func(ctx context.Context, manager *ociManager) error {
		manager.registries = make([]string, 0)

		for _, manifest := range config.G[config.KraftKit](ctx).Unikraft.Manifests {
			if reg, err := manager.registry(ctx, manifest); err == nil && reg.Ping(ctx) == nil {
				manager.registries = append(manager.registries, manifest)
			}
		}

		if len(manager.registries) == 0 {
			manager.registries = []string{DefaultRegistry}
		}

		return nil
	}
}

// WithDefaultAuth uses the KraftKit-set configuration for authentication
// against remote registries.
func WithDefaultAuth() OCIManagerOption {
	return func(ctx context.Context, manager *ociManager) error {
		if manager.auths == nil {
			manager.auths = make(map[string]regtypes.AuthConfig)
		}

		for domain, auth := range config.G[config.KraftKit](ctx).Auth {
			auth, err := repoutils.GetAuthConfig(auth.User, auth.Token, domain)
			if err != nil {
				return err
			}

			manager.auths[domain] = auth
		}

		return nil
	}
}

// WithRegistries sets the list of registries to use when making calls to
// non-canonically named OCI references.
func WithRegistries(registries ...string) OCIManagerOption {
	return func(ctx context.Context, manager *ociManager) error {
		manager.registries = registries
		return nil
	}
}

// WithDockerConfig sets the authentication configuration to use when making
// calls to authenticated registries.
func WithDockerConfig(auth regtypes.AuthConfig) OCIManagerOption {
	return func(ctx context.Context, manager *ociManager) error {
		if auth.ServerAddress == "" {
			return errors.New("cannot use auth config without server address")
		}

		if manager.auths == nil {
			manager.auths = make(map[string]regtypes.AuthConfig, 1)
		}

		manager.auths[auth.ServerAddress] = auth
		return nil
	}
}
