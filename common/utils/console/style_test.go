package console

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStyle(t *testing.T) {
	greyMessage := RenderGreyColor("grey")
	greyMessage.Print()
	greyMessage.Println()

	assert.Equal(t, "grey", greyMessage.String())

	RenderError("error")
	RenderSuccess("success")
	RenderWarning("warning")
	RenderBox("box")
}
