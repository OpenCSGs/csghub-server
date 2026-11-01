package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

func TestRuleItemMatches(t *testing.T) {
	cases := []struct {
		name  string
		item  types.RuleItem
		value string
		want  bool
	}{
		{name: "include hits listed value", item: types.RuleItem{Mode: types.RuleItemTypeInclude, Values: []string{"A100", "H800"}}, value: "A100", want: true},
		{name: "include rejects unlisted value", item: types.RuleItem{Mode: types.RuleItemTypeInclude, Values: []string{"A100"}}, value: "H800", want: false},
		{name: "include with empty values matches nothing", item: types.RuleItem{Mode: types.RuleItemTypeInclude}, value: "A100", want: false},
		{name: "any matches any value", item: types.RuleItem{Mode: types.RuleItemTypeAny, Values: []string{"A100"}}, value: "H800", want: true},
		{name: "unset mode rejects", item: types.RuleItem{}, value: "H800", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, ruleItemMatches(tc.item, tc.value))
		})
	}
}

// TestVoucherAppliesToComputeFilter keeps the dashboard and the deployment
// balance check on the deduct path's compute-rule semantics: an empty rule list
// and any-mode items admit any resource, while an include-mode restriction must
// name the requested cluster and xpu model.
func TestVoucherAppliesToComputeFilter(t *testing.T) {
	cases := []struct {
		name    string
		voucher AccountVoucher
		cluster string
		xpu     string
		want    bool
	}{
		{
			name:    "compute voucher without rules admits any resource",
			voucher: testVoucher(types.VoucherTypeCompute, types.VoucherRules{}, "2026-12-31"),
			cluster: "c1", xpu: "A100", want: true,
		},
		{
			name:    "universal voucher without rules admits any resource",
			voucher: testVoucher(types.VoucherTypeUniversal, types.VoucherRules{}, "2026-12-31"),
			cluster: "c1", xpu: "A100", want: true,
		},
		{
			name:    "universal voucher declaring only token limits admits compute",
			voucher: testVoucher(types.VoucherTypeUniversal, rulesToken(include("ns/model")), "2026-12-31"),
			cluster: "c1", xpu: "A100", want: true,
		},
		{
			name:    "any-mode items ignore their values",
			voucher: testVoucher(types.VoucherTypeCompute, rulesCompute(types.RuleItem{Mode: types.RuleItemTypeAny, Values: []string{"c9"}}, anyItem()), "2026-12-31"),
			cluster: "c1", xpu: "A100", want: true,
		},
		{
			name:    "include-mode cluster naming the requested cluster matches",
			voucher: testVoucher(types.VoucherTypeCompute, rulesCompute(include("c1"), anyItem()), "2026-12-31"),
			cluster: "c1", xpu: "A100", want: true,
		},
		{
			name:    "include-mode cluster naming another cluster does not match",
			voucher: testVoucher(types.VoucherTypeCompute, rulesCompute(include("c2"), anyItem()), "2026-12-31"),
			cluster: "c1", xpu: "A100", want: false,
		},
		{
			name:    "include-mode xpu naming another model does not match",
			voucher: testVoucher(types.VoucherTypeCompute, rulesCompute(anyItem(), include("A100")), "2026-12-31"),
			cluster: "c1", xpu: "H800", want: false,
		},
		{
			name:    "an omitted cluster is admitted by an any-mode rule",
			voucher: testVoucher(types.VoucherTypeCompute, rulesCompute(anyItem(), include("A100")), "2026-12-31"),
			cluster: "", xpu: "A100", want: true,
		},
		{
			name:    "an omitted xpu is not admitted by an xpu restriction",
			voucher: testVoucher(types.VoucherTypeCompute, rulesCompute(include("c1"), include("A100")), "2026-12-31"),
			cluster: "c1", xpu: "", want: false,
		},
		{
			name:    "token voucher never pays a compute resource",
			voucher: testVoucher(types.VoucherTypeToken, types.VoucherRules{}, "2026-12-31"),
			cluster: "c1", xpu: "A100", want: false,
		},
		{
			name:    "unknown voucher type never pays a compute resource",
			voucher: testVoucher("", types.VoucherRules{}, "2026-12-31"),
			cluster: "c1", xpu: "A100", want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, VoucherAppliesToComputeFilter(tc.voucher, tc.cluster, tc.xpu))
		})
	}
}

