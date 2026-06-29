package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPDAcceleratorFromHardware(t *testing.T) {
	tests := []struct {
		name             string
		hw               HardWare
		expectedVendor   PDAcceleratorVendor
		expectedVariant  PDAcceleratorVariant
	}{
		{
			name: "NVIDIA GPU",
			hw: HardWare{
				Gpu: Processor{Type: "nvidia", Num: "1"},
			},
			expectedVendor:  PDAcceleratorVendorNVIDIA,
			expectedVariant: PDAcceleratorVariantGPU,
		},
		{
			name: "AMD GPU",
			hw: HardWare{
				Gpu: Processor{Type: "amd", Num: "2"},
			},
			expectedVendor:  PDAcceleratorVendorAMD,
			expectedVariant: PDAcceleratorVariantGPU,
		},
		{
			name: "Google TPU via NPU field",
			hw: HardWare{
				Npu: Processor{Type: "google", Num: "1"},
			},
			expectedVendor:  PDAcceleratorVendorGoogle,
			expectedVariant: PDAcceleratorVariantTPU,
		},
		{
			name: "CPU only (no accelerators)",
			hw: HardWare{
				Cpu:    CPU{Num: "64"},
				Memory: "128Gi",
			},
			expectedVendor:  PDAcceleratorVendorCPU,
			expectedVariant: PDAcceleratorVariantCPU,
		},
		{
			name: "GPU with zero count falls back to CPU",
			hw: HardWare{
				Gpu: Processor{Type: "nvidia", Num: "0"},
				Cpu: CPU{Num: "4"},
			},
			expectedVendor:  PDAcceleratorVendorCPU,
			expectedVariant: PDAcceleratorVariantCPU,
		},
		{
			name: "Unknown GPU type",
			hw: HardWare{
				Gpu: Processor{Type: "custom-vendor", Num: "1"},
			},
			expectedVendor:  PDAcceleratorVendor("custom-vendor"),
			expectedVariant: PDAcceleratorVariantGPU,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vendor, variant := PDAcceleratorFromHardware(tt.hw)
			require.Equal(t, tt.expectedVendor, vendor)
			require.Equal(t, tt.expectedVariant, variant)
		})
	}
}
