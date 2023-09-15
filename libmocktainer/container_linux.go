// SPDX-License-Identifier: Apache-2.0
// Copyright 2014 Docker, Inc.
// Copyright 2023 Unikraft GmbH and The KraftKit Authors

package libmocktainer

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/opencontainers/runc/libcontainer/system"
	"github.com/opencontainers/runc/libcontainer/utils"

	"kraftkit.sh/libmocktainer/configs"
)

const stdioFdCount = 3

// Container is a libcontainer container object.
type Container struct {
	id                   string
	root                 string
	config               *configs.Config
	initProcess          parentProcess
	initProcessStartTime uint64
	m                    sync.Mutex
	state                containerState
	created              time.Time
	fifo                 *os.File
}

// State represents a running container's state
type State struct {
	BaseState

	// Platform specific fields below here

	// NamespacePaths are filepaths to the container's namespaces. Key is the namespace type
	// with the value as the path.
	NamespacePaths map[configs.NamespaceType]string `json:"namespace_paths"`
}

// ID returns the container's unique ID
func (c *Container) ID() string {
	return c.id
}

// Config returns the container's configuration
func (c *Container) Config() configs.Config {
	return *c.config
}

// Status returns the current status of the container.
func (c *Container) Status() (Status, error) {
	c.m.Lock()
	defer c.m.Unlock()
	return c.currentStatus()
}

// State returns the current container's state information.
func (c *Container) State() (*State, error) {
	c.m.Lock()
	defer c.m.Unlock()
	return c.currentState()
}

// Start starts a process inside the container. Returns error if process fails
// to start. You can track process lifecycle with passed Process structure.
func (c *Container) Start(process *Process) error {
	c.m.Lock()
	defer c.m.Unlock()
	if err := c.createExecFifo(); err != nil {
		return err
	}
	if err := c.start(process); err != nil {
		c.deleteExecFifo()
		return err
	}
	return nil
}

// Exec signals the container to exec the users process at the end of the init.
func (c *Container) Exec() error {
	c.m.Lock()
	defer c.m.Unlock()
	return c.exec()
}

func (c *Container) exec() error {
	path := filepath.Join(c.root, execFifoFilename)
	pid := c.initProcess.pid()
	blockingFifoOpenCh := awaitFifoOpen(path)
	for {
		select {
		case result := <-blockingFifoOpenCh:
			return handleFifoResult(result)

		case <-time.After(time.Millisecond * 100):
			stat, err := system.Stat(pid)
			if err != nil || stat.State == system.Zombie {
				// could be because process started, ran, and completed between our 100ms timeout and our system.Stat() check.
				// see if the fifo exists and has data (with a non-blocking open, which will succeed if the writing process is complete).
				if err := handleFifoResult(fifoOpen(path, false)); err != nil {
					return errors.New("container process is already dead")
				}
				return nil
			}
		}
	}
}

func readFromExecFifo(execFifo io.Reader) error {
	data, err := io.ReadAll(execFifo)
	if err != nil {
		return err
	}
	if len(data) <= 0 {
		return errors.New("cannot start an already running container")
	}
	return nil
}

func awaitFifoOpen(path string) <-chan openResult {
	fifoOpened := make(chan openResult)
	go func() {
		result := fifoOpen(path, true)
		fifoOpened <- result
	}()
	return fifoOpened
}

func fifoOpen(path string, block bool) openResult {
	flags := os.O_RDONLY
	if !block {
		flags |= unix.O_NONBLOCK
	}
	f, err := os.OpenFile(path, flags, 0)
	if err != nil {
		return openResult{err: fmt.Errorf("exec fifo: %w", err)}
	}
	return openResult{file: f}
}

func handleFifoResult(result openResult) error {
	if result.err != nil {
		return result.err
	}
	f := result.file
	defer f.Close()
	if err := readFromExecFifo(f); err != nil {
		return err
	}
	return os.Remove(f.Name())
}

type openResult struct {
	file *os.File
	err  error
}

func (c *Container) start(process *Process) (retErr error) {
	parent, err := c.newParentProcess(process)
	if err != nil {
		return fmt.Errorf("unable to create new parent process: %w", err)
	}

	logsDone := parent.forwardChildLogs()
	if logsDone != nil {
		defer func() {
			// Wait for log forwarder to finish. This depends on
			// runc init closing the _LIBCONTAINER_LOGPIPE log fd.
			err := <-logsDone
			if err != nil && retErr == nil {
				retErr = fmt.Errorf("unable to forward init logs: %w", err)
			}
		}()
	}

	if err := parent.start(); err != nil {
		return fmt.Errorf("unable to start container process: %w", err)
	}

	c.fifo.Close()
	if c.config.Hooks != nil {
		s, err := c.currentOCIState()
		if err != nil {
			return err
		}

		if err := c.config.Hooks[configs.Poststart].RunHooks(s); err != nil {
			if err := ignoreTerminateErrors(parent.terminate()); err != nil {
				logrus.Warn(fmt.Errorf("error running poststart hook: %w", err))
			}
			return err
		}
	}
	return nil
}

