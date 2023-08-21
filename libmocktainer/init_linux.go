// SPDX-License-Identifier: Apache-2.0
// Copyright 2014 Docker, Inc.
// Copyright 2023 Unikraft GmbH and The KraftKit Authors

package libmocktainer

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/opencontainers/runc/libcontainer/utils"

	"kraftkit.sh/libmocktainer/configs"
)

type initType string

const (
	initStandard initType = "standard"
)

type pid struct {
	Pid           int `json:"stage2_pid"`
	PidFirstChild int `json:"stage1_pid"`
}

// network is an internal struct used to setup container networks.
type network struct {
	configs.Network

	// TempVethPeerName is a unique temporary veth peer name that was placed into
	// the container's namespace.
	TempVethPeerName string `json:"temp_veth_peer_name"`
}

type mountFds struct{}

// initConfig is used for transferring parameters from Exec() to Init()
type initConfig struct {
	Args             []string        `json:"args"`
	Env              []string        `json:"env"`
	ProcessLabel     string          `json:"process_label"`
	AppArmorProfile  string          `json:"apparmor_profile"`
	Config           *configs.Config `json:"config"`
	Networks         []*network      `json:"network"`
	PassedFilesCount int             `json:"passed_files_count"`
	ContainerID      string          `json:"containerid"`
	SpecState        *specs.State    `json:"spec_state,omitempty"`
}

// Init is part of "runc init" implementation.
func Init() {
	runtime.GOMAXPROCS(1)
	runtime.LockOSThread()

	if err := startInitialization(); err != nil {
		// If the error is returned, it was not communicated
		// back to the parent (which is not a common case),
		// so print it to stderr here as a last resort.
		//
		// Do not use logrus as we are not sure if it has been
		// set up yet, but most important, if the parent is
		// alive (and its log forwarding is working).
		fmt.Fprintln(os.Stderr, err)
	}
	// Normally, StartInitialization() never returns, meaning
	// if we are here, it had failed.
	os.Exit(1)
}

// Normally, this function does not return. If it returns, with or without an
// error, it means the initialization has failed. If the error is returned,
// it means the error can not be communicated back to the parent.
func startInitialization() (retErr error) {
	// Get the INITPIPE.
	envInitPipe := os.Getenv("_LIBCONTAINER_INITPIPE")
	pipefd, err := strconv.Atoi(envInitPipe)
	if err != nil {
		return fmt.Errorf("unable to convert _LIBCONTAINER_INITPIPE: %w", err)
	}
	pipe := os.NewFile(uintptr(pipefd), "pipe")
	defer pipe.Close()

	defer func() {
		// If this defer is ever called, this means initialization has failed.
		// Send the error back to the parent process in the form of an initError.
		ierr := initError{Message: retErr.Error()}
		if err := writeSyncArg(pipe, procError, ierr); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		// The error is sent, no need to also return it (or it will be reported twice).
		retErr = nil
	}()

	// Set up logging. This is used rarely, and mostly for init debugging.

	// Passing log level is optional; currently libcontainer/integration does not do it.
	if levelStr := os.Getenv("_LIBCONTAINER_LOGLEVEL"); levelStr != "" {
		logLevel, err := strconv.Atoi(levelStr)
		if err != nil {
			return fmt.Errorf("unable to convert _LIBCONTAINER_LOGLEVEL: %w", err)
		}
		logrus.SetLevel(logrus.Level(logLevel))
	}

	logFD, err := strconv.Atoi(os.Getenv("_LIBCONTAINER_LOGPIPE"))
	if err != nil {
		return fmt.Errorf("unable to convert _LIBCONTAINER_LOGPIPE: %w", err)
	}

	logrus.SetOutput(os.NewFile(uintptr(logFD), "logpipe"))
	logrus.SetFormatter(new(logrus.JSONFormatter))
	logrus.Debug("child process in init()")

	// Only init processes have FIFOFD.
	fifofd := -1
	envInitType := os.Getenv("_LIBCONTAINER_INITTYPE")
	it := initType(envInitType)
	if it == initStandard {
		envFifoFd := os.Getenv("_LIBCONTAINER_FIFOFD")
		if fifofd, err = strconv.Atoi(envFifoFd); err != nil {
			return fmt.Errorf("unable to convert _LIBCONTAINER_FIFOFD: %w", err)
		}
	}

	// clear the current process's environment to clean any libcontainer
	// specific env vars.
	os.Clearenv()

	defer func() {
		if err := recover(); err != nil {
			if err2, ok := err.(error); ok {
				retErr = fmt.Errorf("panic from initialization: %w, %s", err2, debug.Stack())
			} else {
				retErr = fmt.Errorf("panic from initialization: %v, %s", err, debug.Stack())
			}
		}
	}()

	// If init succeeds, it will not return, hence none of the defers will be called.
	return containerInit(it, pipe, nil, fifofd, logFD, mountFds{})
}

