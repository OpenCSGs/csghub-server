package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPDRecommendation_IsEmpty_Nil(t *testing.T) {
	var r *PDRecommendation
	require.True(t, r.IsEmpty())
}

func TestPDRecommendation_IsEmpty_Zero(t *testing.T) {
	r := &PDRecommendation{}
	require.True(t, r.IsEmpty())
}

func TestPDRecommendation_IsEmpty_NotEmpty(t *testing.T) {
	r := &PDRecommendation{
		Prefill: PDRoleConfig{TotalGPUs: 8},
		Decode:  PDRoleConfig{TotalGPUs: 16},
	}
	require.False(t, r.IsEmpty())
}

func TestPDRecommendation_TotalGPUs(t *testing.T) {
	r := &PDRecommendation{
		Prefill: PDRoleConfig{TotalGPUs: 8},
		Decode:  PDRoleConfig{TotalGPUs: 16},
	}
	require.Equal(t, 24, r.TotalGPUs())

	var nilRec *PDRecommendation
	require.Equal(t, 0, nilRec.TotalGPUs())
}

func TestModelArchType_String(t *testing.T) {
	require.Equal(t, "dense", ModelArchTypeDense.String())
	require.Equal(t, "moe", ModelArchTypeMoE.String())
	require.Equal(t, "hybrid", ModelArchTypeHybrid.String())
}

func TestModelArchType_IsMoE(t *testing.T) {
	require.False(t, ModelArchTypeDense.IsMoE())
	require.True(t, ModelArchTypeMoE.IsMoE())
	require.True(t, ModelArchTypeHybrid.IsMoE())
}

func TestDetectModelArchType_Dense(t *testing.T) {
	require.Equal(t, ModelArchTypeDense, DetectModelArchType(0))
	require.Equal(t, ModelArchTypeDense, DetectModelArchType(-1))
}

func TestDetectModelArchType_MoE(t *testing.T) {
	require.Equal(t, ModelArchTypeMoE, DetectModelArchType(256))
	require.Equal(t, ModelArchTypeMoE, DetectModelArchType(8))
	require.Equal(t, ModelArchTypeMoE, DetectModelArchType(1))
}