func testVoucher(voucherType types.VoucherType, rules types.VoucherRules, end day) AccountVoucher {
	return AccountVoucher{
		ID:          1,
		VoucherType: voucherType,
		Rules:       rules,
		EndDate:     end.time(),
		times:       times{CreatedAt: end.time()},
	}
}

type day string

func (d day) time() time.Time {
	t, err := time.Parse(time.DateOnly, string(d))
	if err != nil {
		panic(err)
	}
	return t
}

func TestVoucherSortKeyFor_ComputeScene(t *testing.T) {
	usage := voucherUsage{clusterID: "c1", xpuModel: "A100"}

	cases := []struct {
		name    string
		voucher AccountVoucher
		wantOK  bool
		wantKey voucherSortKey
	}{
		{
			name:    "compute rule naming cluster and xpu is most specific",
			voucher: testVoucher(types.VoucherTypeCompute, rulesCompute(include("c1"), include("A100")), "2026-12-31"),
			wantOK:  true, wantKey: voucherSortKey{typeRank: voucherRankSpecific, specificity: 2},
		},
		{
			name:    "compute rule naming only cluster",
			voucher: testVoucher(types.VoucherTypeCompute, rulesCompute(include("c1"), anyItem()), "2026-12-31"),
			wantOK:  true, wantKey: voucherSortKey{typeRank: voucherRankSpecific, specificity: 1},
		},
		{
			name:    "compute rule naming only xpu",
			voucher: testVoucher(types.VoucherTypeCompute, rulesCompute(anyItem(), include("A100")), "2026-12-31"),
			wantOK:  true, wantKey: voucherSortKey{typeRank: voucherRankSpecific, specificity: 1},
		},
		{
			name:    "unrestricted compute rule matches everywhere",
			voucher: testVoucher(types.VoucherTypeCompute, rulesCompute(anyItem(), anyItem()), "2026-12-31"),
			wantOK:  true, wantKey: voucherSortKey{typeRank: voucherRankSpecific, specificity: 0},
		},
		{
			name:    "compute rule for another cluster does not match",
			voucher: testVoucher(types.VoucherTypeCompute, rulesCompute(include("c2"), anyItem()), "2026-12-31"),
			wantOK:  false,
		},
		{
			name:    "compute voucher without compute limits matches as unrestricted",
			voucher: testVoucher(types.VoucherTypeCompute, types.VoucherRules{}, "2026-12-31"),
			wantOK:  true, wantKey: voucherSortKey{typeRank: voucherRankSpecific},
		},
		{
			name:    "token voucher does not apply to compute scene",
			voucher: testVoucher(types.VoucherTypeToken, types.VoucherRules{}, "2026-12-31"),
			wantOK:  false,
		},
		{
			name:    "universal voucher without declared rules applies as unrestricted",
			voucher: testVoucher(types.VoucherTypeUniversal, types.VoucherRules{}, "2026-12-31"),
			wantOK:  true, wantKey: voucherSortKey{typeRank: voucherRankUniversal},
		},
		{
			name:    "universal voucher restricted to the statement's cluster and xpu is most specific",
			voucher: testVoucher(types.VoucherTypeUniversal, rulesCompute(include("c1"), include("A100")), "2026-12-31"),
			wantOK:  true, wantKey: voucherSortKey{typeRank: voucherRankUniversal, specificity: 2},
		},
		{
			name:    "universal voucher restricted to the statement's cluster only",
			voucher: testVoucher(types.VoucherTypeUniversal, rulesCompute(include("c1"), anyItem()), "2026-12-31"),
			wantOK:  true, wantKey: voucherSortKey{typeRank: voucherRankUniversal, specificity: 1},
		},
		{
			name:    "universal voucher restricted to another cluster does not apply",
			voucher: testVoucher(types.VoucherTypeUniversal, rulesCompute(include("c2"), anyItem()), "2026-12-31"),
			wantOK:  false,
		},
		{
			name:    "unknown voucher type applies nowhere",
			voucher: testVoucher("", types.VoucherRules{}, "2026-12-31"),
			wantOK:  false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key, ok := voucherSortKeyFor(tc.voucher, usage)
			require.Equal(t, tc.wantOK, ok)
			require.Equal(t, tc.wantKey, key)
		})
	}
}