func containerInit(t initType, pipe *os.File, _ *os.File, fifoFd, logFd int, _ mountFds) error {
	var config *initConfig
	if err := json.NewDecoder(pipe).Decode(&config); err != nil {
		return err
	}
	if err := populateProcessEnvironment(config.Env); err != nil {
		return err
	}
	switch t {
	case initStandard:
		i := &linuxStandardInit{
			pipe:      pipe,
			parentPid: unix.Getppid(),
			config:    config,
			fifoFd:    fifoFd,
			logFd:     logFd,
		}
		return i.Init()
	}
	return fmt.Errorf("unknown init type %q", t)
}

// current processes's environment.
func populateProcessEnvironment(env []string) error {
	for _, pair := range env {
		p := strings.SplitN(pair, "=", 2)
		if len(p) < 2 {
			return errors.New("invalid environment variable: missing '='")
		}
		name, val := p[0], p[1]
		if name == "" {
			return errors.New("invalid environment variable: name cannot be empty")
		}
		if strings.IndexByte(name, 0) >= 0 {
			return fmt.Errorf("invalid environment variable %q: name contains nul byte (\\x00)", name)
		}
		if strings.IndexByte(val, 0) >= 0 {
			return fmt.Errorf("invalid environment variable %q: value contains nul byte (\\x00)", name)
		}
		if err := os.Setenv(name, val); err != nil {
			return err
		}
	}
	return nil
}

// finalizeNamespace drops the caps, sets the correct user
// and working dir, and closes any leaked file descriptors
// before executing the command inside the namespace
func finalizeNamespace(config *initConfig) error {
	// Ensure that all unwanted fds we may have accidentally
	// inherited are marked close-on-exec so they stay out of the
	// container
	if err := utils.CloseExecFrom(config.PassedFilesCount + 3); err != nil {
		return fmt.Errorf("error closing exec fds: %w", err)
	}

	return nil
}

// syncParentReady sends to the given pipe a JSON payload which indicates that
// the init is ready to Exec the child process. It then waits for the parent to
// indicate that it is cleared to Exec.
func syncParentReady(pipe *os.File) error {
	// Tell parent.
	if err := writeSync(pipe, procReady); err != nil {
		return err
	}
	// Wait for parent to give the all-clear.
	return readSync(pipe, procRun)
}

// setupNetwork sets up and initializes any network interface inside the container.
func setupNetwork(config *initConfig) error {
	for _, config := range config.Networks {
		strategy, err := getStrategy(config.Type)
		if err != nil {
			return err
		}
		if err := strategy.initialize(config); err != nil {
			return err
		}
	}
	return nil
}

func setupRoute(config *configs.Config) error {
	for _, config := range config.Routes {
		_, dst, err := net.ParseCIDR(config.Destination)
		if err != nil {
			return err
		}
		src := net.ParseIP(config.Source)
		if src == nil {
			return fmt.Errorf("Invalid source for route: %s", config.Source)
		}
		gw := net.ParseIP(config.Gateway)
		if gw == nil {
			return fmt.Errorf("Invalid gateway for route: %s", config.Gateway)
		}
		l, err := netlink.LinkByName(config.InterfaceName)
		if err != nil {
			return err
		}
		route := &netlink.Route{
			Scope:     netlink.SCOPE_UNIVERSE,
			Dst:       dst,
			Src:       src,
			Gw:        gw,
			LinkIndex: l.Attrs().Index,
		}
		if err := netlink.RouteAdd(route); err != nil {
			return err
		}
	}
	return nil
}
