package common

import (
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/types"
)

const (
	commentMediaBucket        = "opencsg-public-resource"
	commentMediaEndpoint      = "cdn.example.com"
	commentMediaVHostAudioURL = "https://opencsg-public-resource.cdn.example.com/audio/11111111-1111-4111-8111-111111111111.mp3"
	commentMediaPathAudioURL  = "https://cdn.example.com/opencsg-public-resource/audio/22222222-2222-4222-8222-222222222222.wav"
	commentMediaVHostVideoURL = "https://opencsg-public-resource.cdn.example.com/video/33333333-3333-4333-8333-333333333333.mp4"
)

func TestExtractCommentMediaRefs_MarkdownImages(t *testing.T) {
	content := "![alt](https://opencsg-public-resource.cdn.example.com/11111111-1111-4111-8111-111111111111.png) text " +
		"![b](https://cdn.example.com/opencsg-public-resource/22222222-2222-4222-8222-222222222222.jpg)"
	refs, err := ExtractCommentMediaRefs(content, commentMediaBucket, commentMediaEndpoint, 10)
	require.NoError(t, err)
	require.Len(t, refs, 2)
	require.Equal(t, types.MediaTypeImage, refs[0].Type)
	require.Equal(t, types.MediaTypeImage, refs[1].Type)
}

func TestExtractCommentMediaRefs_MarkdownImageVideoByExtension(t *testing.T) {
	// A Markdown image node referencing a .mp4 must be classified as video
	// (routed to async media moderation), not image (which the synchronous
	// image check would reject as "format not support").
	content := "![v](" + commentMediaVHostVideoURL + ")"
	refs, err := ExtractCommentMediaRefs(content, commentMediaBucket, commentMediaEndpoint, 10)
	require.NoError(t, err)
	require.Len(t, refs, 1)
	require.Equal(t, types.MediaTypeVideo, refs[0].Type)
}

func TestExtractCommentMediaRefs_MarkdownImageAudioByExtension(t *testing.T) {
	content := "![a](" + commentMediaVHostAudioURL + ")"
	refs, err := ExtractCommentMediaRefs(content, commentMediaBucket, commentMediaEndpoint, 10)
	require.NoError(t, err)
	require.Len(t, refs, 1)
	require.Equal(t, types.MediaTypeAudio, refs[0].Type)
}

func TestExtractCommentMediaRefs_OSSVirtualHostVideoURL(t *testing.T) {
	// Real-world Aliyun OSS virtual-host URL form: bucket.oss-<region>.aliyuncs.com/key.mp4
	// The bucket/endpoint must resolve so the ref is accepted for async moderation.
	const (
		ossBucket   = "opencsg-public-resource"
		ossEndpoint = "oss-cn-beijing.aliyuncs.com"
		ossVideoURL = "https://opencsg-public-resource.oss-cn-beijing.aliyuncs.com/comment/c9ada965-6b7f-47e0-8bdd-3495acc616f4.mp4"
	)
	content := "![v](" + ossVideoURL + ")"
	refs, err := ExtractCommentMediaRefs(content, ossBucket, ossEndpoint, 10)
	require.NoError(t, err)
	require.Len(t, refs, 1)
	require.Equal(t, types.MediaTypeVideo, refs[0].Type)
	require.Equal(t, ossBucket, refs[0].Bucket)
	require.NotEmpty(t, refs[0].ObjectKey)
}

func TestExtractCommentMediaRefs_HTMLAudioVideo(t *testing.T) {
	content := `<audio src="` + commentMediaVHostAudioURL + `"></audio>` +
		`<video src="` + commentMediaVHostVideoURL + `"></video>`
	refs, err := ExtractCommentMediaRefs(content, commentMediaBucket, commentMediaEndpoint, 10)
	require.NoError(t, err)
	require.Len(t, refs, 2)
	require.ElementsMatch(t, []types.MediaType{types.MediaTypeAudio, types.MediaTypeVideo}, []types.MediaType{refs[0].Type, refs[1].Type})
}

func TestExtractCommentMediaRefs_HTMLSourceInAudio(t *testing.T) {
	content := `<audio><source src="` + commentMediaPathAudioURL + `" type="audio/wav"></audio>`
	refs, err := ExtractCommentMediaRefs(content, commentMediaBucket, commentMediaEndpoint, 10)
	require.NoError(t, err)
	require.Len(t, refs, 1)
	require.Equal(t, types.MediaTypeAudio, refs[0].Type)
}

