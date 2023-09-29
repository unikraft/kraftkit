package oci

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"kraftkit.sh/config"
	"kraftkit.sh/log"
	"kraftkit.sh/oci/handler"

	cliconfig "github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	regtypes "github.com/docker/docker/api/types/registry"
	"github.com/genuinetools/reg/repoutils"
	"github.com/mitchellh/go-homedir"
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
				return handler.NewContainerdHandler(ctx, contAddr, namespace, manager.auths)
			}

			return nil
		}

		// Fall-back to using a simpler directory/tarball-based OCI handler
		ociDir := filepath.Join(config.G[config.KraftKit](ctx).RuntimeDir, "oci")

		log.G(ctx).WithFields(logrus.Fields{
			"path": ociDir,
		}).Trace("using oci directory handler")

		manager.handle = func(ctx context.Context) (context.Context, handler.Handler, error) {
			handle, err := handler.NewDirectoryHandler(ociDir, manager.auths)
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
			return handler.NewContainerdHandler(ctx, addr, namespace, manager.auths)
		}

		return nil
	}
}

// WithDirectory forces the use of a directory handler by providing a path to
// the directory to use as the OCI root.
func WithDirectory(ctx context.Context, path string) OCIManagerOption {
	return func(ctx context.Context, manager *ociManager) error {
		log.G(ctx).WithFields(logrus.Fields{
			"path": path,
		}).Trace("using oci directory handler")

		manager.handle = func(ctx context.Context) (context.Context, handler.Handler, error) {
			handle, err := handler.NewDirectoryHandler(path, manager.auths)
			if err != nil {
				return nil, nil, err
			}

			return ctx, handle, nil
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

// fileExists returns true if the given path exists and is not a directory.
func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

// defaultAuths uses the provided context to locate possible authentication
// values which can be used when speaking with remote registries.
func defaultAuths(ctx context.Context) (map[string]regtypes.AuthConfig, error) {
	auths := make(map[string]regtypes.AuthConfig)

	for domain, auth := range config.G[config.KraftKit](ctx).Auth {
		auth, err := repoutils.GetAuthConfig(auth.User, auth.Token, domain)
		if err != nil {
			return nil, err
		}

		auths[domain] = auth
	}

	// Podman users may have their container registry auth configured in a
	// different location, that Docker packages aren't aware of.
	// If the Docker config file isn't found, we'll fallback to look where
	// Podman configures it, and parse that as a Docker auth config instead.

	// First, check $HOME/.docker/
	var home string
	var err error
	foundDockerConfig := false

	// If this is run in the context of GitHub actions, use an alternative path
	// for the $HOME.
	if os.Getenv("GITUB_ACTION") == "yes" {
		home = "/github/home"
	} else {
		home, err = homedir.Dir()
	}
	if err == nil {
		foundDockerConfig = fileExists(filepath.Join(home, ".docker/config.json"))
	}

	// If $HOME/.docker/config.json isn't found, check $DOCKER_CONFIG (if set)
	if !foundDockerConfig && os.Getenv("DOCKER_CONFIG") != "" {
		foundDockerConfig = fileExists(filepath.Join(os.Getenv("DOCKER_CONFIG"), "config.json"))
	}

	// If either of those locations are found, load it using Docker's
	// config.Load, which may fail if the config can't be parsed.
	//
	// If neither was found, look for Podman's auth at
	// $XDG_RUNTIME_DIR/containers/auth.json and attempt to load it as a
	// Docker config.
	var cf *configfile.ConfigFile
	if foundDockerConfig {
		cf, err = cliconfig.Load(os.Getenv("DOCKER_CONFIG"))
		if err != nil {
			return nil, err
		}
	} else if f, err := os.Open(filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "containers/auth.json")); err == nil {
		defer f.Close()

		cf, err = cliconfig.LoadFromReader(f)
		if err != nil {
			return nil, err
		}
	}

	if cf != nil {
		for domain, config := range cf.AuthConfigs {
			auths[domain] = regtypes.AuthConfig{
				Auth:          config.Auth,
				Email:         config.Email,
				IdentityToken: config.IdentityToken,
				Password:      config.Password,
				RegistryToken: config.RegistryToken,
				ServerAddress: config.ServerAddress,
				Username:      config.Username,
			}
		}
	}

	return auths, nil
}

// WithDefaultAuth uses the KraftKit-set configuration for authentication
// against remote registries.
func WithDefaultAuth() OCIManagerOption {
	return func(ctx context.Context, manager *ociManager) error {
		var err error

		manager.auths, err = defaultAuths(ctx)
		if err != nil {
			return err
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
			return fmt.Errorf("cannot use auth config without server address")
		}

		if manager.auths == nil {
			manager.auths = make(map[string]regtypes.AuthConfig, 1)
		}

		manager.auths[auth.ServerAddress] = auth
		return nil
	}
}
