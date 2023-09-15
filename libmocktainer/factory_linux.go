// SPDX-License-Identifier: Apache-2.0
// Copyright 2014 Docker, Inc.
// Copyright 2023 Unikraft GmbH and The KraftKit Authors

package libmocktainer

import (
	"encoding/json"
	"errors"
	"os"

	securejoin "github.com/cyphar/filepath-securejoin"

	"github.com/opencontainers/runc/libcontainer/utils"

	"kraftkit.sh/libmocktainer/configs"
	"kraftkit.sh/libmocktainer/configs/validate"
)

const (
	stateFilename    = "state.json"
	execFifoFilename = "exec.fifo"
)

// Create creates a new container with the given id inside a given state
// directory (root), and returns a Container object.
//
// The root is a state directory which many containers can share. It can be
// used later to get the list of containers, or to get information about a
// particular container (see Load).
//
// The id must not be empty and consist of only the following characters:
// ASCII letters, digits, underscore, plus, minus, period. The id must be
// unique and non-existent for the given root path.
func Create(root, id string, config *configs.Config) (*Container, error) {
	if root == "" {
		return nil, errors.New("root not set")
	}
	if err := validateID(id); err != nil {
		return nil, err
	}
	if err := validate.Validate(config); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	containerRoot, err := securejoin.SecureJoin(root, id)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(containerRoot); err == nil {
		return nil, ErrExist
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	// Parent directory is already created above, so Mkdir is enough.
	if err := os.Mkdir(containerRoot, 0o711); err != nil {
		return nil, err
	}
	c := &Container{
		id:     id,
		root:   containerRoot,
		config: config,
	}
	c.state = &stoppedState{c: c}
	return c, nil
}

// Load takes a path to the state directory (root) and an id of an existing
// container, and returns a Container object reconstructed from the saved
// state. This presents a read only view of the container.
func Load(root, id string) (*Container, error) {
	if root == "" {
		return nil, errors.New("root not set")
	}
	// when load, we need to check id is valid or not.
	if err := validateID(id); err != nil {
		return nil, err
	}
	containerRoot, err := securejoin.SecureJoin(root, id)
	if err != nil {
		return nil, err
	}
	state, err := loadState(containerRoot)
	if err != nil {
		return nil, err
	}
	r := &nonChildProcess{
		processPid:       state.InitProcessPid,
		processStartTime: state.InitProcessStartTime,
	}
	c := &Container{
		initProcess:          r,
		initProcessStartTime: state.InitProcessStartTime,
		id:                   id,
		config:               &state.Config,
		root:                 containerRoot,
		created:              state.Created,
	}
	c.state = &loadedState{c: c}
	if err := c.refreshState(); err != nil {
		return nil, err
	}
	return c, nil
}

func loadState(root string) (*State, error) {
	stateFilePath, err := securejoin.SecureJoin(root, stateFilename)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(stateFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotExist
		}
		return nil, err
	}
	defer f.Close()
	var state *State
	if err := json.NewDecoder(f).Decode(&state); err != nil {
		return nil, err
	}
	return state, nil
}

// validateID checks if the supplied container ID is valid, returning
// the ErrInvalidID in case it is not.
//
// The format of valid ID was never formally defined, instead the code
// was modified to allow or disallow specific characters.
//
// Currently, a valid ID is a non-empty string consisting only of
// the following characters:
// - uppercase (A-Z) and lowercase (a-z) Latin letters;
// - digits (0-9);
// - underscore (_);
// - plus sign (+);
// - minus sign (-);
// - period (.).
//
// In addition, IDs that can't be used to represent a file name
// (such as . or ..) are rejected.
func validateID(id string) error {
	if len(id) < 1 {
		return ErrInvalidID
	}

	// Allowed characters: 0-9 A-Z a-z _ + - .
	for i := 0; i < len(id); i++ {
		c := id[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '_':
		case c == '+':
		case c == '-':
		case c == '.':
		default:
			return ErrInvalidID
		}

	}

	if string(os.PathSeparator)+id != utils.CleanPath(string(os.PathSeparator)+id) {
		return ErrInvalidID
	}

	return nil
}
