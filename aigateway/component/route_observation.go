package component

import (
	"net/http"
)

// RouteCallObservation is inert when feedback is disabled or no rank was used.
type RouteCallObservation struct {
	modify func(*http.Response) error
	finish func(error)
}

func (o *RouteCallObservation) Enabled() bool { return o.finish != nil }

func (o *RouteCallObservation) Modify(response *http.Response) error {
	if o.modify != nil {
		return o.modify(response)
	}
	return nil
}
func (o *RouteCallObservation) Finish(err error) {
	if o.finish != nil {
		o.finish(err)
	}
}