func TestVoucherSortKeyFor_ComputeSceneWithoutXPUModel(t *testing.T) {
	// a statement without an xpu model cannot be attributed to a compute
	// resource: dedicated compute vouchers are skipped, while a universal
	// voucher keeps paying as long as no include-mode restriction pins the
	// missing xpu dimension
	usage := voucherUsage{clusterID: "c1"}

	_, ok := voucherSortKeyFor(testVoucher(types.VoucherTypeCompute, rulesCompute(anyItem(), anyItem()), "2026-12-31"), usage)
	require.False(t, ok)

	// an unrestricted universal voucher (no declared compute rules) still pays
	key, ok := voucherSortKeyFor(testVoucher(types.VoucherTypeUniversal, types.VoucherRules{}, "2026-12-31"), usage)
	require.True(t, ok)
	require.Equal(t, voucherSortKey{typeRank: voucherRankUniversal}, key)

	// declared any-mode compute rules are still unrestricted
	key, ok = voucherSortKeyFor(testVoucher(types.VoucherTypeUniversal, rulesCompute(anyItem(), anyItem()), "2026-12-31"), usage)
	require.True(t, ok)
	require.Equal(t, voucherSortKey{typeRank: voucherRankUniversal}, key)

	// a cluster-only restriction stays attributable without an xpu model
	key, ok = voucherSortKeyFor(testVoucher(types.VoucherTypeUniversal, rulesCompute(include("c1"), anyItem()), "2026-12-31"), usage)
	require.True(t, ok)
	require.Equal(t, voucherSortKey{typeRank: voucherRankUniversal, specificity: 1}, key)

	// an xpu-model restriction cannot be attributed and skips the voucher
	_, ok = voucherSortKeyFor(testVoucher(types.VoucherTypeUniversal, rulesCompute(anyItem(), include("A100")), "2026-12-31"), usage)
	require.False(t, ok)
}

func TestVoucherSortKeyFor_TokenScene(t *testing.T) {
	usage := voucherUsage{tokenScene: true, modelID: "ns/model"}

	cases := []struct {
		name    string
		voucher AccountVoucher
		wantOK  bool
		wantKey voucherSortKey
	}{
		{
			name:    "token rule naming the model is specific",
			voucher: testVoucher(types.VoucherTypeToken, rulesToken(include("ns/model")), "2026-12-31"),
			wantOK:  true, wantKey: voucherSortKey{typeRank: voucherRankSpecific, specificity: 1},
		},
		{
			name:    "token rule for another model does not match",
			voucher: testVoucher(types.VoucherTypeToken, rulesToken(include("other/model")), "2026-12-31"),
			wantOK:  false,
		},
		{
			name:    "unrestricted token rule matches every model",
			voucher: testVoucher(types.VoucherTypeToken, rulesToken(anyItem()), "2026-12-31"),
			wantOK:  true, wantKey: voucherSortKey{typeRank: voucherRankSpecific, specificity: 0},
		},
		{
			name:    "compute voucher does not apply to token scene",
			voucher: testVoucher(types.VoucherTypeCompute, rulesCompute(include("c1"), include("A100")), "2026-12-31"),
			wantOK:  false,
		},
		{
			name:    "universal voucher with unrestricted token limits applies to every model",
			voucher: testVoucher(types.VoucherTypeUniversal, rulesToken(anyItem()), "2026-12-31"),
			wantOK:  true, wantKey: voucherSortKey{typeRank: voucherRankUniversal},
		},
		{
			name:    "universal voucher restricted to the statement's model is specific",
			voucher: testVoucher(types.VoucherTypeUniversal, rulesToken(include("ns/model")), "2026-12-31"),
			wantOK:  true, wantKey: voucherSortKey{typeRank: voucherRankUniversal, specificity: 1},
		},
		{
			name:    "universal voucher restricted to another model does not apply",
			voucher: testVoucher(types.VoucherTypeUniversal, rulesToken(include("other/model")), "2026-12-31"),
			wantOK:  false,
		},
		{
			name:    "universal voucher without declared token limits does not apply",
			voucher: testVoucher(types.VoucherTypeUniversal, types.VoucherRules{}, "2026-12-31"),
			wantOK:  false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key, ok := voucherSortKeyFor(tc.voucher, usage)
			require.Equal(t, tc.wantOK, ok)
			require.Equal(t, tc.wantKey, key)
		})
	}
}