// Signal sends a specified signal to container's init.
//
// When s is SIGKILL and the container does not have its own PID namespace, all
// the container's processes are killed. In this scenario, the libcontainer
// user may be required to implement a proper child reaper.
func (c *Container) Signal(s os.Signal) error {
	c.m.Lock()
	defer c.m.Unlock()
	status, err := c.currentStatus()
	if err != nil {
		return err
	}
	// To avoid a PID reuse attack, don't kill non-running container.
	switch status {
	case Running, Created:
	default:
		return ErrNotRunning
	}

	err = c.initProcess.signal(s)
	if err != nil {
		return fmt.Errorf("unable to signal init: %w", err)
	}
	return nil
}

func (c *Container) createExecFifo() error {
	rootuid := 0
	rootgid := 0

	fifoName := filepath.Join(c.root, execFifoFilename)
	if _, err := os.Stat(fifoName); err == nil {
		return fmt.Errorf("exec fifo %s already exists", fifoName)
	}
	oldMask := unix.Umask(0o000)
	if err := unix.Mkfifo(fifoName, 0o622); err != nil {
		unix.Umask(oldMask)
		return err
	}
	unix.Umask(oldMask)
	return os.Chown(fifoName, rootuid, rootgid)
}

func (c *Container) deleteExecFifo() {
	fifoName := filepath.Join(c.root, execFifoFilename)
	os.Remove(fifoName)
}

// includeExecFifo opens the container's execfifo as a pathfd, so that the
// container cannot access the statedir (and the FIFO itself remains
// un-opened). It then adds the FifoFd to the given exec.Cmd as an inherited
// fd, with _LIBCONTAINER_FIFOFD set to its fd number.
func (c *Container) includeExecFifo(cmd *exec.Cmd) error {
	fifoName := filepath.Join(c.root, execFifoFilename)
	fifo, err := os.OpenFile(fifoName, unix.O_PATH|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	c.fifo = fifo

	cmd.ExtraFiles = append(cmd.ExtraFiles, fifo)
	cmd.Env = append(cmd.Env,
		"_LIBCONTAINER_FIFOFD="+strconv.Itoa(stdioFdCount+len(cmd.ExtraFiles)-1))
	return nil
}

func (c *Container) newParentProcess(p *Process) (parentProcess, error) {
	parentInitPipe, childInitPipe, err := utils.NewSockPair("init")
	if err != nil {
		return nil, fmt.Errorf("unable to create init pipe: %w", err)
	}
	messageSockPair := filePair{parentInitPipe, childInitPipe}

	parentLogPipe, childLogPipe, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("unable to create log pipe: %w", err)
	}
	logFilePair := filePair{parentLogPipe, childLogPipe}

	cmd := c.commandTemplate(p, childInitPipe, childLogPipe)

	if err := c.includeExecFifo(cmd); err != nil {
		return nil, fmt.Errorf("unable to setup exec fifo: %w", err)
	}
	return c.newInitProcess(p, cmd, messageSockPair, logFilePair)
}

func (c *Container) commandTemplate(p *Process, childInitPipe *os.File, childLogPipe *os.File) *exec.Cmd {
	cmd := exec.Command("/proc/self/exe", "init")
	cmd.Args[0] = os.Args[0]
	cmd.Stdin = p.Stdin
	cmd.Stdout = p.Stdout
	cmd.Stderr = p.Stderr
	cmd.Dir = c.config.Rootfs
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &unix.SysProcAttr{}
	}
	cmd.Env = append(cmd.Env, "GOMAXPROCS="+os.Getenv("GOMAXPROCS"))
	cmd.ExtraFiles = append(cmd.ExtraFiles, p.ExtraFiles...)
	cmd.ExtraFiles = append(cmd.ExtraFiles, childInitPipe)
	cmd.Env = append(cmd.Env,
		"_LIBCONTAINER_INITPIPE="+strconv.Itoa(stdioFdCount+len(cmd.ExtraFiles)-1),
		"_LIBCONTAINER_STATEDIR="+c.root,
	)

	cmd.ExtraFiles = append(cmd.ExtraFiles, childLogPipe)
	cmd.Env = append(cmd.Env,
		"_LIBCONTAINER_LOGPIPE="+strconv.Itoa(stdioFdCount+len(cmd.ExtraFiles)-1))
	if p.LogLevel != "" {
		cmd.Env = append(cmd.Env, "_LIBCONTAINER_LOGLEVEL="+p.LogLevel)
	}

	// NOTE: when running a container with no PID namespace and the parent process spawning the container is
	// PID1 the pdeathsig is being delivered to the container's init process by the kernel for some reason
	// even with the parent still running.
	if c.config.ParentDeathSignal > 0 {
		cmd.SysProcAttr.Pdeathsig = unix.Signal(c.config.ParentDeathSignal)
	}
	return cmd
}

