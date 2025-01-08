//go:build !saas

package handler

func (h *DatasetHandler) allowCreatePublic() bool {
	return true
}