func TestExtractCommentMediaRefs_ExternalAudioRejected(t *testing.T) {
	content := `<audio src="https://other.example.com/a.mp3"></audio>`
	_, err := ExtractCommentMediaRefs(content, commentMediaBucket, commentMediaEndpoint, 10)
	require.Error(t, err)
}

func TestExtractCommentMediaRefs_ExternalImageRejected(t *testing.T) {
	content := `<img src="https://other.example.com/a.png">`
	_, err := ExtractCommentMediaRefs(content, commentMediaBucket, commentMediaEndpoint, 10)
	require.Error(t, err)
}

func TestExtractCommentMediaRefs_NonUUIDObjectRejected(t *testing.T) {
	content := `<img src="https://opencsg-public-resource.cdn.example.com/a.png">`
	_, err := ExtractCommentMediaRefs(content, commentMediaBucket, commentMediaEndpoint, 10)
	require.Error(t, err)
}

func TestExtractCommentMediaRefs_ControlledUUIDImageAccepted(t *testing.T) {
	content := `<img src="https://opencsg-public-resource.cdn.example.com/33333333-3333-4333-8333-333333333333.png">`
	refs, err := ExtractCommentMediaRefs(content, commentMediaBucket, commentMediaEndpoint, 10)
	require.NoError(t, err)
	require.Len(t, refs, 1)
	require.Equal(t, types.MediaTypeImage, refs[0].Type)
}

func TestExtractCommentMediaRefs_URLModifiersRejected(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "query",
			content: `<video src="` + commentMediaVHostVideoURL + `?x-oss-process=video/snapshot,t_0,f_jpg"></video>`,
		},
		{
			name:    "fragment",
			content: `<audio src="` + commentMediaVHostAudioURL + `#t=10"></audio>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ExtractCommentMediaRefs(tt.content, commentMediaBucket, commentMediaEndpoint, 10)
			require.ErrorContains(t, err, "must not contain query parameters or fragments")
		})
	}
}

func TestExtractCommentMediaRefs_Dedup(t *testing.T) {
	content := `<audio src="` + commentMediaVHostAudioURL + `"></audio>` +
		`<audio src="` + commentMediaVHostAudioURL + `"></audio>`
	refs, err := ExtractCommentMediaRefs(content, commentMediaBucket, commentMediaEndpoint, 10)
	require.NoError(t, err)
	require.Len(t, refs, 1)
}

func TestExtractCommentMediaRefs_MaxRefs(t *testing.T) {
	content := `<audio src="` + commentMediaVHostAudioURL + `"></audio>` +
		`<video src="` + commentMediaVHostVideoURL + `"></video>`
	_, err := ExtractCommentMediaRefs(content, commentMediaBucket, commentMediaEndpoint, 1)
	require.Error(t, err)
}

func TestExtractCommentMediaRefs_VirtualHostAndPathStyle(t *testing.T) {
	content := `<audio src="` + commentMediaVHostAudioURL + `"></audio>` +
		`<audio src="` + commentMediaPathAudioURL + `"></audio>`
	refs, err := ExtractCommentMediaRefs(content, commentMediaBucket, commentMediaEndpoint, 10)
	require.NoError(t, err)
	require.Len(t, refs, 2)
	for _, ref := range refs {
		require.Equal(t, commentMediaBucket, ref.Bucket)
		require.NotEmpty(t, ref.ObjectKey)
	}
}

func TestExtractCommentMediaRefs_ResourceKey(t *testing.T) {
	ref := types.MediaRef{Bucket: "b", ObjectKey: "o"}
	key, err := ref.ResourceKey()
	require.NoError(t, err)
	require.NotEmpty(t, key)
	// same bucket+objectKey → same key
	ref2 := types.MediaRef{Bucket: "b", ObjectKey: "o"}
	require.Equal(t, key, mustResourceKey(t, ref2))
	// different objectKey → different key
	ref3 := types.MediaRef{Bucket: "b", ObjectKey: "o2"}
	require.NotEqual(t, key, mustResourceKey(t, ref3))
	// missing fields → error
	_, err = (types.MediaRef{Bucket: "b"}).ResourceKey()
	require.Error(t, err)
}

func mustResourceKey(t *testing.T, ref types.MediaRef) string {
	t.Helper()
	key, err := ref.ResourceKey()
	require.NoError(t, err)
	return key
}
