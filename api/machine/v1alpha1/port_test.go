package v1alpha1

import (
	"strings"
	"testing"
	corev1 "k8s.io/api/core/v1"
)

func TestParsePort(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantErr      bool
		expectedPort int32
		expectedProt corev1.Protocol
		expectedIP   string
	}{
		{
			name:         "Standard TCP mapping",
			input:        "8080:80/tcp",
			wantErr:      false,
			expectedPort: 80,
			expectedProt: corev1.ProtocolTCP,
		},
		{
			name:         "Default to TCP when deleted",
			input:        "80:80",
			wantErr:      false,
			expectedPort: 80,
			expectedProt: corev1.ProtocolTCP,
		},
		{
			name:         "Mapping with Host IP",
			input:        "127.0.0.1:8080:80/tcp",
			wantErr:      false,
			expectedPort: 80,
			expectedProt: corev1.ProtocolTCP,
			expectedIP:   "127.0.0.1",
		},
		{
			name:    "Port above maximum limit ",
			input:   "80:65536/tcp",
			wantErr: true,
		},
		{
			name:    "Port is zero",
			input:   "80:0/tcp",
			wantErr: true,
		},
		{
			name:    "Negative port number",
			input:   "80:-1/tcp",
			wantErr: true,
		},
		{
			name:    "Invalid string input",
			input:   "not-a-port",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePort(tt.input)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParsePort(%q) should have failed", tt.input)
				}
				return
			}

			if err != nil {
				t.Fatalf("ParsePort(%q) unexpected error: %v", tt.input, err)
			}

			if len(got) == 0 {
				t.Fatal("Expected at least one port mapping, got zero")
			}

			
			if got[0].MachinePort != tt.expectedPort {
				t.Errorf("MachinePort = %v, want %v", got[0].MachinePort, tt.expectedPort)
			}

			
			if tt.expectedIP != "" && got[0].HostIP != tt.expectedIP {
				t.Errorf("HostIP = %v, want %v", got[0].HostIP, tt.expectedIP)
			}

		
			if !strings.EqualFold(string(got[0].Protocol), string(tt.expectedProt)) {
				t.Errorf("Protocol = %v, want %v", got[0].Protocol, tt.expectedProt)
			}
		})
	}
}

func TestMachinePorts_String(t *testing.T) {
	ports := MachinePorts{
		{
			HostIP:      "127.0.0.1",
			HostPort:    8080,
			MachinePort: 80,
			Protocol:    corev1.ProtocolTCP,
		},
	}

	expected := "127.0.0.1:8080->80/TCP"
	if !strings.EqualFold(ports.String(), expected) {
		t.Errorf("String() = %q, want %q", ports.String(), expected)
	}
}