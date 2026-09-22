package component

import "errors"

var (
	ErrForbidden = errors.New("forbidden")

	// ErrRechargeNotInvoicable is returned when some recharge orders cannot be
	// invoiced, e.g. they do not exist, are not paid, or are already attached
	// to another non-failed invoice.
	ErrRechargeNotInvoicable = errors.New("some recharge orders are not invoicable")

	// ErrInvoiceAlreadyFailed is returned when trying to update an invoice
	// whose status is already failed. Failed is a terminal state: its
	// recharge orders are released back to the invoicable pool and a new
	// invoice application must be made for them.
	ErrInvoiceAlreadyFailed = errors.New("failed invoice cannot be updated")
)
