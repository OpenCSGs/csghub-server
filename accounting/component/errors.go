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

	// ErrInvoiceNotFound is returned when an invoice does not exist.
	ErrInvoiceNotFound = errors.New("invoice not found")

	// ErrInvoiceNotIssued is returned when trying to send an invoice email for
	// an invoice whose status is not issued.
	ErrInvoiceNotIssued = errors.New("invoice email can only be sent for issued invoices")

	// ErrInvoiceURLMissing is returned when an issued invoice has no download
	// URL to send.
	ErrInvoiceURLMissing = errors.New("invoice url is missing")
)