func TestApplicableVouchers_Order(t *testing.T) {
	t.Run("compute scene keeps specific rules first and universal last", func(t *testing.T) {
		usage := voucherUsage{clusterID: "c1", xpuModel: "A100"}
		vouchers := []AccountVoucher{
			testVoucher(types.VoucherTypeUniversal, types.VoucherRules{}, "2026-01-01"),
			testVoucher(types.VoucherTypeUniversal, rulesCompute(include("c1"), include("A100")), "2026-02-01"),
			testVoucher(types.VoucherTypeCompute, rulesCompute(anyItem(), anyItem()), "2026-01-01"),
			testVoucher(types.VoucherTypeCompute, rulesCompute(include("c1"), include("A100")), "2026-01-01"),
			testVoucher(types.VoucherTypeCompute, rulesCompute(include("c1"), anyItem()), "2026-01-01"),
			// a matching rule beats a non-matching one filtered out below
			testVoucher(types.VoucherTypeCompute, rulesCompute(include("c2"), anyItem()), "2026-12-31"),
		}

		sorted := applicableVouchers(vouchers, usage)

		require.Len(t, sorted, 5)
		// the rule naming both cluster and xpu is the most specific
		require.Equal(t, types.VoucherTypeCompute, sorted[0].VoucherType)
		require.Equal(t, types.RuleItemTypeInclude, sorted[0].Rules.ComputeLimits[0].ClusterIDs.Mode)
		require.Equal(t, types.RuleItemTypeInclude, sorted[0].Rules.ComputeLimits[0].XPUModels.Mode)
		// then the cluster-only rule
		require.Equal(t, types.RuleItemTypeInclude, sorted[1].Rules.ComputeLimits[0].ClusterIDs.Mode)
		require.Equal(t, types.RuleItemTypeAny, sorted[1].Rules.ComputeLimits[0].XPUModels.Mode)
		// then the unrestricted compute rule
		require.Equal(t, types.VoucherTypeCompute, sorted[2].VoucherType)
		require.Equal(t, types.RuleItemTypeAny, sorted[2].Rules.ComputeLimits[0].ClusterIDs.Mode)
		require.Equal(t, types.RuleItemTypeAny, sorted[2].Rules.ComputeLimits[0].XPUModels.Mode)
		// then the restricted universal voucher, ahead of the unrestricted one
		require.Equal(t, types.VoucherTypeUniversal, sorted[3].VoucherType)
		require.Equal(t, types.RuleItemTypeInclude, sorted[3].Rules.ComputeLimits[0].ClusterIDs.Mode)
		// and the unrestricted universal voucher deducts last despite expiring first
		require.Equal(t, types.VoucherTypeUniversal, sorted[4].VoucherType)
		require.Empty(t, sorted[4].Rules.ComputeLimits)
	})

	t.Run("token scene keeps token vouchers before universal ones", func(t *testing.T) {
		usage := voucherUsage{tokenScene: true, modelID: "ns/model"}
		vouchers := []AccountVoucher{
			// the universal voucher expires sooner but must still deduct last
			testVoucher(types.VoucherTypeUniversal, rulesToken(anyItem()), "2026-01-01"),
			testVoucher(types.VoucherTypeToken, rulesToken(anyItem()), "2026-06-01"),
			testVoucher(types.VoucherTypeToken, rulesToken(include("ns/model")), "2026-12-31"),
		}

		sorted := applicableVouchers(vouchers, usage)

		require.Len(t, sorted, 3)
		require.Equal(t, types.VoucherTypeToken, sorted[0].VoucherType)
		require.Equal(t, types.RuleItemTypeInclude, sorted[0].Rules.TokenLimits.Models.Mode)
		require.Equal(t, types.VoucherTypeToken, sorted[1].VoucherType)
		require.Equal(t, types.VoucherTypeUniversal, sorted[2].VoucherType)
	})

	t.Run("restricted universal deducts before unrestricted universal", func(t *testing.T) {
		usage := voucherUsage{tokenScene: true, modelID: "ns/model"}
		vouchers := []AccountVoucher{
			// the unrestricted voucher expires sooner but the restricted one
			// is consumed first within the universal tier
			testVoucher(types.VoucherTypeUniversal, rulesToken(anyItem()), "2026-01-01"),
			testVoucher(types.VoucherTypeUniversal, rulesToken(include("ns/model")), "2026-12-31"),
		}

		sorted := applicableVouchers(vouchers, usage)

		require.Len(t, sorted, 2)
		require.Equal(t, types.RuleItemTypeInclude, sorted[0].Rules.TokenLimits.Models.Mode)
		require.Equal(t, types.RuleItemTypeAny, sorted[1].Rules.TokenLimits.Models.Mode)
	})

	t.Run("earliest expiration breaks ties within the same rank", func(t *testing.T) {
		usage := voucherUsage{tokenScene: true, modelID: "ns/model"}
		vouchers := []AccountVoucher{
			testVoucher(types.VoucherTypeToken, rulesToken(anyItem()), "2026-12-31"),
			testVoucher(types.VoucherTypeToken, rulesToken(anyItem()), "2026-03-01"),
		}

		sorted := applicableVouchers(vouchers, usage)

		require.Len(t, sorted, 2)
		require.Equal(t, day("2026-03-01").time(), sorted[0].EndDate)
		require.Equal(t, day("2026-12-31").time(), sorted[1].EndDate)
	})
}

