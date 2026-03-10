package compose

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const minimalCompose = `
services:
  app:
    image: alpine
`

const invalidCompose = `
services:
  app: {}
`

func writeComposeFile(t *testing.T, dir, name, content string) {
	t.Helper()

	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(content), 0o644)
	if err != nil {
		t.Fatalf("failed to write compose file: %v", err)
	}
}

func TestNewProjectFromComposeFile(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		createFile     bool
		fileName       string
		explicitInput  string
		content        string
		expectErr      bool
		expectDetected string
	}{
		{
			name:      "no file and no explicit",
			expectErr: true,
		},
		{
			name:           "auto detect default file",
			createFile:     true,
			fileName:       DefaultFileNames[0],
			content:        minimalCompose,
			expectDetected: DefaultFileNames[0],
		},
		{
			name:           "explicit file provided",
			createFile:     true,
			fileName:       "custom.yml",
			explicitInput:  "custom.yml",
			content:        minimalCompose,
			expectDetected: "custom.yml",
		},
		{
			name:          "explicit file missing",
			explicitInput: "doesnotexist.yml",
			expectErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workdir := t.TempDir()

			if tt.createFile {
				writeComposeFile(t, workdir, tt.fileName, tt.content)
			}

			project, err := NewProjectFromComposeFile(
				ctx,
				workdir,
				tt.explicitInput,
			)

			if tt.expectErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, project)

			assert.Len(t, project.ComposeFiles, 1)
			assert.Equal(t, tt.expectDetected, project.ComposeFiles[0])
			assert.Equal(t, workdir, project.WorkingDir)
		})
	}
}

func TestProjectValidate(t *testing.T) {
	ctx := context.Background()
	workdir := t.TempDir()

	tests := []struct {
		name      string
		content   string
		expectErr bool
	}{
		{
			name:      "valid service with image",
			content:   minimalCompose,
			expectErr: false,
		},
		{
			name:      "invalid service no image no build",
			content:   invalidCompose,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeComposeFile(t, workdir, "compose.yml", tt.content)

			project, err := NewProjectFromComposeFile(ctx, workdir, "compose.yml")
			if tt.expectErr {
				require.Error(t, err)
				return
			}

			err = project.Validate(ctx)

			if err != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAssignIPs(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		project        *Project
		expectErr      bool
		expectAssigned bool
	}{
		{
			name: "dynamic ip assigned successfully",
			project: &Project{
				Project: &types.Project{
					Networks: types.Networks{
						"net1": {
							Name:     "net1",
							External: false,
							Ipam: types.IPAMConfig{
								Config: []*types.IPAMPool{
									{Subnet: "192.168.1.0/29"},
								},
							},
						},
					},
					Services: types.Services{
						"app": {
							Name: "app",
							Networks: map[string]*types.ServiceNetworkConfig{
								"net1": {},
							},
						},
					},
				},
			},
			expectAssigned: true,
		},
		{
			name: "static ip respected",
			project: &Project{
				Project: &types.Project{
					Networks: types.Networks{
						"net1": {
							Name:     "net1",
							External: false,
							Ipam: types.IPAMConfig{
								Config: []*types.IPAMPool{
									{Subnet: "192.168.1.0/29"},
								},
							},
						},
					},
					Services: types.Services{
						"app": {
							Name: "app",
							Networks: map[string]*types.ServiceNetworkConfig{
								"net1": {
									Ipv4Address: "192.168.1.5",
								},
							},
						},
					},
				},
			},
			expectAssigned: true,
		},
		{
			name: "invalid subnet",
			project: &Project{
				Project: &types.Project{
					Networks: types.Networks{
						"net1": {
							Name: "net1",
							Ipam: types.IPAMConfig{
								Config: []*types.IPAMPool{
									{Subnet: "invalid"},
								},
							},
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "service references missing network",
			project: &Project{
				Project: &types.Project{
					Networks: types.Networks{},
					Services: types.Services{
						"app": {
							Name: "app",
							Networks: map[string]*types.ServiceNetworkConfig{
								"ghost": {},
							},
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "subnet exhaustion",
			project: &Project{
				Project: &types.Project{
					Networks: types.Networks{
						"net1": {
							Name: "net1",
							Ipam: types.IPAMConfig{
								Config: []*types.IPAMPool{
									{Subnet: "10.0.0.0/31"},
								},
							},
						},
					},
					Services: types.Services{
						"a": {
							Name: "a",
							Networks: map[string]*types.ServiceNetworkConfig{
								"net1": {},
							},
						},
						"b": {
							Name: "b",
							Networks: map[string]*types.ServiceNetworkConfig{
								"net1": {},
							},
						},
					},
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.project.AssignIPs(ctx)

			if tt.expectErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tt.expectAssigned {
				for _, svc := range tt.project.Services {
					for _, netCfg := range svc.Networks {
						assert.NotEmpty(t, netCfg.Ipv4Address)
					}
				}
			}
		})
	}
}

func TestServicesOrderedByDependencies(t *testing.T) {
	ctx := context.Background()

	project := &Project{
		Project: &types.Project{
			Services: types.Services{
				"db": {Name: "db"},
				"api": {
					Name: "api",
					DependsOn: types.DependsOnConfig{
						"db": {Required: true},
					},
				},
				"frontend": {
					Name: "frontend",
					DependsOn: types.DependsOnConfig{
						"api": {Required: true},
					},
				},
			},
		},
	}

	input := types.Services{
		"frontend": project.Services["frontend"],
	}

	result := project.ServicesOrderedByDependencies(ctx, input, true)

	require.Len(t, result, 3)

	assert.Equal(t, "db", result[0].Name)
	assert.Equal(t, "api", result[1].Name)
	assert.Equal(t, "frontend", result[2].Name)
}

func TestServicesReversedByDependencies(t *testing.T) {
	ctx := context.Background()

	project := &Project{
		Project: &types.Project{
			Services: types.Services{
				"db": {Name: "db"},
				"api": {
					Name: "api",
					DependsOn: types.DependsOnConfig{
						"db": {Required: true},
					},
				},
				"frontend": {
					Name: "frontend",
					DependsOn: types.DependsOnConfig{
						"api": {Required: true},
					},
				},
			},
		},
	}

	input := types.Services{
		"db": project.Services["db"],
	}

	result := project.ServicesReversedByDependencies(ctx, input, true)

	require.Len(t, result, 3)

	assert.Equal(t, "frontend", result[0].Name)
	assert.Equal(t, "api", result[1].Name)
	assert.Equal(t, "db", result[2].Name)
}