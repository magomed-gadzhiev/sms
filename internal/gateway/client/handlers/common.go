package handlers

import (
	"github.com/smpp-server/smpp-server/internal/shared/response"
)

// respondJSON delegates to the shared response package
var respondJSON = response.JSON

// respondError delegates to the shared response package
var respondError = response.Error

// respondGRPCError delegates to the shared response package
var respondGRPCError = response.GRPCError