// TestUniversalVoucherRuleCombos covers the four universal voucher rule
// combinations of the deduction spec: compute_limits and token_limits are
// orthogonal, and each scene only evaluates the side billing it.
func TestUniversalVoucherRuleCombos(t *testing.T) {
	computeUsage := voucherUsage{clusterID: "c1", xpuModel: "A100"}
	tokenUsage := voucherUsage{tokenScene: true, modelID: "ns/model"}

	t.Run("combo A: both sides unrestricted applies everywhere", func(t *testing.T) {
		v := testVoucher(types.VoucherTypeUniversal, rulesBoth(anyItem(), anyItem(), anyItem()), "2026-12-31")

		key, ok := voucherSortKeyFor(v, computeUsage)
		require.True(t, ok)
		require.Equal(t, voucherSortKey{typeRank: voucherRankUniversal}, key)

		key, ok = voucherSortKeyFor(v, tokenUsage)
		require.True(t, ok)
		require.Equal(t, voucherSortKey{typeRank: voucherRankUniversal}, key)
	})

	t.Run("combo B: compute restricted, token unrestricted", func(t *testing.T) {
		v := testVoucher(types.VoucherTypeUniversal, rulesBoth(include("low-cost-dc"), anyItem(), anyItem()), "2026-12-31")

		// the compute side only pays in the listed cluster
		_, ok := voucherSortKeyFor(v, computeUsage)
		require.False(t, ok)

		key, ok := voucherSortKeyFor(v, voucherUsage{clusterID: "low-cost-dc", xpuModel: "T4"})
		require.True(t, ok)
		require.Equal(t, voucherSortKey{typeRank: voucherRankUniversal, specificity: 1}, key)

		// the compute restriction never leaks into the token side
		key, ok = voucherSortKeyFor(v, tokenUsage)
		require.True(t, ok)
		require.Equal(t, voucherSortKey{typeRank: voucherRankUniversal}, key)
	})

	t.Run("combo C: token restricted, compute unrestricted", func(t *testing.T) {
		v := testVoucher(types.VoucherTypeUniversal, rulesBoth(anyItem(), anyItem(), include("ns/model")), "2026-12-31")

		// the token side only pays the listed model
		_, ok := voucherSortKeyFor(v, voucherUsage{tokenScene: true, modelID: "other/model"})
		require.False(t, ok)

		key, ok := voucherSortKeyFor(v, tokenUsage)
		require.True(t, ok)
		require.Equal(t, voucherSortKey{typeRank: voucherRankUniversal, specificity: 1}, key)

		// the token restriction never leaks into the compute side
		key, ok = voucherSortKeyFor(v, computeUsage)
		require.True(t, ok)
		require.Equal(t, voucherSortKey{typeRank: voucherRankUniversal}, key)
	})

	t.Run("combo D: both sides restricted", func(t *testing.T) {
		v := testVoucher(types.VoucherTypeUniversal, rulesBoth(include("c1"), include("A100"), include("ns/model")), "2026-12-31")

		key, ok := voucherSortKeyFor(v, computeUsage)
		require.True(t, ok)
		require.Equal(t, voucherSortKey{typeRank: voucherRankUniversal, specificity: 2}, key)

		key, ok = voucherSortKeyFor(v, tokenUsage)
		require.True(t, ok)
		require.Equal(t, voucherSortKey{typeRank: voucherRankUniversal, specificity: 1}, key)

		_, ok = voucherSortKeyFor(v, voucherUsage{clusterID: "c2", xpuModel: "A100"})
		require.False(t, ok)

		_, ok = voucherSortKeyFor(v, voucherUsage{tokenScene: true, modelID: "other/model"})
		require.False(t, ok)
	})
}