func (c *Container) newInitProcess(p *Process, cmd *exec.Cmd, messageSockPair, logFilePair filePair) (*initProcess, error) {
	cmd.Env = append(cmd.Env, "_LIBCONTAINER_INITTYPE="+string(initStandard))
	nsMaps := make(map[configs.NamespaceType]string)
	for _, ns := range c.config.Namespaces {
		if ns.Path != "" {
			nsMaps[ns.Type] = ns.Path
		}
	}
	data, err := c.bootstrapData(c.config.Namespaces.CloneFlags(), nsMaps, initStandard)
	if err != nil {
		return nil, err
	}

	init := &initProcess{
		cmd:             cmd,
		messageSockPair: messageSockPair,
		logFilePair:     logFilePair,
		config:          c.newInitConfig(p),
		container:       c,
		process:         p,
		bootstrapData:   data,
	}
	c.initProcess = init
	return init, nil
}

func (c *Container) newInitConfig(process *Process) *initConfig {
	cfg := &initConfig{
		Config:           c.config,
		Args:             process.Args,
		Env:              process.Env,
		PassedFilesCount: len(process.ExtraFiles),
		ContainerID:      c.ID(),
		AppArmorProfile:  c.config.AppArmorProfile,
		ProcessLabel:     c.config.ProcessLabel,
	}
	if process.AppArmorProfile != "" {
		cfg.AppArmorProfile = process.AppArmorProfile
	}
	if process.Label != "" {
		cfg.ProcessLabel = process.Label
	}
	return cfg
}

// Destroy destroys the container, if its in a valid state.
//
// Any event registrations are removed before the container is destroyed.
// No error is returned if the container is already destroyed.
//
// Running containers must first be stopped using Signal.
// Paused containers must first be resumed using Resume.
func (c *Container) Destroy() error {
	c.m.Lock()
	defer c.m.Unlock()
	return c.state.destroy()
}

func (c *Container) updateState(process parentProcess) (*State, error) {
	if process != nil {
		c.initProcess = process
	}
	state, err := c.currentState()
	if err != nil {
		return nil, err
	}
	err = c.saveState(state)
	if err != nil {
		return nil, err
	}
	return state, nil
}

func (c *Container) saveState(s *State) (retErr error) {
	tmpFile, err := os.CreateTemp(c.root, "state-")
	if err != nil {
		return err
	}

	defer func() {
		if retErr != nil {
			tmpFile.Close()
			os.Remove(tmpFile.Name())
		}
	}()

	err = utils.WriteJSON(tmpFile, s)
	if err != nil {
		return err
	}
	err = tmpFile.Close()
	if err != nil {
		return err
	}

	stateFilePath := filepath.Join(c.root, stateFilename)
	return os.Rename(tmpFile.Name(), stateFilePath)
}

func (c *Container) currentStatus() (Status, error) {
	if err := c.refreshState(); err != nil {
		return -1, err
	}
	return c.state.status(), nil
}

// refreshState needs to be called to verify that the current state on the
// container is what is true.  Because consumers of libcontainer can use it
// out of process we need to verify the container's status based on runtime
// information and not rely on our in process info.
func (c *Container) refreshState() error {
	t := c.runType()
	switch t {
	case Created:
		return c.state.transition(&createdState{c: c})
	case Running:
		return c.state.transition(&runningState{c: c})
	}
	return c.state.transition(&stoppedState{c: c})
}

func (c *Container) runType() Status {
	if c.initProcess == nil {
		return Stopped
	}
	pid := c.initProcess.pid()
	stat, err := system.Stat(pid)
	if err != nil {
		return Stopped
	}
	if stat.StartTime != c.initProcessStartTime || stat.State == system.Zombie || stat.State == system.Dead {
		return Stopped
	}
	// We'll create exec fifo and blocking on it after container is created,
	// and delete it after start container.
	if _, err := os.Stat(filepath.Join(c.root, execFifoFilename)); err == nil {
		return Created
	}
	return Running
}

