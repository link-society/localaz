// Package azerr models the error responses returned by Azure Storage so that
// the Azure SDKs and the Azure CLI interpret emulator failures exactly the way
// they would interpret failures from the real service.
package azerr

import (
	"encoding/json"
	"encoding/xml"
	"net/http"

	"localaz.dev/internal/utils/azwire"
)

// Code is an Azure Storage error code (the value of the x-ms-error-code header
// and the <Code> element in the XML error body).
type Code string

const (
	CodeContainerNotFound      Code = "ContainerNotFound"
	CodeContainerAlreadyExists Code = "ContainerAlreadyExists"
	CodeContainerBeingDeleted  Code = "ContainerBeingDeleted"
	CodeBlobNotFound           Code = "BlobNotFound"
	CodeBlobAlreadyExists      Code = "BlobAlreadyExists"
	CodeInvalidResourceName    Code = "InvalidResourceName"
	CodeOutOfRangeInput        Code = "OutOfRangeInput"
	CodeInvalidInput           Code = "InvalidInput"
	CodeInvalidQueryParameter  Code = "InvalidQueryParameterValue"
	CodeUnsupportedHeader      Code = "UnsupportedHeader"
	CodeMissingRequiredHeader  Code = "MissingRequiredHeader"
	CodeInvalidBlockList       Code = "InvalidBlockList"
	CodeInternalError          Code = "InternalError"
	CodeAuthFailed             Code = "AuthenticationFailed"

	// Queue service.
	CodeQueueNotFound      Code = "QueueNotFound"
	CodeQueueAlreadyExists Code = "QueueAlreadyExists"
	CodeMessageNotFound    Code = "MessageNotFound"
	CodePopReceiptMismatch Code = "PopReceiptMismatch"

	// Table service.
	CodeTableNotFound         Code = "TableNotFound"
	CodeTableAlreadyExists    Code = "TableAlreadyExists"
	CodeResourceNotFound      Code = "ResourceNotFound"
	CodeEntityAlreadyExists   Code = "EntityAlreadyExists"
	CodeUpdateConditionNotMet Code = "UpdateConditionNotSatisfied"
)

// Error is a structured Azure Storage error. It implements the error interface
// and carries the HTTP status code and Azure error code needed to build a
// faithful response.
type Error struct {
	Status  int
	Code    Code
	Message string
}

func (e *Error) Error() string {
	return string(e.Code) + ": " + e.Message
}

// New builds an *Error.
func New(status int, code Code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

// Predefined constructors for the common cases.

func ContainerNotFound() *Error {
	return New(http.StatusNotFound, CodeContainerNotFound, "The specified container does not exist.")
}

func ContainerAlreadyExists() *Error {
	return New(http.StatusConflict, CodeContainerAlreadyExists, "The specified container already exists.")
}

func BlobNotFound() *Error {
	return New(http.StatusNotFound, CodeBlobNotFound, "The specified blob does not exist.")
}

func InvalidResourceName() *Error {
	return New(http.StatusBadRequest, CodeInvalidResourceName, "The specified resource name contains invalid characters.")
}

func InvalidBlockList() *Error {
	return New(http.StatusBadRequest, CodeInvalidBlockList, "The specified block list is invalid.")
}

func Internal(message string) *Error {
	if message == "" {
		message = "The server encountered an internal error. Please retry the request."
	}
	return New(http.StatusInternalServerError, CodeInternalError, message)
}

// Queue constructors.

func QueueNotFound() *Error {
	return New(http.StatusNotFound, CodeQueueNotFound, "The specified queue does not exist.")
}

func QueueAlreadyExists() *Error {
	return New(http.StatusConflict, CodeQueueAlreadyExists, "The specified queue already exists.")
}

func MessageNotFound() *Error {
	return New(http.StatusNotFound, CodeMessageNotFound, "The specified message does not exist.")
}

func PopReceiptMismatch() *Error {
	return New(http.StatusBadRequest, CodePopReceiptMismatch,
		"The specified pop receipt did not match the pop receipt for a dequeued message.")
}

// Table constructors.

func TableNotFound() *Error {
	return New(http.StatusNotFound, CodeTableNotFound, "The table specified does not exist.")
}

func TableAlreadyExists() *Error {
	return New(http.StatusConflict, CodeTableAlreadyExists, "The table specified already exists.")
}

func EntityNotFound() *Error {
	return New(http.StatusNotFound, CodeResourceNotFound, "The specified resource does not exist.")
}

func EntityAlreadyExists() *Error {
	return New(http.StatusConflict, CodeEntityAlreadyExists, "The specified entity already exists.")
}

func UpdateConditionNotMet() *Error {
	return New(http.StatusPreconditionFailed, CodeUpdateConditionNotMet,
		"The update condition specified in the request was not satisfied.")
}

// xmlBody is the on-the-wire representation of an Azure Storage error.
type xmlBody struct {
	XMLName xml.Name `xml:"Error"`
	Code    Code     `xml:"Code"`
	Message string   `xml:"Message"`
}

// XML serializes the error into the Azure error body, returning the document
// including the XML declaration.
func (e *Error) XML() ([]byte, error) {
	return azwire.MarshalXMLIndent(xmlBody{Code: e.Code, Message: e.Message})
}

// JSON serializes the error into the OData error body used by the Table
// service, which reports failures as JSON rather than XML.
func (e *Error) JSON() []byte {
	type message struct {
		Lang  string `json:"lang"`
		Value string `json:"value"`
	}
	type odataError struct {
		Code    Code    `json:"code"`
		Message message `json:"message"`
	}
	body, err := json.Marshal(struct {
		Error odataError `json:"odata.error"`
	}{Error: odataError{Code: e.Code, Message: message{Lang: "en-US", Value: e.Message}}})
	if err != nil {
		return []byte(`{"odata.error":{"code":"InternalError"}}`)
	}
	return body
}