// resolveVoucherUsage reads the database only for compute scenes, so the token
// scene cases run against a zero transaction.
func TestResolveVoucherUsage_TokenScene(t *testing.T) {
	cases := []struct {
		name        string
		scene       types.SceneType
		resourceID  string
		customerID  string
		wantModelID string
		wantNil     bool
	}{
		{
			name:        "serverless resource id carries the model segment",
			scene:       types.SceneModelServerless,
			resourceID:  "csghub://serverless/ns/model",
			wantModelID: "ns/model",
		},
		{
			name:        "external model id comes from the thirdparty prefix",
			scene:       types.SceneMultiModalServerless,
			resourceID:  "thirdparty://ext-model",
			wantModelID: "ext-model",
		},
		{
			name:        "unrecognized resource id falls back to customer id",
			scene:       types.SceneModelServerless,
			resourceID:  "some-legacy-id",
			customerID:  "legacy/model",
			wantModelID: "legacy/model",
		},
		{
			name:       "no resolvable model skips voucher deduction",
			scene:      types.SceneModelServerless,
			resourceID: "some-legacy-id",
			wantNil:    true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := AccountStatement{
				Scene:      tc.scene,
				ResourceID: tc.resourceID,
				CustomerID: tc.customerID,
			}

			usage, err := resolveVoucherUsage(t.Context(), bun.Tx{}, input)

			require.NoError(t, err)
			if tc.wantNil {
				require.Nil(t, usage)
				return
			}
			require.NotNil(t, usage)
			require.True(t, usage.tokenScene)
			require.Equal(t, tc.wantModelID, usage.modelID)
		})
	}
}

func include(values ...string) types.RuleItem {
	return types.RuleItem{Mode: types.RuleItemTypeInclude, Values: values}
}

func anyItem() types.RuleItem {
	return types.RuleItem{Mode: types.RuleItemTypeAny}
}

func rulesCompute(clusterIDs, xpuModels types.RuleItem) types.VoucherRules {
	return types.VoucherRules{
		ComputeLimits: []types.VoucherComputeRule{
			{ClusterIDs: clusterIDs, XPUModels: xpuModels},
		},
	}
}

func rulesToken(models types.RuleItem) types.VoucherRules {
	return types.VoucherRules{TokenLimits: types.VoucherTokenRule{Models: models}}
}

func rulesBoth(clusterIDs, xpuModels, models types.RuleItem) types.VoucherRules {
	rules := rulesCompute(clusterIDs, xpuModels)
	rules.TokenLimits = types.VoucherTokenRule{Models: models}
	return rules
}
