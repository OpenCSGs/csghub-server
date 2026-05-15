package handler

import (
	"mime/multipart"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/types"
)

type ClawHubTester struct {
	*testutil.GinTester
	handler *ClawHubHandler
	mocks   struct {
		clawhub *mockcomponent.MockClawHubComponent
	}
}

func NewClawHubTester(t *testing.T) *ClawHubTester {
	tester := &ClawHubTester{GinTester: testutil.NewGinTester()}
	tester.mocks.clawhub = mockcomponent.NewMockClawHubComponent(t)
	tester.handler = &ClawHubHandler{
		clawhub: tester.mocks.clawhub,
	}
	return tester
}

func (ct *ClawHubTester) WithHandleFunc(fn func(h *ClawHubHandler) gin.HandlerFunc) *ClawHubTester {
	ct.Handler(fn(ct.handler))
	return ct
}

func TestClawHubHandler_SearchPassesCurrentUser(t *testing.T) {
	tester := NewClawHubTester(t).WithHandleFunc(func(h *ClawHubHandler) gin.HandlerFunc {
		return h.Search
	})
	tester.WithUser().WithQuery("q", "agent").WithQuery("limit", "5")

	resp := &types.ClawHubSearchResponse{
		Results: []types.ClawHubSearchResult{
			{
				Slug:        "u:agent",
				DisplayName: "Agent",
				Version:     "1.0.0",
				Score:       1.0,
			},
		},
	}
	tester.mocks.clawhub.EXPECT().Search(tester.Ctx(), "agent", 5, "u").Return(resp, nil)

	tester.Execute()
	tester.ResponseEqSimple(t, 200, resp)
}

func TestClawHubHandler_PublishSkill_AcceptsPlainPayloadPath(t *testing.T) {
	tester := NewClawHubTester(t).WithHandleFunc(func(h *ClawHubHandler) gin.HandlerFunc {
		return h.PublishSkill
	})
	tester.WithUser()
	tester.WithMultipartForm(&multipart.Form{
		Value: map[string][]string{
			"payload": {"/Users/zzh/os-code/skillhub/skillhub/x-search-1.0.0"},
			"version": {"1.0.3"},
		},
		File: map[string][]*multipart.FileHeader{},
	})

	tester.mocks.clawhub.EXPECT().PublishSkill(tester.Ctx(), &types.ClawHubPublishRequest{
		Slug:        "x-search",
		DisplayName: "x-search",
		Version:     "1.0.3",
	}, map[string][]byte{}, "u").Return(&types.ClawHubPublishSkillResponse{
		Ok:        true,
		SkillId:   "1",
		VersionId: "2",
	}, nil)

	tester.Execute()
	tester.ResponseEqSimple(t, 200, &types.ClawHubPublishSkillResponse{
		Ok:        true,
		SkillId:   "1",
		VersionId: "2",
	})
}

func TestClawHubHandler_GetSkillVersion(t *testing.T) {
	tester := NewClawHubTester(t).WithHandleFunc(func(h *ClawHubHandler) gin.HandlerFunc {
		return func(ctx *gin.Context) {
			ctx.Params = gin.Params{
				{Key: "slug", Value: "zhzhang%3Aauto-updater"},
				{Key: "version", Value: "1.0.2"},
			}
			h.GetSkillVersion(ctx)
		}
	})
	tester.WithUser()

	resp := &types.ClawHubSkillVersionResponse{
		Version: &types.ClawHubSkillVersionInfo{
			Version:   "1.0.2",
			Changelog: "",
		},
		Skill: &types.ClawHubVersionSkillInfo{
			Slug:        "zhzhang:auto-updater",
			DisplayName: "Auto Updater",
		},
	}
	tester.mocks.clawhub.EXPECT().GetSkillVersion(
		tester.Ctx(),
		"zhzhang:auto-updater",
		"1.0.2",
		"u",
	).Return(resp, nil)

	tester.Execute()
	tester.ResponseEqSimple(t, 200, resp)
}

func TestClawHubHandler_PublishSkill_NormalizesDashedVersionSlug(t *testing.T) {
	tester := NewClawHubTester(t).WithHandleFunc(func(h *ClawHubHandler) gin.HandlerFunc {
		return h.PublishSkill
	})
	tester.WithUser()
	tester.WithMultipartForm(&multipart.Form{
		Value: map[string][]string{
			"payload": {"/Users/zzh/skills/self-improving-agent-3-0-21"},
		},
		File: map[string][]*multipart.FileHeader{},
	})

	tester.mocks.clawhub.EXPECT().PublishSkill(tester.Ctx(), &types.ClawHubPublishRequest{
		Slug:        "self-improving-agent",
		DisplayName: "self-improving-agent",
	}, map[string][]byte{}, "u").Return(&types.ClawHubPublishSkillResponse{
		Ok:        true,
		SkillId:   "5",
		VersionId: "6",
	}, nil)

	tester.Execute()
	tester.ResponseEqSimple(t, 200, &types.ClawHubPublishSkillResponse{
		Ok:        true,
		SkillId:   "5",
		VersionId: "6",
	})
}

func TestClawHubHandler_PublishSkill_StripsDisplayNameVersion(t *testing.T) {
	tester := NewClawHubTester(t).WithHandleFunc(func(h *ClawHubHandler) gin.HandlerFunc {
		return h.PublishSkill
	})
	tester.WithUser()
	tester.WithMultipartForm(&multipart.Form{
		Value: map[string][]string{
			"payload": {`{"slug":"proactive-agent-3-1-0","displayName":"Proactive Agent 3.1.0"}`},
		},
		File: map[string][]*multipart.FileHeader{},
	})

	tester.mocks.clawhub.EXPECT().PublishSkill(tester.Ctx(), &types.ClawHubPublishRequest{
		Slug:        "proactive-agent",
		DisplayName: "Proactive Agent",
	}, map[string][]byte{}, "u").Return(&types.ClawHubPublishSkillResponse{
		Ok:        true,
		SkillId:   "7",
		VersionId: "8",
	}, nil)

	tester.Execute()
	tester.ResponseEqSimple(t, 200, &types.ClawHubPublishSkillResponse{
		Ok:        true,
		SkillId:   "7",
		VersionId: "8",
	})
}

func TestClawHubHandler_PublishSkill_DefaultsVersionFromFormCompatibility(t *testing.T) {
	tester := NewClawHubTester(t).WithHandleFunc(func(h *ClawHubHandler) gin.HandlerFunc {
		return h.PublishSkill
	})
	tester.WithUser()
	tester.WithMultipartForm(&multipart.Form{
		Value: map[string][]string{
			"slug": {"csghub-server-api"},
		},
		File: map[string][]*multipart.FileHeader{},
	})

	tester.mocks.clawhub.EXPECT().PublishSkill(tester.Ctx(), &types.ClawHubPublishRequest{
		Slug:        "csghub-server-api",
		DisplayName: "csghub-server-api",
		Version:     "",
	}, map[string][]byte{}, "u").Return(&types.ClawHubPublishSkillResponse{
		Ok:        true,
		SkillId:   "3",
		VersionId: "4",
	}, nil)

	tester.Execute()
	require.Equal(t, 200, tester.Response().Code)
}