func (c *Container) currentState() (*State, error) {
	var (
		startTime uint64
		pid       = -1
	)
	if c.initProcess != nil {
		pid = c.initProcess.pid()
		startTime, _ = c.initProcess.startTime()
	}

	state := &State{
		BaseState: BaseState{
			ID:                   c.ID(),
			Config:               *c.config,
			InitProcessPid:       pid,
			InitProcessStartTime: startTime,
			Created:              c.created,
		},
		NamespacePaths: make(map[configs.NamespaceType]string),
	}
	if pid > 0 {
		for _, ns := range c.config.Namespaces {
			state.NamespacePaths[ns.Type] = ns.GetPath(pid)
		}
		for _, nsType := range configs.NamespaceTypes() {
			if !configs.IsNamespaceSupported(nsType) {
				continue
			}
			if _, ok := state.NamespacePaths[nsType]; !ok {
				ns := configs.Namespace{Type: nsType}
				state.NamespacePaths[ns.Type] = ns.GetPath(pid)
			}
		}
	}
	return state, nil
}

func (c *Container) currentOCIState() (*specs.State, error) {
	bundle, annotations := utils.Annotations(c.config.Labels)
	state := &specs.State{
		Version:     specs.Version,
		ID:          c.ID(),
		Bundle:      bundle,
		Annotations: annotations,
	}
	status, err := c.currentStatus()
	if err != nil {
		return nil, err
	}
	state.Status = specs.ContainerState(status.String())
	if status != Stopped {
		if c.initProcess != nil {
			state.Pid = c.initProcess.pid()
		}
	}
	return state, nil
}

// orderNamespacePaths sorts namespace paths into a list of paths that we
// can setns in order.
func (c *Container) orderNamespacePaths(namespaces map[configs.NamespaceType]string) ([]string, error) {
	paths := []string{}
	for _, ns := range configs.NamespaceTypes() {

		// Remove namespaces that we don't need to join.
		if !c.config.Namespaces.Contains(ns) {
			continue
		}

		if p, ok := namespaces[ns]; ok && p != "" {
			// check if the requested namespace is supported
			if !configs.IsNamespaceSupported(ns) {
				return nil, fmt.Errorf("namespace %s is not supported", ns)
			}
			// only set to join this namespace if it exists
			if _, err := os.Lstat(p); err != nil {
				return nil, fmt.Errorf("namespace path: %w", err)
			}
			// do not allow namespace path with comma as we use it to separate
			// the namespace paths
			if strings.ContainsRune(p, ',') {
				return nil, fmt.Errorf("invalid namespace path %s", p)
			}
			paths = append(paths, fmt.Sprintf("%s:%s", configs.NsName(ns), p))
		}

	}

	return paths, nil
}

// netlinkError is an error wrapper type for use by custom netlink message
// types. Panics with errors are wrapped in netlinkError so that the recover
// in bootstrapData can distinguish intentional panics.
type netlinkError struct{ error }

// bootstrapData encodes the necessary data in netlink binary format
// as a io.Reader.
// Consumer can write the data to a bootstrap program
// such as one that uses nsenter package to bootstrap the container's
// init process correctly, i.e. with correct namespaces, uid/gid
// mapping etc.
func (c *Container) bootstrapData(cloneFlags uintptr, nsMaps map[configs.NamespaceType]string, _ initType) (_ io.Reader, Err error) {
	// create the netlink message
	r := nl.NewNetlinkRequest(int(InitMsg), 0)

	// Our custom messages cannot bubble up an error using returns, instead
	// they will panic with the specific error type, netlinkError. In that
	// case, recover from the panic and return that as an error.
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(netlinkError); ok {
				Err = e.error
			} else {
				panic(r)
			}
		}
	}()

	// write cloneFlags
	r.AddData(&Int32msg{
		Type:  CloneFlagsAttr,
		Value: uint32(cloneFlags),
	})

	// write custom namespace paths
	if len(nsMaps) > 0 {
		nsPaths, err := c.orderNamespacePaths(nsMaps)
		if err != nil {
			return nil, err
		}
		r.AddData(&Bytemsg{
			Type:  NsPathsAttr,
			Value: []byte(strings.Join(nsPaths, ",")),
		})
	}

	return bytes.NewReader(r.Serialize()), nil
}

// ignoreTerminateErrors returns nil if the given err matches an error known
// to indicate that the terminate occurred successfully or err was nil, otherwise
// err is returned unaltered.
func ignoreTerminateErrors(err error) error {
	if err == nil {
		return nil
	}
	// terminate() might return an error from either Kill or Wait.
	// The (*Cmd).Wait documentation says: "If the command fails to run
	// or doesn't complete successfully, the error is of type *ExitError".
	// Filter out such errors (like "exit status 1" or "signal: killed").
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return nil
	}
	if errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	s := err.Error()
	if strings.Contains(s, "Wait was already called") {
		return nil
	}
	return err
}
