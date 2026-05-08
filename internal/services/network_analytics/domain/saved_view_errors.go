package domain

import "errors"

// Sentinel errors returned by ViewsRepository.Delete (and any other view-mutating
// operations that need to differentiate "missing" vs "forbidden" vs "system template").
//
// Higher layers (gRPC, HTTP) MUST inspect these via errors.Is to map to the right
// status code instead of swallowing everything as 500.

// ErrViewNotFound is returned when a saved view with the given id does not exist.
// gRPC layer should map to codes.NotFound; HTTP to 404 VIEW_NOT_FOUND.
var ErrViewNotFound = errors.New("saved view not found")

// ErrViewIsTemplate is returned when the caller tries to mutate (e.g. delete) a
// system template view (is_template=true, user_id IS NULL). Templates can only be
// cloned, not modified or deleted. gRPC: codes.PermissionDenied with the word
// "template" in the message; HTTP: 403 VIEW_IS_TEMPLATE.
var ErrViewIsTemplate = errors.New("saved view is a system template")

// ErrViewForbidden is returned when the saved view exists and is not a template,
// but it does not belong to the requesting (partnerID, userID) tuple.
// gRPC: codes.PermissionDenied; HTTP: 403 VIEW_FORBIDDEN.
var ErrViewForbidden = errors.New("saved view access forbidden")
